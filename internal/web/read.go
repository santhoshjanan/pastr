package web

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/santhoshjanan/pastr/internal/paste"
	"github.com/santhoshjanan/pastr/internal/store"
)

type readData struct {
	Paste    store.PasteView
	Body     template.HTML // sanitised when the paste was saved
	Gated    bool
	TimeLeft string
	Date     string
	URL      string
}

// isPrefetch reports a browser or proxy loading the page ahead of a visit that may never happen.
func isPrefetch(r *http.Request) bool {
	for _, h := range []string{"Sec-Purpose", "Purpose", "X-Purpose", "X-Moz"} {
		if v := strings.ToLower(r.Header.Get(h)); strings.Contains(v, "prefetch") || strings.Contains(v, "preview") {
			return true
		}
	}
	return false
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !paste.ValidSlug(slug) {
		s.gone(w, r)
		return
	}
	v, err := s.store.ReadPaste(r.Context(), slug, s.now())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	h := w.Header()
	h.Set("X-Robots-Tag", "noindex, nofollow")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-store")

	url := s.cfg.BaseURL + "/p/" + slug
	// Link-preview fetchers get the title only, and are not readers.
	if paste.IsPreviewCrawler(r.UserAgent()) || isPrefetch(r) {
		s.render(w, r, http.StatusOK, "preview.html",
			pageData{Title: v.Title, Colour: paste.SignalColour(slug), NoNav: true, NoScript: true, Data: readData{Paste: v, URL: url}})
		return
	}

	// The owner looking at their own paste, and HEAD requests, do not use up a view.
	owner := currentUser(r) != nil && currentUser(r).ID == v.UserID
	if r.Method == http.MethodGet && !owner {
		// Counting re-checks every condition, so a reader who lost a race for the
		// last view gets the gone page instead of the body loaded above.
		if v, err = s.store.ConsumeView(r.Context(), slug, s.now()); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	s.render(w, r, http.StatusOK, "read.html", pageData{Title: v.Title, Colour: paste.SignalColour(slug), NoNav: true, NoScript: true, Data: readData{
		Paste: v, Body: template.HTML(v.BodyHTML), Gated: v.RevealMode == "tap",
		TimeLeft: paste.TimeLeft(v.ExpiresAt, s.now()), Date: v.CreatedAt.Format("2 January 2006"), URL: url,
	}})
}
