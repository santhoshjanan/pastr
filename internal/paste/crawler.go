package paste

import (
	"regexp"
	"strings"
)

// Substrings of user agents that fetch pages automatically: link-preview builders,
// search indexers, chat unfurlers, mail scanners and plain HTTP libraries. They are
// shown the title only and never count as a view, so they cannot hold a spoiler or
// burn a view-limited paste before a person opens it.
var crawlerMarkers = []string{
	"facebookexternalhit", "whatsapp", "mastodon", "pleroma", "bluesky", "skypeuripreview", "viber",
	"pinterest", "embedly", "iframely", "yandex.com/bots", "inspectiontool", "headlesschrome",
	"crawler", "spider", "preview", "scanner", "fetcher", "unfurl",
	"curl/", "wget/", "go-http-client", "python-requests", "python-urllib", "aiohttp", "httpx",
	"okhttp", "java/", "node-fetch", "axios/", "libwww", "guzzlehttp",
}

// "bot" as a word ending: Twitterbot/1.0, "(bot; ...)", "Slackbot-LinkExpanding".
// A phone model such as CUBOT_NOTE does not match.
var botWord = regexp.MustCompile(`bot([/;),\s-]|$)`)

// IsPreviewCrawler reports whether a request's user agent belongs to an automated
// fetcher. A missing user agent counts: every browser sends one.
func IsPreviewCrawler(userAgent string) bool {
	ua := strings.ToLower(userAgent)
	if ua == "" || botWord.MatchString(ua) {
		return true
	}
	for _, m := range crawlerMarkers {
		if strings.Contains(ua, m) {
			return true
		}
	}
	return false
}
