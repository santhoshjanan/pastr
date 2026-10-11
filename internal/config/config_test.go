package config

import "testing"

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(map[string]string{"PASTR_BASE_URL": "https://p.example.com/"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.BaseURL != "https://p.example.com" {
		t.Errorf("BaseURL = %q, want trailing slash removed", c.BaseURL)
	}
	if c.Addr != ":8080" || c.DBPath != "/data/pastr.db" {
		t.Errorf("defaults wrong: %+v", c)
	}
	if c.UmamiEnabled() {
		t.Error("Umami should be off when unset")
	}
	if !c.SecureCookies() {
		t.Error("https base URL should mean secure cookies")
	}
}

func TestLoadErrors(t *testing.T) {
	cases := map[string]map[string]string{
		"missing base url":  {},
		"relative base url": {"PASTR_BASE_URL": "p.example.com"},
		"base url has path": {"PASTR_BASE_URL": "https://example.com/pastr"},
		"half umami":        {"PASTR_BASE_URL": "https://a.b", "PASTR_UMAMI_WEBSITE_ID": "x"},
		"bad umami url":     {"PASTR_BASE_URL": "https://a.b", "PASTR_UMAMI_WEBSITE_ID": "x", "PASTR_UMAMI_SCRIPT_URL": "nope"},
		"half admin":        {"PASTR_BASE_URL": "https://a.b", "PASTR_ADMIN_USERNAME": "me"},
	}
	for name, m := range cases {
		if _, err := Load(env(m)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestUmamiOriginAndHTTP(t *testing.T) {
	c, err := Load(env(map[string]string{
		"PASTR_BASE_URL":         "http://localhost:8080",
		"PASTR_UMAMI_SCRIPT_URL": "https://stats.example.com/script.js",
		"PASTR_UMAMI_WEBSITE_ID": "abc",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.UmamiOrigin(); got != "https://stats.example.com" {
		t.Errorf("UmamiOrigin = %q", got)
	}
	if c.SecureCookies() {
		t.Error("http base URL must not force secure cookies")
	}
}
