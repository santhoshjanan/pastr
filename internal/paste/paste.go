// Package paste holds the rules for pastes that do not need the database.
package paste

import (
	"crypto/rand"
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	SlugLen       = 10
	MaxTitleChars = 120
	MaxBodyBytes  = 100 * 1024
	maxViewLimit  = 1_000_000

	alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var colours = [...]string{"red", "blue", "green", "orange"}

// NewSlug draws 10 base62 characters from crypto/rand without modulo bias.
func NewSlug() (string, error) {
	out := make([]byte, 0, SlugLen)
	buf := make([]byte, SlugLen*2)
	for len(out) < SlugLen {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if b < 248 && len(out) < SlugLen { // 248 = 62 * 4
				out = append(out, alphabet[int(b)%62])
			}
		}
	}
	return string(out), nil
}

func ValidSlug(s string) bool {
	if len(s) != SlugLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(alphabet, rune(s[i])) {
			return false
		}
	}
	return true
}

// SignalColour gives each paste one of four colours that never changes.
func SignalColour(slug string) string {
	h := fnv.New32a()
	h.Write([]byte(slug))
	return colours[h.Sum32()%uint32(len(colours))]
}

// ValidateTitle collapses whitespace and enforces the length limit in characters.
func ValidateTitle(raw string) (string, error) {
	title := strings.Join(strings.Fields(raw), " ")
	if title == "" {
		return "", errors.New("Give it a title. It's what people see before they open it.")
	}
	if utf8.RuneCountInString(title) > MaxTitleChars {
		return "", fmt.Errorf("Keep the title under %d characters.", MaxTitleChars)
	}
	return title, nil
}

func ExpiresAt(choice string, now time.Time) (*time.Time, error) {
	var d time.Duration
	switch choice {
	case "never":
		return nil, nil
	case "1d":
		d = 24 * time.Hour
	case "1w":
		d = 7 * 24 * time.Hour
	case "1m":
		d = 30 * 24 * time.Hour
	default:
		return nil, errors.New("Pick how long this should last.")
	}
	t := now.Add(d).UTC()
	return &t, nil
}

func ParseMaxViews(raw string) (*int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxViewLimit {
		return nil, errors.New("The view limit is a whole number, 1 or more. Leave it blank for no limit.")
	}
	return &n, nil
}

// TimeLeft returns a short phrase such as "gone in 3 days", or "" when there is no expiry.
func TimeLeft(expires *time.Time, now time.Time) string {
	if expires == nil {
		return ""
	}
	d := expires.Sub(now)
	switch {
	case d < time.Minute:
		return "gone in under a minute"
	case d < time.Hour:
		return "gone in " + plural(int(d/time.Minute), "minute")
	case d < 24*time.Hour:
		return "gone in " + plural(int(d/time.Hour), "hour")
	}
	return "gone in " + plural(int(d/(24*time.Hour)), "day")
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return strconv.Itoa(n) + " " + unit + "s"
}
