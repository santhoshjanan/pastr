package web

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"

	"github.com/santhoshjanan/pastr/internal/store"
)

// pageData is what every template receives. Handlers fill the exported page fields;
// render fills the rest.
type pageData struct {
	Title      string
	Colour     string // signal colour: red, blue, green or orange
	Error      string
	Notice     string
	Track      string // Umami event to fire when the page loads
	TrackProps string // JSON object of event properties
	Data       any
	// An empty {{define}} cannot override a block in html/template, so pages that
	// must drop the navigation or our script say so here instead.
	NoNav    bool
	NoScript bool
	// StyleNonce is set only on editor pages; see Server.editorNonce.
	StyleNonce string

	User        *store.User
	CSRF        string
	UmamiScript string
	UmamiID     string
}

const msgBroke = "That's on our side. Try again in a moment."

var pageNames = []string{
	"gone.html", "error.html", "login.html", "signup.html", "account.html",
	"read.html", "preview.html",
	"list.html", "form.html", "ready.html", "admin.html",
}

func loadPages(s *Server) (map[string]*template.Template, error) {
	funcs := template.FuncMap{"asset": s.asset}
	pages := map[string]*template.Template{}
	for _, name := range pageNames {
		t, err := template.New("base.html").Funcs(funcs).ParseFS(templateFS, "templates/base.html", "templates/"+name)
		if err != nil {
			return nil, err
		}
		pages[name] = t
	}
	return pages, nil
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, page string, d pageData) {
	d.User = currentUser(r)
	if sess := currentSession(r); sess != nil {
		d.CSRF = sess.CSRFToken
	}
	if d.Colour == "" {
		d.Colour = "red"
	}
	d.UmamiScript, d.UmamiID = s.cfg.UmamiScriptURL, s.cfg.UmamiWebsiteID

	var buf bytes.Buffer
	if err := s.pages[page].ExecuteTemplate(&buf, "base", d); err != nil {
		s.log.Error("render", "page", page, "err", err)
		http.Error(w, "Something broke on our side.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// gone is the one response for anything a visitor may not see or that does not exist.
func (s *Server) gone(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Cache-Control", "no-store")
	s.render(w, r, http.StatusNotFound, "gone.html", pageData{Title: "This one's gone", Track: "gone", NoNav: true})
}

// fail maps a store or internal error to a response without leaking detail.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		s.gone(w, r)
		return
	}
	id, _ := r.Context().Value(keyRequestID).(string)
	// The route pattern is logged, not the path: paste URLs are secrets.
	s.log.Error("request failed", "request_id", id, "route", r.Pattern, "err", err)
	s.render(w, r, http.StatusInternalServerError, "error.html", pageData{Title: "Something broke", Error: msgBroke, NoNav: true, NoScript: true})
}
