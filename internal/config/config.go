// Package config reads pastr's settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type Config struct {
	BaseURL        string
	Addr           string
	DBPath         string
	AdminUsername  string
	AdminPassword  string
	TrustedProxy   string
	UmamiScriptURL string
	UmamiWebsiteID string
}

func Load(getenv func(string) string) (Config, error) {
	c := Config{
		BaseURL:        strings.TrimRight(getenv("PASTR_BASE_URL"), "/"),
		Addr:           orDefault(getenv("PASTR_ADDR"), ":8080"),
		DBPath:         orDefault(getenv("PASTR_DB_PATH"), "/data/pastr.db"),
		AdminUsername:  getenv("PASTR_ADMIN_USERNAME"),
		AdminPassword:  getenv("PASTR_ADMIN_PASSWORD"),
		TrustedProxy:   getenv("PASTR_TRUSTED_PROXY"),
		UmamiScriptURL: getenv("PASTR_UMAMI_SCRIPT_URL"),
		UmamiWebsiteID: getenv("PASTR_UMAMI_WEBSITE_ID"),
	}
	if c.BaseURL == "" {
		return c, errors.New("PASTR_BASE_URL is required, for example https://paste.example.com")
	}
	u, err := absoluteURL(c.BaseURL)
	if err != nil {
		return c, fmt.Errorf("PASTR_BASE_URL: %w", err)
	}
	if u.Path != "" {
		return c, errors.New("PASTR_BASE_URL must not contain a path")
	}
	if (c.UmamiScriptURL == "") != (c.UmamiWebsiteID == "") {
		return c, errors.New("set both PASTR_UMAMI_SCRIPT_URL and PASTR_UMAMI_WEBSITE_ID, or neither")
	}
	if c.UmamiScriptURL != "" {
		if _, err := absoluteURL(c.UmamiScriptURL); err != nil {
			return c, fmt.Errorf("PASTR_UMAMI_SCRIPT_URL: %w", err)
		}
	}
	if (c.AdminUsername == "") != (c.AdminPassword == "") {
		return c, errors.New("set both PASTR_ADMIN_USERNAME and PASTR_ADMIN_PASSWORD, or neither")
	}
	return c, nil
}

func (c Config) UmamiEnabled() bool { return c.UmamiScriptURL != "" }

// UmamiOrigin returns scheme and host of the tracker, for the Content Security Policy.
func (c Config) UmamiOrigin() string {
	u, err := url.Parse(c.UmamiScriptURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func (c Config) SecureCookies() bool { return strings.HasPrefix(c.BaseURL, "https://") }

func absoluteURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("must be an absolute http or https URL")
	}
	return u, nil
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
