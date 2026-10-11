package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/santhoshjanan/pastr/internal/auth"
	"github.com/santhoshjanan/pastr/internal/config"
	"github.com/santhoshjanan/pastr/internal/store"
)

const testPassword = "correct horse battery"

type testEnv struct {
	t     *testing.T
	srv   *httptest.Server
	store *store.Store
	now   *time.Time
	web   *Server
}

func newTestServerWith(t *testing.T, mod func(*config.Config)) *testEnv {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0).UTC()
	e := &testEnv{t: t, store: st, now: &now}
	e.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.web.Handler().ServeHTTP(w, r)
	}))
	cfg := config.Config{BaseURL: e.srv.URL, Addr: ":0"}
	if mod != nil {
		mod(&cfg)
	}
	e.web, err = New(cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	e.web.SetClock(func() time.Time { return *e.now })
	t.Cleanup(func() { e.srv.Close(); st.Close() })
	return e
}

func newTestServer(t *testing.T) *testEnv { return newTestServerWith(t, nil) }

func (e *testEnv) client() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (e *testEnv) get(c *http.Client, path string) (*http.Response, string) {
	e.t.Helper()
	resp, err := c.Get(e.srv.URL + path)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// post sends a form with the Origin header a browser would send.
func (e *testEnv) post(c *http.Client, path string, form url.Values) *http.Response {
	e.t.Helper()
	req, _ := http.NewRequest("POST", e.srv.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", e.srv.URL)
	resp, err := c.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func (e *testEnv) addUser(username string, admin bool) store.User {
	e.t.Helper()
	h, _ := auth.HashPassword(testPassword)
	u, err := e.store.CreateUser(context.Background(), username, h, admin, *e.now)
	if err != nil {
		e.t.Fatal(err)
	}
	return u
}

func (e *testEnv) signIn(username string, admin bool) *http.Client {
	e.t.Helper()
	e.addUser(username, admin)
	c := e.client()
	resp := e.post(c, "/login", url.Values{"username": {username}, "password": {testPassword}})
	if resp.StatusCode != http.StatusSeeOther {
		e.t.Fatalf("sign-in status = %d", resp.StatusCode)
	}
	return c
}

var csrfRE = regexp.MustCompile(`name="csrf" value="([^"]+)"`)

func (e *testEnv) csrf(c *http.Client) string {
	e.t.Helper()
	_, body := e.get(c, "/account")
	m := csrfRE.FindStringSubmatch(body)
	if m == nil {
		e.t.Fatal("no csrf token on /account")
	}
	return m[1]
}
