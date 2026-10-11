package web

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/santhoshjanan/pastr/internal/config"
)

func TestHealthAndSecurityHeaders(t *testing.T) {
	e := newTestServerWith(t, func(c *config.Config) {
		c.UmamiScriptURL, c.UmamiWebsiteID = "https://stats.example.com/script.js", "site-1"
	})
	resp, _ := e.get(e.client(), "/healthz")
	if resp.StatusCode != 200 {
		t.Fatalf("healthz = %d", resp.StatusCode)
	}
	resp, body := e.get(e.client(), "/login")
	csp := resp.Header.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'none'", "script-src 'self' https://stats.example.com", "frame-ancestors 'none'", "form-action 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing %q: %s", want, csp)
		}
	}
	if strings.Contains(csp, "unsafe-inline") {
		t.Errorf("CSP allows inline code: %s", csp)
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff")
	}
	if !strings.Contains(body, `src="https://stats.example.com/script.js"`) || !strings.Contains(body, `data-website-id="site-1"`) {
		t.Error("Umami tracker tag missing")
	}
}

func TestNoUmamiWhenUnset(t *testing.T) {
	e := newTestServer(t)
	_, body := e.get(e.client(), "/login")
	if strings.Contains(body, "data-website-id") {
		t.Error("tracker served although Umami is not configured")
	}
}

func TestSignInAndOut(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	if resp, _ := e.get(c, "/account"); resp.StatusCode != 200 {
		t.Fatalf("account after sign-in = %d", resp.StatusCode)
	}
	resp := e.post(c, "/logout", url.Values{"csrf": {e.csrf(c)}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("logout = %d", resp.StatusCode)
	}
	if resp, _ := e.get(c, "/account"); resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/login" {
		t.Errorf("account after logout = %d → %s", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestSessionCookieFlags(t *testing.T) {
	// Review Focus 4: an http base URL must not produce a Secure cookie.
	e := newTestServer(t) // httptest serves plain http
	e.addUser("ana", false)
	resp := e.post(e.client(), "/login", url.Values{"username": {"ana"}, "password": {testPassword}})
	var found bool
	for _, ck := range resp.Cookies() {
		if ck.Name != "pastr_session" {
			continue
		}
		found = true
		if ck.Secure {
			t.Error("Secure cookie on an http base URL; sign-in would silently fail")
		}
		if !ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode {
			t.Errorf("cookie flags wrong: %+v", ck)
		}
	}
	if !found {
		t.Fatal("no session cookie set")
	}

	e2 := newTestServerWith(t, func(c *config.Config) { c.BaseURL = "https://p.example.com" })
	e2.addUser("ana", false)
	req, _ := http.NewRequest("POST", e2.srv.URL+"/login", strings.NewReader(url.Values{"username": {"ana"}, "password": {testPassword}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://p.example.com")
	resp2, err := e2.client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	secure := false
	for _, ck := range resp2.Cookies() {
		if ck.Name == "pastr_session" {
			secure = ck.Secure
		}
	}
	if !secure {
		t.Error("https base URL must produce a Secure session cookie")
	}
}

func TestWrongPasswordAndUnknownUserLookTheSame(t *testing.T) {
	e := newTestServer(t)
	e.addUser("ana", false)
	bad := func(user string) (int, string) {
		req, _ := http.NewRequest("POST", e.srv.URL+"/login", strings.NewReader(url.Values{"username": {user}, "password": {"wrong password"}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", e.srv.URL)
		resp, err := e.client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, strings.ReplaceAll(string(b), user, "")
	}
	s1, b1 := bad("ana")
	s2, b2 := bad("nobody")
	if s1 != http.StatusUnauthorized || s1 != s2 || b1 != b2 {
		t.Errorf("responses differ: %d vs %d", s1, s2)
	}
	if !strings.Contains(b1, "That username or password isn&#39;t right") && !strings.Contains(b1, "That username or password isn't right") {
		t.Error("expected the friendly error message")
	}
}

func TestLoginThrottle(t *testing.T) {
	e := newTestServer(t)
	e.addUser("ana", false)
	c := e.client()
	for i := 0; i < 5; i++ {
		e.post(c, "/login", url.Values{"username": {"ana"}, "password": {"wrong password"}})
	}
	if resp := e.post(c, "/login", url.Values{"username": {"ana"}, "password": {testPassword}}); resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("6th attempt = %d, want 429 even with the right password", resp.StatusCode)
	}
	*e.now = e.now.Add(16 * time.Minute)
	if resp := e.post(c, "/login", url.Values{"username": {"ana"}, "password": {testPassword}}); resp.StatusCode != http.StatusSeeOther {
		t.Errorf("after the window = %d, want 303", resp.StatusCode)
	}
}

func TestOriginAndCSRF(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)

	req, _ := http.NewRequest("POST", e.srv.URL+"/account", strings.NewReader("display_name=x&csrf="+e.csrf(c)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://evil.example")
	if resp, _ := c.Do(req); resp.StatusCode != http.StatusForbidden {
		t.Errorf("foreign origin = %d, want 403", resp.StatusCode)
	}
	req, _ = http.NewRequest("POST", e.srv.URL+"/account", strings.NewReader("display_name=x&csrf="+e.csrf(c)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if resp, _ := c.Do(req); resp.StatusCode != http.StatusForbidden {
		t.Errorf("missing origin = %d, want 403", resp.StatusCode)
	}
	if resp := e.post(c, "/account", url.Values{"display_name": {"x"}, "csrf": {"wrong"}}); resp.StatusCode != http.StatusForbidden {
		t.Errorf("bad csrf = %d, want 403", resp.StatusCode)
	}
}

func TestSignupToggle(t *testing.T) {
	e := newTestServer(t)
	if resp, _ := e.get(e.client(), "/signup"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("signup while disabled = %d, want 404", resp.StatusCode)
	}
	if resp := e.post(e.client(), "/signup", url.Values{"username": {"bo"}, "password": {testPassword}}); resp.StatusCode != http.StatusNotFound {
		t.Errorf("signup POST while disabled = %d, want 404", resp.StatusCode)
	}
	e.store.SetSignupsEnabled(context.Background(), true)
	c := e.client()
	if resp := e.post(c, "/signup", url.Values{"username": {"Bo_1"}, "password": {testPassword}}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("signup = %d", resp.StatusCode)
	}
	u, err := e.store.UserByUsername(context.Background(), "bo_1")
	if err != nil || u.IsAdmin {
		t.Errorf("new user = %+v, %v; must exist and not be admin", u, err)
	}
	if resp := e.post(e.client(), "/signup", url.Values{"username": {"bo_1"}, "password": {testPassword}}); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("duplicate signup = %d, want 422", resp.StatusCode)
	}
	if resp := e.post(e.client(), "/signup", url.Values{"username": {"cy"}, "password": {"short"}}); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("weak password = %d, want 422", resp.StatusCode)
	}
}

func TestAccountUpdates(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	resp := e.post(c, "/account", url.Values{"csrf": {e.csrf(c)}, "action": {"profile"}, "display_name": {"Ana K"}, "display_link": {"https://x.com/ana"}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("profile save = %d", resp.StatusCode)
	}
	u, _ := e.store.UserByUsername(context.Background(), "ana")
	if u.DisplayName != "Ana K" || u.DisplayLink != "https://x.com/ana" {
		t.Errorf("profile = %+v", u)
	}
	resp = e.post(c, "/account", url.Values{"csrf": {e.csrf(c)}, "action": {"profile"}, "display_link": {"javascript:alert(1)"}})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("javascript: link = %d, want 422", resp.StatusCode)
	}
	resp = e.post(c, "/account", url.Values{"csrf": {e.csrf(c)}, "action": {"password"}, "current": {"wrong password"}, "new": {"another good one"}})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("wrong current password = %d, want 422", resp.StatusCode)
	}
	resp = e.post(c, "/account", url.Values{"csrf": {e.csrf(c)}, "action": {"password"}, "current": {testPassword}, "new": {"another good one"}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("password change = %d", resp.StatusCode)
	}
}

func TestUnknownRouteIsGone(t *testing.T) {
	e := newTestServer(t)
	resp, body := e.get(e.client(), "/nope")
	if resp.StatusCode != http.StatusNotFound || !strings.Contains(body, "This one&#39;s gone") && !strings.Contains(body, "This one's gone") {
		t.Errorf("unknown route = %d", resp.StatusCode)
	}
}

func TestPasswordChangeKeepsThisDeviceAndEndsOthers(t *testing.T) {
	e := newTestServer(t)
	here := e.signIn("ana", false)
	elsewhere := e.client()
	e.post(elsewhere, "/login", url.Values{"username": {"ana"}, "password": {testPassword}})

	resp := e.post(here, "/account", url.Values{"csrf": {e.csrf(here)}, "action": {"password"}, "current": {testPassword}, "new": {"another good one"}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("password change = %d", resp.StatusCode)
	}
	if resp, _ := e.get(here, "/account"); resp.StatusCode != http.StatusOK {
		t.Errorf("the device that changed the password was signed out: %d", resp.StatusCode)
	}
	if resp, _ := e.get(elsewhere, "/account"); resp.StatusCode != http.StatusSeeOther {
		t.Errorf("another device stayed signed in after the password change: %d", resp.StatusCode)
	}
}
