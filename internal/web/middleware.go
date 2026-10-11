package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strings"

	"github.com/santhoshjanan/pastr/internal/store"
)

const (
	cookieName = "pastr_session"
	// A 100 KB body can triple in size once form-encoded.
	maxRequestBody = 512 * 1024
)

type ctxKey int

const (
	keyUser ctxKey = iota
	keySession
	keyRequestID
)

func currentUser(r *http.Request) *store.User {
	u, _ := r.Context().Value(keyUser).(*store.User)
	return u
}

func currentSession(r *http.Request) *store.Session {
	x, _ := r.Context().Value(keySession).(*store.Session)
	return x
}

// recoverer tags each request with an ID and turns panics into the plain error page.
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 8)
		rand.Read(b)
		id := hex.EncodeToString(b)
		r = r.WithContext(context.WithValue(r.Context(), keyRequestID, id))
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				// The path is left out: paste URLs are secrets.
				s.log.Error("panic", "request_id", id, "panic", v, "stack", string(debug.Stack()))
				s.render(w, r, http.StatusInternalServerError, "error.html", pageData{Title: "Something broke", Error: msgBroke, NoNav: true, NoScript: true})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// contentSecurityPolicy builds the policy. styleExtra is empty everywhere except
// the editor pages, which pass a per-response nonce for the styles Trix injects.
func (s *Server) contentSecurityPolicy(styleExtra string) string {
	script, connect := "'self'", "'self'"
	if o := s.cfg.UmamiOrigin(); o != "" {
		script += " " + o
		connect += " " + o
	}
	return "default-src 'none'; style-src 'self'" + styleExtra + "; script-src " + script + "; connect-src " + connect +
		"; img-src 'self' data:; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	csp := s.contentSecurityPolicy("")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		if s.cfg.SecureCookies() {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) loadSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ck, err := r.Cookie(cookieName); err == nil && ck.Value != "" {
			if sess, user, err := s.sessions.Lookup(r.Context(), ck.Value); err == nil {
				ctx := context.WithValue(r.Context(), keyUser, &user)
				ctx = context.WithValue(ctx, keySession, &sess)
				r = r.WithContext(ctx)
				w.Header().Set("Cache-Control", "no-store")
			}
		}
		next.ServeHTTP(w, r)
	})
}

// protect rejects state-changing requests that do not come from our own pages:
// the Origin header must match, and a signed-in request must carry the session's CSRF token.
func (s *Server) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Origin") != s.baseOrigin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		if sess := currentSession(r); sess != nil {
			if err := r.ParseForm(); err != nil {
				s.render(w, r, http.StatusRequestEntityTooLarge, "error.html", pageData{
					Title: "That's too long", Error: "Go back, trim it down, and try again.", NoNav: true, NoScript: true})
				return
			}
			if subtle.ConstantTimeCompare([]byte(r.PostFormValue("csrf")), []byte(sess.CSRFToken)) != 1 {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r) == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.requireUser(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).IsAdmin {
			s.gone(w, r)
			return
		}
		next(w, r)
	})
}

// clientIP is used only as an in-memory throttle key. The forwarded header is
// believed only when the request arrives from the configured proxy.
func (s *Server) clientIP(r *http.Request) string {
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	peer := ap.Addr().Unmap()
	if s.hasTrustedProxy && s.trustedProxy.Contains(peer) {
		// The proxy appends the address it saw, so the last entry across every
		// header line is the only one a client cannot forge.
		parts := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
		if client, err := netip.ParseAddr(strings.TrimSpace(parts[len(parts)-1])); err == nil {
			return client.Unmap().String()
		}
	}
	return peer.String()
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: s.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode,
	})
}
