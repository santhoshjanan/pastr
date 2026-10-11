// Package web serves pastr's pages.
package web

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/santhoshjanan/pastr/internal/auth"
	"github.com/santhoshjanan/pastr/internal/config"
	"github.com/santhoshjanan/pastr/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type Server struct {
	cfg        config.Config
	store      *store.Store
	sessions   auth.Sessions
	logins     *auth.Throttle
	signups    *auth.Throttle
	log        *slog.Logger
	now        func() time.Time
	pages      map[string]*template.Template
	assetVer   string
	baseOrigin string

	trustedProxy    netip.Prefix
	hasTrustedProxy bool
}

func New(cfg config.Config, st *store.Store, logger *slog.Logger) (*Server, error) {
	s := &Server{cfg: cfg, store: st, log: logger}
	s.SetClock(time.Now)
	u, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	s.baseOrigin = u.Scheme + "://" + u.Host
	if s.trustedProxy, s.hasTrustedProxy, err = cfg.TrustedProxyPrefix(); err != nil {
		return nil, err
	}
	if s.assetVer, err = hashFS(staticFS); err != nil {
		return nil, err
	}
	if s.pages, err = loadPages(s); err != nil {
		return nil, err
	}
	return s, nil
}

// SetClock replaces the time source everywhere it is used. Tests call it; production does not.
func (s *Server) SetClock(now func() time.Time) {
	s.now = now
	s.sessions = auth.Sessions{Store: s.store, Now: now}
	s.logins = auth.NewThrottle(5, 15*time.Minute, now)
	s.signups = auth.NewThrottle(5, time.Hour, now)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.Handle("GET /static/{ver}/{path...}", s.staticHandler())

	mux.HandleFunc("GET /login", s.handleLoginForm)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("GET /signup", s.handleSignupForm)
	mux.HandleFunc("POST /signup", s.handleSignup)
	mux.HandleFunc("POST /logout", s.requireUser(s.handleLogout))
	mux.HandleFunc("GET /account", s.requireUser(s.handleAccountForm))
	mux.HandleFunc("POST /account", s.requireUser(s.handleAccount))

	s.pasteRoutes(mux) // Tasks 8 and 9
	s.adminRoutes(mux) // Task 10

	mux.HandleFunc("/", s.gone)
	return s.recoverer(s.securityHeaders(s.limitBody(s.loadSession(s.protect(mux)))))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok"))
}

// staticHandler serves embedded assets under a version segment, so they can be cached forever.
func (s *Server) staticHandler() http.Handler {
	sub, _ := fs.Sub(staticFS, "static")
	files := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Directories are not listed.
		if p := r.PathValue("path"); r.PathValue("ver") != s.assetVer || p == "" || strings.HasSuffix(p, "/") {
			s.gone(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/" + r.PathValue("path")
		files.ServeHTTP(w, r2)
	})
}

func (s *Server) asset(name string) string { return "/static/" + s.assetVer + "/" + name }

func hashFS(fsys fs.FS) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		h.Write([]byte(p))
		h.Write(b)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:12], err
}

func (s *Server) pasteRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /p/{slug}", s.handleRead)
	mux.HandleFunc("GET /{$}", s.requireUser(s.handleList))
	mux.HandleFunc("GET /new", s.requireUser(s.handleNewForm))
	mux.HandleFunc("POST /new", s.requireUser(s.handleCreate))
	mux.HandleFunc("GET /p/{slug}/ready", s.requireOwner(s.handleReady))
	mux.HandleFunc("GET /p/{slug}/edit", s.requireOwner(s.handleEditForm))
	mux.HandleFunc("POST /p/{slug}/edit", s.requireOwner(s.handleEdit))
	mux.HandleFunc("POST /p/{slug}/delete", s.requireOwner(s.handleDelete))
}
