package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/santhoshjanan/pastr/internal/config"
	"github.com/santhoshjanan/pastr/internal/store"
)

func TestEditCannotLowerLimitBelowViews(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	_, slug := e.create(c, validForm())
	for i := 0; i < 3; i++ {
		e.store.ConsumeView(context.Background(), slug, *e.now)
	}
	f := validForm()
	f.Set("csrf", e.csrf(c))
	f.Set("expiry", "keep")
	f.Set("max_views", "3")
	if resp := e.post(c, "/p/"+slug+"/edit", f); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("limit equal to the views already used = %d, want 422", resp.StatusCode)
	}
	if p, err := e.store.PasteBySlug(context.Background(), slug); err != nil || *p.MaxViews != 50 {
		t.Errorf("paste changed by a rejected edit: %+v, %v", p, err)
	}
	f.Set("csrf", e.csrf(c))
	f.Set("max_views", "4")
	if resp := e.post(c, "/p/"+slug+"/edit", f); resp.StatusCode != http.StatusSeeOther {
		t.Errorf("limit above the views used = %d, want 303", resp.StatusCode)
	}
}

func TestNonASCIIBodyUnderLimitIsAccepted(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	f := validForm()
	f.Set("body", "<div>"+strings.Repeat("த", 30000)+"</div>") // 90 KB of UTF-8, about 270 KB once form-encoded
	if resp, slug := e.create(c, f); slug == "" {
		t.Errorf("90 KB non-ASCII body rejected with %d", resp.StatusCode)
	}
}

func TestBodyLimitAppliesAfterSanitising(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	f := validForm()
	f.Set("body", strings.Repeat(`<a href="https://a.co/x">l</a>`, 3200)) // 96 KB in, several times that once link attributes are added
	if resp, slug := e.create(c, f); slug != "" || resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("body that grows past the limit when sanitised = %d (slug %q), want 422", resp.StatusCode, slug)
	}
}

func TestAutomatedFetchersDoNotBurnViews(t *testing.T) {
	e := newTestServer(t)
	p := seed(e, e.addUser("ana", false), func(p *store.Paste) { p.MaxViews = intPtr(1) })
	for _, ua := range []string{"", "curl/8.7.1", "Go-http-client/2.0", "python-requests/2.32", "Synapse (bot; +https://github.com/matrix-org/synapse)", "Mozilla/5.0 (compatible; Google-InspectionTool/1.0;)"} {
		if _, body := e.getUA("/p/"+p.Slug, ua); strings.Contains(body, "SPOILERBODY") {
			t.Errorf("fetcher %q received the body", ua)
		}
	}
	req, _ := http.NewRequest("GET", e.srv.URL+"/p/"+p.Slug, nil)
	req.Header.Set("User-Agent", phoneUA)
	req.Header.Set("Sec-Purpose", "prefetch")
	resp, err := e.client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got, _ := e.store.PasteBySlug(context.Background(), p.Slug); got.ViewCount != 0 {
		t.Errorf("automated fetches used %d views", got.ViewCount)
	}
	if resp, body := e.getUA("/p/"+p.Slug, phoneUA); resp.StatusCode != 200 || !strings.Contains(body, "SPOILERBODY") {
		t.Errorf("the one real reader got %d", resp.StatusCode)
	}
	// A phone whose model name happens to contain "bot" is still a person.
	p2 := seed(e, e.addUser("bo", false), nil)
	for _, model := range []string{"CUBOT_NOTE_30", "CUBOT NOTE 30", "CUBOT KINGKONG 9 Build/TP1A"} {
		ua := "Mozilla/5.0 (Linux; Android 13; " + model + ") AppleWebKit/537.36 Chrome/126.0 Mobile Safari/537.36"
		if _, body := e.getUA("/p/"+p2.Slug, ua); !strings.Contains(body, "SPOILERBODY") {
			t.Errorf("a %s phone was treated as a crawler", model)
		}
	}
	if _, body := e.getUA("/p/"+p2.Slug, "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 [Pinterest/iOS]"); !strings.Contains(body, "SPOILERBODY") {
		t.Error("Pinterest's in-app browser was treated as a crawler")
	}
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		name, trusted, remote string
		xff                   []string
		want                  string
	}{
		{"no proxy configured ignores the header", "", "203.0.113.9:5000", []string{"1.2.3.4"}, "203.0.113.9"},
		{"untrusted peer ignores the header", "10.0.0.1", "203.0.113.9:5000", []string{"1.2.3.4"}, "203.0.113.9"},
		{"trusted peer, last entry wins", "10.0.0.1", "10.0.0.1:5000", []string{"6.6.6.6, 198.51.100.7"}, "198.51.100.7"},
		{"attacker's own header line cannot come first", "10.0.0.1", "10.0.0.1:5000", []string{"6.6.6.6", "198.51.100.7"}, "198.51.100.7"},
		{"CIDR range", "172.18.0.0/16", "172.18.0.5:5000", []string{"198.51.100.7"}, "198.51.100.7"},
		{"IPv4-mapped peer", "10.0.0.1", "[::ffff:10.0.0.1]:5000", []string{"198.51.100.7"}, "198.51.100.7"},
		{"garbage forwarded value falls back to the peer", "10.0.0.1", "10.0.0.1:5000", []string{"not-an-ip"}, "10.0.0.1"},
	}
	for _, c := range cases {
		e := newTestServerWith(t, func(cfg *config.Config) { cfg.TrustedProxy = c.trusted })
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remote
		for _, v := range c.xff {
			r.Header.Add("X-Forwarded-For", v)
		}
		if got := e.web.clientIP(r); got != c.want {
			t.Errorf("%s: clientIP = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestGoneIsNotCacheableAndStaticHasNoListing(t *testing.T) {
	e := newTestServer(t)
	resp, _ := e.get(e.client(), "/p/0000000000")
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("gone page Cache-Control = %q", resp.Header.Get("Cache-Control"))
	}
	for _, dir := range []string{"", "vendor/"} {
		resp, body := e.get(e.client(), e.web.asset(dir))
		if resp.StatusCode != http.StatusNotFound || strings.Contains(body, "trix.css") || strings.Contains(body, "editor.js") {
			t.Errorf("static directory %q listed: %d", dir, resp.StatusCode)
		}
	}
	if resp, _ := e.get(e.client(), e.web.asset("app.css")); resp.StatusCode != 200 {
		t.Errorf("stylesheet = %d", resp.StatusCode)
	}
}

func TestSignupWhenSignedInAndThrottleOnlyCountsRealAttempts(t *testing.T) {
	e := newTestServer(t)
	e.store.SetSignupsEnabled(context.Background(), true)
	c := e.signIn("ana", false)
	if resp, _ := e.get(c, "/signup"); resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Errorf("signed-in GET /signup = %d → %q, want a redirect home", resp.StatusCode, resp.Header.Get("Location"))
	}
	for i := 0; i < 8; i++ {
		e.post(e.client(), "/signup", url.Values{"username": {"cyd"}, "password": {"short"}})
	}
	if resp := e.post(e.client(), "/signup", url.Values{"username": {"cyd"}, "password": {testPassword}}); resp.StatusCode != http.StatusSeeOther {
		t.Errorf("sign-up after typos = %d; validation errors must not use up the limit", resp.StatusCode)
	}
}

func TestOverLongBodyIsHandedBackToTheAuthor(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	f := validForm()
	f.Set("csrf", e.csrf(c))
	f.Set("body", "<div>KEEPMYWORDS "+strings.Repeat("a", 110*1024)+"</div>")
	req, _ := http.NewRequest("POST", e.srv.URL+"/new", strings.NewReader(f.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", e.srv.URL)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(body), "KEEPMYWORDS") {
		t.Errorf("status %d; the editor came back without the author's text", resp.StatusCode)
	}
}
