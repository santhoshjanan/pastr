package web

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/santhoshjanan/pastr/internal/store"
)

var slugInLocation = regexp.MustCompile(`^/p/([0-9A-Za-z]{10})/ready`)

func (e *testEnv) create(c *http.Client, form url.Values) (*http.Response, string) {
	e.t.Helper()
	form.Set("csrf", e.csrf(c))
	resp := e.post(c, "/new", form)
	m := slugInLocation.FindStringSubmatch(resp.Header.Get("Location"))
	if m == nil {
		return resp, ""
	}
	return resp, m[1]
}

func validForm() url.Values {
	return url.Values{"title": {"The ending of Dune"}, "body": {"<div>He walks into the <strong>desert</strong>.</div>"},
		"reveal": {"tap"}, "expiry": {"1w"}, "max_views": {"50"}}
}

func TestCreateAndReady(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	resp, slug := e.create(c, validForm())
	if resp.StatusCode != http.StatusSeeOther || slug == "" {
		t.Fatalf("create = %d → %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	p, err := e.store.PasteBySlug(context.Background(), slug)
	if err != nil {
		t.Fatal(err)
	}
	if p.RevealMode != "tap" || p.MaxViews == nil || *p.MaxViews != 50 || p.ExpiresAt == nil || !p.ExpiresAt.Equal(e.now.Add(7*24*time.Hour)) {
		t.Errorf("stored paste = %+v", p)
	}
	_, body := e.get(c, "/p/"+slug+"/ready?e=paste-create")
	if !strings.Contains(body, e.srv.URL+"/p/"+slug) || !strings.Contains(body, `data-copy="`) {
		t.Error("ready page lacks the URL or the copy control")
	}
	if !strings.Contains(body, `data-track="paste-create"`) {
		t.Error("ready page lacks the create event")
	}
	if _, list := e.get(c, "/"); !strings.Contains(list, "The ending of Dune") {
		t.Error("new paste missing from the list")
	}
}

func TestCreateSanitisesBody(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	f := validForm()
	f.Set("body", `<div>ok<script>alert(1)</script><img src=x onerror=alert(1)></div>`)
	_, slug := e.create(c, f)
	p, err := e.store.PasteBySlug(context.Background(), slug)
	if err != nil || !strings.Contains(p.BodyHTML, "ok") {
		t.Fatalf("paste not stored: %q, %v", p.BodyHTML, err)
	}
	if strings.Contains(p.BodyHTML, "script") || strings.Contains(p.BodyHTML, "img") {
		t.Errorf("unsanitised body stored: %s", p.BodyHTML)
	}
}

func TestCreateRejectsBadInput(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	cases := map[string]func(url.Values){
		"empty editor (Review Focus 1)": func(f url.Values) { f.Set("body", "<div><br></div>") },
		"blank title":                   func(f url.Values) { f.Set("title", "  ") },
		"long title":                    func(f url.Values) { f.Set("title", strings.Repeat("a", 121)) },
		"body over 100 KB":              func(f url.Values) { f.Set("body", "<div>"+strings.Repeat("a", 102401)+"</div>") },
		"unknown reveal mode":           func(f url.Values) { f.Set("reveal", "blur") },
		"unknown expiry":                func(f url.Values) { f.Set("expiry", "2y") },
		"zero view limit":               func(f url.Values) { f.Set("max_views", "0") },
	}
	for name, mod := range cases {
		f := validForm()
		mod(f)
		resp, slug := e.create(c, f)
		if resp.StatusCode != http.StatusUnprocessableEntity || slug != "" {
			t.Errorf("%s: status %d, want 422", name, resp.StatusCode)
		}
	}
	f := validForm()
	f.Set("csrf", e.csrf(c))
	f.Set("body", "<div><br></div>")
	req, _ := http.NewRequest("POST", e.srv.URL+"/new", strings.NewReader(f.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", e.srv.URL)
	if resp, err := c.Do(req); err != nil {
		t.Fatal(err)
	} else {
		page, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !strings.Contains(string(page), "Write something first.") {
			t.Error("empty editor: the message does not say what to do")
		}
	}
	if list, _ := e.store.ListPastesByUser(context.Background(), 1, *e.now); len(list) != 0 {
		t.Errorf("%d pastes were created from invalid input", len(list))
	}
}

func TestEditKeepsOrResetsExpiry(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	_, slug := e.create(c, validForm())
	before, _ := e.store.PasteBySlug(context.Background(), slug)
	e.store.ConsumeView(context.Background(), slug, *e.now)
	*e.now = e.now.Add(48 * time.Hour)

	f := validForm()
	f.Set("csrf", e.csrf(c))
	f.Set("title", "Renamed")
	f.Set("expiry", "keep")
	if resp := e.post(c, "/p/"+slug+"/edit", f); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("edit = %d", resp.StatusCode)
	}
	after, _ := e.store.PasteBySlug(context.Background(), slug)
	if after.Title != "Renamed" || !after.ExpiresAt.Equal(*before.ExpiresAt) {
		t.Errorf("keep changed the expiry: %v → %v", before.ExpiresAt, after.ExpiresAt)
	}
	if after.ViewCount != 1 {
		t.Errorf("editing reset the view count: %d", after.ViewCount)
	}

	f.Set("csrf", e.csrf(c))
	f.Set("expiry", "1d")
	e.post(c, "/p/"+slug+"/edit", f)
	after, _ = e.store.PasteBySlug(context.Background(), slug)
	if !after.ExpiresAt.Equal(e.now.Add(24 * time.Hour)) {
		t.Errorf("new expiry must count from the edit: %v", after.ExpiresAt)
	}
}

func TestOnlyOwnerCanManage(t *testing.T) {
	e := newTestServer(t)
	ana := e.signIn("ana", false)
	_, slug := e.create(ana, validForm())
	bo := e.signIn("bo", false)

	for _, path := range []string{"/p/" + slug + "/edit", "/p/" + slug + "/ready"} {
		if resp, _ := e.get(bo, path); resp.StatusCode != http.StatusNotFound {
			t.Errorf("non-owner GET %s = %d, want 404", path, resp.StatusCode)
		}
	}
	f := validForm()
	f.Set("csrf", e.csrf(bo))
	if resp := e.post(bo, "/p/"+slug+"/edit", f); resp.StatusCode != http.StatusNotFound {
		t.Errorf("non-owner edit = %d, want 404", resp.StatusCode)
	}
	if resp := e.post(bo, "/p/"+slug+"/delete", url.Values{"csrf": {e.csrf(bo)}}); resp.StatusCode != http.StatusNotFound {
		t.Errorf("non-owner delete = %d, want 404", resp.StatusCode)
	}
	if _, err := e.store.PasteBySlug(context.Background(), slug); err != nil {
		t.Errorf("paste was changed by a non-owner: %v", err)
	}
	if resp, _ := e.get(e.client(), "/p/"+slug+"/edit"); resp.StatusCode != http.StatusSeeOther {
		t.Errorf("signed-out edit = %d, want redirect to login", resp.StatusCode)
	}
	if _, list := e.get(bo, "/"); strings.Contains(list, slug) {
		t.Error("another user's paste appears in the list")
	}
}

func TestDelete(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	_, slug := e.create(c, validForm())
	if resp := e.post(c, "/p/"+slug+"/delete", url.Values{"csrf": {e.csrf(c)}}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	if _, err := e.store.PasteBySlug(context.Background(), slug); err != store.ErrNotFound {
		t.Errorf("paste survived delete: %v", err)
	}
}

func TestEmptyListAndEditorAssets(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	if _, body := e.get(c, "/"); !strings.Contains(body, "Nothing here yet") {
		t.Error("empty state missing")
	}
	_, body := e.get(c, "/new")
	if !strings.Contains(body, "vendor/trix.umd.min.js") || !strings.Contains(body, "<trix-editor") {
		t.Error("editor not loaded on /new")
	}
	if _, body := e.get(c, "/"); strings.Contains(body, "trix") {
		t.Error("editor loaded on a page that does not need it")
	}
}

func TestEditorPagesGetAStyleNonceAndNothingElseDoes(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	nonceRE := regexp.MustCompile(`style-src 'self' 'nonce-([A-Za-z0-9_-]{16,})'`)

	resp, body := e.get(c, "/new")
	m := nonceRE.FindStringSubmatch(resp.Header.Get("Content-Security-Policy"))
	if m == nil {
		t.Fatalf("no style nonce in the editor page policy: %s", resp.Header.Get("Content-Security-Policy"))
	}
	if !strings.Contains(body, `<meta name="trix-csp-nonce" content="`+m[1]+`">`) {
		t.Error("editor page does not hand the nonce to Trix")
	}
	resp2, _ := e.get(c, "/new")
	if m2 := nonceRE.FindStringSubmatch(resp2.Header.Get("Content-Security-Policy")); m2 == nil || m2[1] == m[1] {
		t.Error("the nonce must change on every response")
	}

	_, slug := e.create(c, validForm())
	for _, path := range []string{"/p/" + slug, "/", "/login"} {
		resp, _ := e.get(e.client(), path)
		csp := resp.Header.Get("Content-Security-Policy")
		if strings.Contains(csp, "nonce-") || strings.Contains(csp, "unsafe-inline") {
			t.Errorf("%s has a relaxed style policy: %s", path, csp)
		}
	}
}
