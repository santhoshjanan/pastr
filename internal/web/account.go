package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/santhoshjanan/pastr/internal/auth"
	"github.com/santhoshjanan/pastr/internal/store"
)

const (
	msgBadLogin      = "That username or password isn't right."
	maxUsernameInput = 64
)

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, r, http.StatusOK, "login.html", pageData{Title: "Sign in", Data: ""})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	username := strings.ToLower(strings.TrimSpace(r.PostFormValue("username")))
	password := r.PostFormValue("password")
	ip := s.clientIP(r)
	pairKey := username + "|" + ip

	badLogin := func() {
		s.render(w, r, http.StatusUnauthorized, "login.html", pageData{Title: "Sign in", Data: username, Error: msgBadLogin})
	}
	if len(username) > maxUsernameInput { // cannot be a real account; skip the throttle and the hash
		username = ""
		badLogin()
		return
	}
	// Slots are reserved before the password is checked, so a burst of parallel
	// guesses cannot outrun the limits. A successful sign-in hands them back.
	okPair := s.logins.Attempt(pairKey)
	okIP := s.loginsIP.Attempt(ip)
	okUser := s.loginsUser.Attempt(username)
	if !okPair || !okIP || !okUser {
		if okPair {
			s.logins.Release(pairKey)
		}
		if okIP {
			s.loginsIP.Release(ip)
		}
		if okUser {
			s.loginsUser.Release(username)
		}
		s.render(w, r, http.StatusTooManyRequests, "login.html",
			pageData{Title: "Sign in", Data: username, Error: "Too many tries. Wait a few minutes, then try again."})
		return
	}
	u, err := s.store.UserByUsername(r.Context(), username)
	ok := false
	switch {
	case err == nil:
		ok = auth.VerifyPassword(u.PasswordHash, password) && !u.Disabled
	case errors.Is(err, store.ErrNotFound):
		auth.BurnTime(password) // same cost as a real check
	default:
		s.fail(w, r, err)
		return
	}
	if !ok {
		badLogin() // the reserved slots stay spent
		return
	}
	s.logins.Reset(pairKey)
	s.loginsIP.Release(ip)
	s.loginsUser.Release(username)
	s.startSession(w, r, u.ID, "/?e=login")
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID int64, to string) {
	token, err := s.sessions.Start(r.Context(), userID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setSessionCookie(w, token, int(auth.SessionTTL.Seconds()))
	http.Redirect(w, r, to, http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if ck, err := r.Cookie(cookieName); err == nil {
		s.sessions.End(r.Context(), ck.Value)
	}
	s.setSessionCookie(w, "", -1)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) signupsOpen(w http.ResponseWriter, r *http.Request) bool {
	on, err := s.store.SignupsEnabled(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return false
	}
	if !on {
		s.gone(w, r)
	}
	return on
}

func (s *Server) handleSignupForm(w http.ResponseWriter, r *http.Request) {
	if !s.signupsOpen(w, r) {
		return
	}
	if currentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, r, http.StatusOK, "signup.html", pageData{Title: "Create an account", Data: ""})
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if !s.signupsOpen(w, r) {
		return
	}
	raw := r.PostFormValue("username")
	reject := func(status int, msg string) {
		s.render(w, r, status, "signup.html", pageData{Title: "Create an account", Data: raw, Error: msg})
	}
	ipKey := "ip:" + s.clientIP(r)
	if !s.signups.Attempt(ipKey) {
		reject(http.StatusTooManyRequests, "Too many sign-ups from here. Try again later.")
		return
	}
	// A typo is not an attempt: validation errors hand the slot back.
	username, err := auth.ValidateUsername(raw)
	if err != nil {
		s.signups.Release(ipKey)
		reject(http.StatusUnprocessableEntity, err.Error())
		return
	}
	password := r.PostFormValue("password")
	if err := auth.ValidatePassword(password); err != nil {
		s.signups.Release(ipKey)
		reject(http.StatusUnprocessableEntity, err.Error())
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	u, err := s.store.CreateUser(r.Context(), username, hash, false, s.now())
	if errors.Is(err, store.ErrConflict) {
		reject(http.StatusUnprocessableEntity, "That username is taken.")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.startSession(w, r, u.ID, "/?e=signup")
}

func (s *Server) handleAccountForm(w http.ResponseWriter, r *http.Request) {
	d := pageData{Title: "Account"}
	switch r.URL.Query().Get("saved") {
	case "profile":
		d.Notice = "Byline saved."
	case "password":
		d.Notice = "Password changed."
	}
	s.render(w, r, http.StatusOK, "account.html", d)
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	reject := func(msg string) {
		s.render(w, r, http.StatusUnprocessableEntity, "account.html", pageData{Title: "Account", Error: msg})
	}
	switch r.PostFormValue("action") {
	case "profile":
		name := strings.Join(strings.Fields(r.PostFormValue("display_name")), " ")
		link := strings.TrimSpace(r.PostFormValue("display_link"))
		if utf8.RuneCountInString(name) > 60 {
			reject("Keep the display name under 60 characters.")
			return
		}
		if link != "" {
			lu, err := url.Parse(link)
			if err != nil || (lu.Scheme != "http" && lu.Scheme != "https") || lu.Host == "" || len(link) > 300 {
				reject("The link needs to start with https:// or http://.")
				return
			}
		}
		if err := s.store.UpdateProfile(r.Context(), u.ID, name, link); err != nil {
			s.fail(w, r, err)
			return
		}
		http.Redirect(w, r, "/account?saved=profile", http.StatusSeeOther)
	case "password":
		if !auth.VerifyPassword(u.PasswordHash, r.PostFormValue("current")) {
			reject("Your current password isn't right.")
			return
		}
		if err := auth.ValidatePassword(r.PostFormValue("new")); err != nil {
			reject(err.Error())
			return
		}
		hash, err := auth.HashPassword(r.PostFormValue("new"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if err := s.store.UpdatePassword(r.Context(), u.ID, hash); err != nil {
			s.fail(w, r, err)
			return
		}
		// Changing the password ended every session, this one included; start a fresh one here.
		s.startSession(w, r, u.ID, "/account?saved=password")
	default:
		s.gone(w, r)
	}
}
