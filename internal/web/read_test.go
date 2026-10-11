package web

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/santhoshjanan/pastr/internal/paste"
	"github.com/santhoshjanan/pastr/internal/store"
)

const phoneUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 Safari/604.1"

func seed(e *testEnv, owner store.User, mod func(*store.Paste)) store.Paste {
	e.t.Helper()
	slug, _ := paste.NewSlug()
	p := store.Paste{Slug: slug, UserID: owner.ID, Title: "The ending of Dune", BodyHTML: "<div>SPOILERBODY he walks into the desert.</div>",
		RevealMode: "immediate", CreatedAt: *e.now, UpdatedAt: *e.now}
	if mod != nil {
		mod(&p)
	}
	if err := e.store.CreatePaste(context.Background(), p); err != nil {
		e.t.Fatal(err)
	}
	return p
}

func (e *testEnv) getUA(path, ua string) (*http.Response, string) {
	req, _ := http.NewRequest("GET", e.srv.URL+path, nil)
	req.Header.Set("User-Agent", ua)
	resp, err := e.client().Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func TestReadImmediate(t *testing.T) {
	e := newTestServer(t)
	p := seed(e, e.addUser("ana", false), nil)
	resp, body := e.getUA("/p/"+p.Slug, phoneUA)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, "SPOILERBODY") || strings.Contains(body, "<details") {
		t.Error("immediate paste must show the body with no reveal control")
	}
	if strings.Contains(body, "events.js") || strings.Contains(body, "<script") {
		t.Error("read page must ship no script when Umami is off")
	}
	if resp.Header.Get("X-Robots-Tag") != "noindex, nofollow" || resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("paste headers wrong: %v", resp.Header)
	}
	if !strings.Contains(body, "signal-"+paste.SignalColour(p.Slug)) {
		t.Error("signal colour class missing")
	}
	got, _ := e.store.PasteBySlug(context.Background(), p.Slug)
	if got.ViewCount != 1 {
		t.Errorf("view_count = %d, want 1", got.ViewCount)
	}
}

func TestReadGated(t *testing.T) {
	e := newTestServer(t)
	p := seed(e, e.addUser("ana", false), func(p *store.Paste) { p.RevealMode = "tap" })
	_, body := e.getUA("/p/"+p.Slug, phoneUA)
	for _, want := range []string{"<details", "Show the text", `data-umami-event="reveal"`, `data-umami-event-slug="` + p.Slug + `"`, "SPOILERBODY"} {
		if !strings.Contains(body, want) {
			t.Errorf("gated page missing %q", want)
		}
	}
	if strings.Contains(body, "<details open") {
		t.Error("gated paste must start closed")
	}
}

func TestPreviewNeverLeaksBody(t *testing.T) {
	e := newTestServer(t)
	p := seed(e, e.addUser("ana", false), func(p *store.Paste) { p.MaxViews = intPtr(1) })

	resp, body := e.getUA("/p/"+p.Slug, "Twitterbot/1.0")
	if resp.StatusCode != 200 || strings.Contains(body, "SPOILERBODY") {
		t.Fatal("crawler received the body")
	}
	if !strings.Contains(body, `<meta property="og:title" content="The ending of Dune">`) {
		t.Error("crawler page lacks og:title")
	}
	got, _ := e.store.PasteBySlug(context.Background(), p.Slug)
	if got.ViewCount != 0 {
		t.Errorf("crawler counted as a view: %d", got.ViewCount)
	}

	_, body = e.getUA("/p/"+p.Slug, phoneUA)
	head := body[:strings.Index(body, "</head>")]
	if strings.Contains(head, "SPOILERBODY") || strings.Contains(head, "desert") {
		t.Error("body text leaked into <head> metadata")
	}
}

func TestViewLimitBurns(t *testing.T) {
	e := newTestServer(t)
	p := seed(e, e.addUser("ana", false), func(p *store.Paste) { p.MaxViews = intPtr(2) })
	for i := 1; i <= 2; i++ {
		if resp, body := e.getUA("/p/"+p.Slug, phoneUA); resp.StatusCode != 200 || !strings.Contains(body, "SPOILERBODY") {
			t.Fatalf("view %d = %d", i, resp.StatusCode)
		}
	}
	if resp, body := e.getUA("/p/"+p.Slug, phoneUA); resp.StatusCode != 404 || strings.Contains(body, "SPOILERBODY") {
		t.Errorf("third view = %d, want gone", resp.StatusCode)
	}
	if _, err := e.store.PasteBySlug(context.Background(), p.Slug); err != store.ErrNotFound {
		t.Errorf("burned paste not deleted: %v", err)
	}
}

func TestOwnerAndHeadDoNotCount(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	owner, _ := e.store.UserByUsername(context.Background(), "ana")
	p := seed(e, owner, func(p *store.Paste) { p.MaxViews = intPtr(1) })

	if resp, _ := e.get(c, "/p/"+p.Slug); resp.StatusCode != 200 {
		t.Fatalf("owner view = %d", resp.StatusCode)
	}
	req, _ := http.NewRequest("HEAD", e.srv.URL+"/p/"+p.Slug, nil)
	req.Header.Set("User-Agent", phoneUA)
	e.client().Do(req)
	got, _ := e.store.PasteBySlug(context.Background(), p.Slug)
	if got.ViewCount != 0 {
		t.Errorf("view_count = %d after owner GET and HEAD, want 0", got.ViewCount)
	}
}

func TestGoneIsUniform(t *testing.T) {
	e := newTestServer(t)
	ana := e.addUser("ana", false)
	past := e.now.Add(-time.Minute)
	expired := seed(e, ana, func(p *store.Paste) { p.ExpiresAt = &past })
	bo := e.addUser("bo", false)
	hidden := seed(e, bo, nil)
	e.store.SetDisabled(context.Background(), bo.ID, true)

	_, want := e.getUA("/p/0000000000", phoneUA)
	for name, slug := range map[string]string{"expired": expired.Slug, "owner disabled": hidden.Slug} {
		resp, body := e.getUA("/p/"+slug, phoneUA)
		if resp.StatusCode != 404 || body != want {
			t.Errorf("%s: status %d, or body differs from the unknown-slug page", name, resp.StatusCode)
		}
	}
	// Crawlers get the same answer, so a preview cannot confirm a paste once existed.
	if resp, _ := e.getUA("/p/"+expired.Slug, "Twitterbot/1.0"); resp.StatusCode != 404 {
		t.Errorf("crawler on expired paste = %d", resp.StatusCode)
	}
}

func TestMalformedSlugs(t *testing.T) {
	// Review Focus 3.
	e := newTestServer(t)
	for _, path := range []string{"/p/short", "/p/" + strings.Repeat("a", 500), "/p/aaaa-aaaaa", "/p/aaaaaaaaa%00", "/p/"} {
		if resp, _ := e.getUA(path, phoneUA); resp.StatusCode != 404 {
			t.Errorf("%s = %d, want 404", path, resp.StatusCode)
		}
	}
}

func TestHostileTitleIsEscaped(t *testing.T) {
	// Review Focus 2.
	e := newTestServer(t)
	title := `"><script>alert(1)</script> & 🎬`
	p := seed(e, e.addUser("ana", false), func(p *store.Paste) { p.Title = title })
	for _, ua := range []string{phoneUA, "Twitterbot/1.0"} {
		_, body := e.getUA("/p/"+p.Slug, ua)
		if strings.Contains(body, "<script>alert(1)") {
			t.Errorf("title rendered unescaped for %s", ua)
		}
		if !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, "🎬") {
			t.Errorf("escaped title not found for %s", ua)
		}
	}
}

func TestByline(t *testing.T) {
	e := newTestServer(t)
	ana := e.addUser("ana", false)
	p := seed(e, ana, nil)
	if _, body := e.getUA("/p/"+p.Slug, phoneUA); strings.Contains(body, "poster__byline") {
		t.Error("byline shown although none is set")
	}
	e.store.UpdateProfile(context.Background(), ana.ID, "Ana K", "https://x.com/ana")
	_, body := e.getUA("/p/"+p.Slug, phoneUA)
	if !strings.Contains(body, "Ana K") || !strings.Contains(body, `href="https://x.com/ana"`) {
		t.Error("byline missing")
	}
}

func intPtr(n int) *int { return &n }
