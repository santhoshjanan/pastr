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
	"twitterbot", "slackbot", "discordbot", "telegrambot", "linkedinbot", "redditbot", "facebot",
	"googlebot", "bingbot", "duckduckbot", "applebot", "pinterestbot", "pinterest/0.",
	"facebookexternalhit", "whatsapp", "mastodon", "pleroma", "bluesky", "skypeuripreview", "viber",
	"embedly", "iframely", "yandex.com/bots", "inspectiontool", "headlesschrome",
	"crawler", "spider", "preview", "scanner", "fetcher", "unfurl",
	"curl/", "wget/", "go-http-client", "python-requests", "python-urllib", "aiohttp", "httpx",
	"okhttp", "java/", "node-fetch", "axios/", "libwww", "guzzlehttp",
}

// "bot" closing a product token: "SomeBot/1.0", "(bot; ...)", "Somebot-Fetch".
// A space is not enough, or phones such as "CUBOT NOTE 30" would be turned away.
var botWord = regexp.MustCompile(`bot([/;)-]|$)`)

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
