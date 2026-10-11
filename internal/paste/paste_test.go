package paste

import (
	"strings"
	"testing"
	"time"
)

func TestSlug(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		s, err := NewSlug()
		if err != nil {
			t.Fatal(err)
		}
		if !ValidSlug(s) {
			t.Fatalf("NewSlug produced invalid slug %q", s)
		}
		if seen[s] {
			t.Fatalf("duplicate slug %q in 1000 draws", s)
		}
		seen[s] = true
	}
	for _, bad := range []string{"", "short", strings.Repeat("a", 11), "aaaa-aaaaa", "aaaaaaaaa/", "ääääääääää"} {
		if ValidSlug(bad) {
			t.Errorf("ValidSlug(%q) = true", bad)
		}
	}
}

func TestSignalColourIsStable(t *testing.T) {
	valid := map[string]bool{"red": true, "blue": true, "green": true, "orange": true}
	used := map[string]bool{}
	for i := 0; i < 200; i++ {
		s, _ := NewSlug()
		c := SignalColour(s)
		if !valid[c] {
			t.Fatalf("unexpected colour %q", c)
		}
		if c != SignalColour(s) {
			t.Fatal("colour changed between calls")
		}
		used[c] = true
	}
	if len(used) != 4 {
		t.Errorf("only %d of 4 colours appeared in 200 slugs", len(used))
	}
}

func TestValidateTitle(t *testing.T) {
	if got, err := ValidateTitle("  The ending of  Dune\n"); err != nil || got != "The ending of Dune" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := ValidateTitle("   "); err == nil {
		t.Error("blank title accepted")
	}
	if _, err := ValidateTitle(strings.Repeat("é", 121)); err == nil {
		t.Error("121-character title accepted")
	}
	if _, err := ValidateTitle(strings.Repeat("é", 120)); err != nil {
		t.Errorf("120 multi-byte characters rejected: %v", err)
	}
}

func TestExpiresAt(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	for choice, want := range map[string]time.Duration{"1d": 24 * time.Hour, "1w": 7 * 24 * time.Hour, "1m": 30 * 24 * time.Hour} {
		got, err := ExpiresAt(choice, now)
		if err != nil || got == nil || !got.Equal(now.Add(want)) {
			t.Errorf("%s: got %v, %v", choice, got, err)
		}
	}
	if got, err := ExpiresAt("never", now); err != nil || got != nil {
		t.Errorf("never: got %v, %v", got, err)
	}
	if _, err := ExpiresAt("2y", now); err == nil {
		t.Error("unknown choice accepted")
	}
}

func TestParseMaxViews(t *testing.T) {
	if got, err := ParseMaxViews("  "); err != nil || got != nil {
		t.Errorf("blank: got %v, %v", got, err)
	}
	if got, err := ParseMaxViews("25"); err != nil || got == nil || *got != 25 {
		t.Errorf("25: got %v, %v", got, err)
	}
	for _, bad := range []string{"0", "-1", "1.5", "ten", "1000001"} {
		if _, err := ParseMaxViews(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestTimeLeft(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	at := func(d time.Duration) *time.Time { t := now.Add(d); return &t }
	cases := []struct {
		in   *time.Time
		want string
	}{
		{nil, ""},
		{at(30 * time.Second), "gone in under a minute"},
		{at(90 * time.Second), "gone in 1 minute"},
		{at(5 * time.Hour), "gone in 5 hours"},
		{at(36 * time.Hour), "gone in 1 day"},
		{at(20 * 24 * time.Hour), "gone in 20 days"},
	}
	for _, c := range cases {
		if got := TimeLeft(c.in, now); got != c.want {
			t.Errorf("TimeLeft = %q, want %q", got, c.want)
		}
	}
}

func TestIsPreviewCrawler(t *testing.T) {
	bots := []string{
		"Twitterbot/1.0",
		"Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)",
		"Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)",
		"facebookexternalhit/1.1",
		"LinkedInBot/1.0",
		"TelegramBot (like TwitterBot)",
		"WhatsApp/2.23.20.0",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mastodon/4.2.0 (http.rb/5.1.1; +https://mastodon.social/)",
	}
	for _, ua := range bots {
		if !IsPreviewCrawler(ua) {
			t.Errorf("not detected: %s", ua)
		}
	}
	people := []string{
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Mobile Safari/537.36",
		"",
	}
	for _, ua := range people {
		if IsPreviewCrawler(ua) {
			t.Errorf("person treated as crawler: %q", ua)
		}
	}
}
