package web

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestAdminIsHiddenFromOthers(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("ana", false)
	if resp, _ := e.get(c, "/admin"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("non-admin GET /admin = %d, want 404", resp.StatusCode)
	}
	if resp := e.post(c, "/admin", url.Values{"csrf": {e.csrf(c)}, "action": {"signups"}, "on": {"true"}}); resp.StatusCode != http.StatusNotFound {
		t.Errorf("non-admin POST /admin = %d, want 404", resp.StatusCode)
	}
	if on, _ := e.store.SignupsEnabled(context.Background()); on {
		t.Error("a non-admin opened sign-ups")
	}
	if _, body := e.get(c, "/"); strings.Contains(body, `href="/admin"`) {
		t.Error("admin link shown to a non-admin")
	}
}

func TestAdminSignupSwitch(t *testing.T) {
	e := newTestServer(t)
	c := e.signIn("root", true)
	e.post(c, "/admin", url.Values{"csrf": {e.csrf(c)}, "action": {"signups"}, "on": {"true"}})
	if on, _ := e.store.SignupsEnabled(context.Background()); !on {
		t.Error("sign-ups not enabled")
	}
	e.post(c, "/admin", url.Values{"csrf": {e.csrf(c)}, "action": {"signups"}, "on": {"false"}})
	if on, _ := e.store.SignupsEnabled(context.Background()); on {
		t.Error("sign-ups not disabled")
	}
}

func TestAdminDisableEnableAndReset(t *testing.T) {
	e := newTestServer(t)
	admin := e.signIn("root", true)
	userClient := e.signIn("ana", false)
	ana, _ := e.store.UserByUsername(context.Background(), "ana")
	id := strconv.FormatInt(ana.ID, 10)
	p := seed(e, ana, nil)

	e.post(admin, "/admin", url.Values{"csrf": {e.csrf(admin)}, "action": {"disable"}, "user": {id}})
	if resp, _ := e.get(userClient, "/account"); resp.StatusCode != http.StatusSeeOther {
		t.Errorf("disabled user still signed in: %d", resp.StatusCode)
	}
	if resp, _ := e.getUA("/p/"+p.Slug, phoneUA); resp.StatusCode != http.StatusNotFound {
		t.Errorf("disabled user's paste still readable: %d", resp.StatusCode)
	}
	if resp := e.post(e.client(), "/login", url.Values{"username": {"ana"}, "password": {testPassword}}); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("disabled user signed in: %d", resp.StatusCode)
	}

	e.post(admin, "/admin", url.Values{"csrf": {e.csrf(admin)}, "action": {"enable"}, "user": {id}})
	if resp, _ := e.getUA("/p/"+p.Slug, phoneUA); resp.StatusCode != 200 {
		t.Errorf("paste not restored after enabling: %d", resp.StatusCode)
	}

	resp := e.post(admin, "/admin", url.Values{"csrf": {e.csrf(admin)}, "action": {"reset"}, "user": {id}, "password": {"short"}})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("weak reset password = %d, want 422", resp.StatusCode)
	}
	e.post(admin, "/admin", url.Values{"csrf": {e.csrf(admin)}, "action": {"reset"}, "user": {id}, "password": {"a brand new password"}})
	if resp := e.post(e.client(), "/login", url.Values{"username": {"ana"}, "password": {"a brand new password"}}); resp.StatusCode != http.StatusSeeOther {
		t.Errorf("sign-in with the reset password = %d", resp.StatusCode)
	}
}

func TestAdminCannotLockThemselvesOut(t *testing.T) {
	// Review Focus 5.
	e := newTestServer(t)
	c := e.signIn("root", true)
	root, _ := e.store.UserByUsername(context.Background(), "root")
	resp := e.post(c, "/admin", url.Values{"csrf": {e.csrf(c)}, "action": {"disable"}, "user": {strconv.FormatInt(root.ID, 10)}})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("self-disable = %d, want 422", resp.StatusCode)
	}
	if got, _ := e.store.UserByID(context.Background(), root.ID); got.Disabled {
		t.Error("the only admin disabled their own account")
	}
}
