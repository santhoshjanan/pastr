package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/santhoshjanan/pastr/internal/config"
	"github.com/santhoshjanan/pastr/internal/store"
)

// loginFrom signs in through the trusted test proxy so each call can come from a chosen address.
func (e *testEnv) loginFrom(ip, username, password string) int {
	e.t.Helper()
	req, _ := http.NewRequest("POST", e.srv.URL+"/login", strings.NewReader(url.Values{"username": {username}, "password": {password}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", e.srv.URL)
	req.Header.Set("X-Forwarded-For", ip)
	resp, err := e.client().Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestStrangerCannotLockAnAccountOut(t *testing.T) {
	e := newTestServerWith(t, func(c *config.Config) { c.TrustedProxy = "127.0.0.1" })
	e.addUser("root", true)
	for i := 0; i < 12; i++ {
		e.loginFrom("203.0.113.66", "root", "wrong password")
	}
	if got := e.loginFrom("203.0.113.66", "root", testPassword); got != http.StatusTooManyRequests {
		t.Errorf("the guessing address = %d, want 429", got)
	}
	if got := e.loginFrom("198.51.100.7", "root", testPassword); got != http.StatusSeeOther {
		t.Errorf("the real owner, from another address = %d; a stranger's guesses locked them out", got)
	}
}

func TestOneAddressCannotSprayManyUsernames(t *testing.T) {
	e := newTestServerWith(t, func(c *config.Config) { c.TrustedProxy = "127.0.0.1" })
	e.addUser("ana", false)
	for i := 0; i < 25; i++ {
		e.loginFrom("203.0.113.66", "user"+string(rune('a'+i)), "wrong password")
	}
	if got := e.loginFrom("203.0.113.66", "ana", testPassword); got != http.StatusTooManyRequests {
		t.Errorf("26th attempt from one address = %d, want 429", got)
	}
}

func TestIPv6ThrottleKeyIsTheSlash64(t *testing.T) {
	e := newTestServerWith(t, func(c *config.Config) { c.TrustedProxy = "127.0.0.1" })
	r1, _ := http.NewRequest("GET", "/", nil)
	r1.RemoteAddr = "127.0.0.1:1"
	r1.Header.Set("X-Forwarded-For", "2001:db8:1:2:aaaa:bbbb:cccc:dddd")
	r2, _ := http.NewRequest("GET", "/", nil)
	r2.RemoteAddr = "127.0.0.1:1"
	r2.Header.Set("X-Forwarded-For", "2001:db8:1:2::1")
	if a, b := e.web.clientIP(r1), e.web.clientIP(r2); a != b || a != "2001:db8:1:2::" {
		t.Errorf("two addresses in one /64 gave keys %q and %q", a, b)
	}
}

func TestPrefetchIsNotAReusableSuccess(t *testing.T) {
	e := newTestServer(t)
	p := seed(e, e.addUser("ana", false), func(p *store.Paste) { p.MaxViews = intPtr(1) })
	req, _ := http.NewRequest("GET", e.srv.URL+"/p/"+p.Slug, nil)
	req.Header.Set("User-Agent", phoneUA)
	req.Header.Set("Sec-Purpose", "prefetch;prerender")
	resp, err := e.client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode < 400 || resp.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("prefetch = %d, Cache-Control %q; a browser could show this title-only page on the real visit",
			resp.StatusCode, resp.Header.Get("Cache-Control"))
	}
	if resp, body := e.getUA("/p/"+p.Slug, phoneUA); resp.StatusCode != 200 || !strings.Contains(body, "SPOILERBODY") {
		t.Errorf("the visit after a prefetch = %d", resp.StatusCode)
	}
}
