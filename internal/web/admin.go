package web

import (
	"net/http"
	"strconv"

	"github.com/santhoshjanan/pastr/internal/auth"
	"github.com/santhoshjanan/pastr/internal/store"
)

type adminData struct {
	SignupsOn bool
	Users     []store.User
}

func (s *Server) adminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin", s.requireAdmin(func(w http.ResponseWriter, r *http.Request) { s.renderAdmin(w, r, http.StatusOK, "") }))
	mux.HandleFunc("POST /admin", s.requireAdmin(s.handleAdmin))
}

func (s *Server) renderAdmin(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	on, err := s.store.SignupsEnabled(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := pageData{Title: "Admin", Error: errMsg, Data: adminData{SignupsOn: on, Users: users}}
	if r.URL.Query().Get("saved") == "1" && errMsg == "" {
		d.Notice = "Saved."
	}
	s.render(w, r, status, "admin.html", d)
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	ctx, me := r.Context(), currentUser(r)
	reject := func(msg string) { s.renderAdmin(w, r, http.StatusUnprocessableEntity, msg) }

	action := r.PostFormValue("action")
	if action == "signups" {
		if err := s.store.SetSignupsEnabled(ctx, r.PostFormValue("on") == "true"); err != nil {
			s.fail(w, r, err)
			return
		}
		http.Redirect(w, r, "/admin?saved=1", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("user"), 10, 64)
	if err != nil {
		s.gone(w, r)
		return
	}
	target, err := s.store.UserByID(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	switch action {
	case "disable":
		if target.ID == me.ID {
			reject("You can't disable your own account.")
			return
		}
		if target.IsAdmin {
			n, err := s.store.CountEnabledAdmins(ctx)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			if n <= 1 {
				reject("That's the last admin. Someone has to hold the keys.")
				return
			}
		}
		err = s.store.SetDisabled(ctx, target.ID, true)
	case "enable":
		err = s.store.SetDisabled(ctx, target.ID, false)
	case "reset":
		pw := r.PostFormValue("password")
		if verr := auth.ValidatePassword(pw); verr != nil {
			reject(verr.Error())
			return
		}
		var hash string
		if hash, err = auth.HashPassword(pw); err == nil {
			err = s.store.UpdatePassword(ctx, target.ID, hash) // also ends the user's sessions
		}
		if err == nil && target.ID == me.ID {
			s.startSession(w, r, me.ID, "/admin?saved=1") // keep the admin signed in here
			return
		}
	default:
		s.gone(w, r)
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin?saved=1", http.StatusSeeOther)
}
