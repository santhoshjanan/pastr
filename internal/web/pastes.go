package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/santhoshjanan/pastr/internal/paste"
	"github.com/santhoshjanan/pastr/internal/sanitize"
	"github.com/santhoshjanan/pastr/internal/store"
)

type listRow struct {
	Paste    store.Paste
	URL      string
	Colour   string
	TimeLeft string
}

type formData struct {
	Editing  bool
	Slug     string
	Title    string
	Body     string // sanitised HTML handed back to the editor
	Reveal   string
	Expiry   string
	MaxViews string
	TimeLeft string
}

type readyData struct {
	Paste store.Paste
	URL   string
}

// Events a redirect may ask the next page to report. Anything else is ignored.
var trackable = map[string]bool{"login": true, "signup": true, "paste-create": true, "paste-edit": true, "paste-delete": true}

func trackFrom(r *http.Request) string {
	if e := r.URL.Query().Get("e"); trackable[e] {
		return e
	}
	return ""
}

func trackProps(p store.Paste, expiry string) string {
	b, _ := json.Marshal(map[string]any{"slug": p.Slug, "reveal": p.RevealMode, "expiry": expiry, "view_limit": p.MaxViews != nil})
	return string(b)
}

// requireOwner loads the paste in the URL and continues only for its owner.
// Everyone else gets the gone page, the same as for a paste that does not exist.
func (s *Server) requireOwner(next func(http.ResponseWriter, *http.Request, store.Paste)) http.HandlerFunc {
	return s.requireUser(func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		if !paste.ValidSlug(slug) {
			s.gone(w, r)
			return
		}
		p, err := s.store.PasteBySlug(r.Context(), slug)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if p.UserID != currentUser(r).ID {
			s.gone(w, r)
			return
		}
		next(w, r, p)
	})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	pastes, err := s.store.ListPastesByUser(r.Context(), currentUser(r).ID, s.now())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	rows := make([]listRow, 0, len(pastes))
	for _, p := range pastes {
		rows = append(rows, listRow{Paste: p, URL: s.cfg.BaseURL + "/p/" + p.Slug,
			Colour: paste.SignalColour(p.Slug), TimeLeft: paste.TimeLeft(p.ExpiresAt, s.now())})
	}
	s.render(w, r, http.StatusOK, "list.html", pageData{Title: "My pastes", Track: trackFrom(r), TrackProps: "{}", Data: rows})
}

func (s *Server) handleNewForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "form.html", pageData{Title: "New paste", Data: formData{Reveal: "tap", Expiry: "never"}})
}

// readForm validates the submitted paste fields onto p. The returned formData
// echoes what was typed, for re-rendering the form with a message.
func (s *Server) readForm(r *http.Request, p *store.Paste, editing bool) (formData, error) {
	fd := formData{Editing: editing, Slug: p.Slug, Title: r.PostFormValue("title"), Reveal: r.PostFormValue("reveal"),
		Expiry: r.PostFormValue("expiry"), MaxViews: r.PostFormValue("max_views")}
	raw := r.PostFormValue("body")
	if len(raw) > paste.MaxBodyBytes {
		return fd, errors.New("That's too long. Keep it under 100 KB.")
	}
	body := sanitize.HTML(raw)
	fd.Body = body

	title, err := paste.ValidateTitle(fd.Title)
	if err != nil {
		return fd, err
	}
	if sanitize.IsBlank(body) {
		return fd, errors.New("Write something first.")
	}
	if fd.Reveal != "immediate" && fd.Reveal != "tap" {
		return fd, errors.New("Pick how the text should appear.")
	}
	maxViews, err := paste.ParseMaxViews(fd.MaxViews)
	if err != nil {
		return fd, err
	}
	if !(editing && fd.Expiry == "keep") {
		exp, err := paste.ExpiresAt(fd.Expiry, s.now())
		if err != nil {
			return fd, err
		}
		p.ExpiresAt = exp
	}
	p.Title, p.BodyHTML, p.RevealMode, p.MaxViews, p.UpdatedAt = title, body, fd.Reveal, maxViews, s.now()
	return fd, nil
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	p := store.Paste{UserID: currentUser(r).ID, CreatedAt: s.now()}
	fd, err := s.readForm(r, &p, false)
	if err != nil {
		s.render(w, r, http.StatusUnprocessableEntity, "form.html", pageData{Title: "New paste", Error: err.Error(), Data: fd})
		return
	}
	// A slug collision is astronomically unlikely; retry a few times all the same.
	for attempt := 0; ; attempt++ {
		if p.Slug, err = paste.NewSlug(); err != nil {
			s.fail(w, r, err)
			return
		}
		err = s.store.CreatePaste(r.Context(), p)
		if !errors.Is(err, store.ErrConflict) || attempt == 4 {
			break
		}
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/p/"+p.Slug+"/ready?e=paste-create&x="+fd.Expiry, http.StatusSeeOther)
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request, p store.Paste) {
	s.render(w, r, http.StatusOK, "ready.html", pageData{
		Title: "Link ready", Colour: paste.SignalColour(p.Slug),
		Track: trackFrom(r), TrackProps: trackProps(p, expiryLabel(r.URL.Query().Get("x"))),
		Data: readyData{Paste: p, URL: s.cfg.BaseURL + "/p/" + p.Slug},
	})
}

// expiryLabel keeps event properties to the known choices.
func expiryLabel(x string) string {
	switch x {
	case "1d", "1w", "1m", "never", "keep":
		return x
	}
	return ""
}

func (s *Server) handleEditForm(w http.ResponseWriter, r *http.Request, p store.Paste) {
	fd := formData{Editing: true, Slug: p.Slug, Title: p.Title, Body: p.BodyHTML, Reveal: p.RevealMode,
		Expiry: "keep", TimeLeft: paste.TimeLeft(p.ExpiresAt, s.now())}
	if p.MaxViews != nil {
		fd.MaxViews = strconv.Itoa(*p.MaxViews)
	}
	s.render(w, r, http.StatusOK, "form.html", pageData{Title: "Edit paste", Colour: paste.SignalColour(p.Slug), Data: fd})
}

func (s *Server) handleEdit(w http.ResponseWriter, r *http.Request, p store.Paste) {
	fd, err := s.readForm(r, &p, true)
	if err != nil {
		s.render(w, r, http.StatusUnprocessableEntity, "form.html",
			pageData{Title: "Edit paste", Colour: paste.SignalColour(p.Slug), Error: err.Error(), Data: fd})
		return
	}
	if err := s.store.UpdatePaste(r.Context(), p); err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/p/"+p.Slug+"/ready?e=paste-edit&x="+fd.Expiry, http.StatusSeeOther)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, p store.Paste) {
	if err := s.store.DeletePaste(r.Context(), p.Slug, p.UserID); err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/?e=paste-delete", http.StatusSeeOther)
}
