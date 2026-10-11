package paste

import "strings"

// Substrings of user agents that fetch pages to build link previews or indexes.
// They are shown the title only and never count as a view.
var crawlerMarkers = []string{
	"twitterbot", "facebookexternalhit", "facebot", "slackbot", "discordbot", "linkedinbot",
	"telegrambot", "whatsapp", "mastodon", "pleroma", "bluesky", "skypeuripreview", "redditbot",
	"pinterest", "embedly", "iframely", "googlebot", "bingbot", "duckduckbot", "yandexbot", "applebot",
	"bot/", "crawler", "spider", "preview",
}

func IsPreviewCrawler(userAgent string) bool {
	ua := strings.ToLower(userAgent)
	if ua == "" {
		return false
	}
	for _, m := range crawlerMarkers {
		if strings.Contains(ua, m) {
			return true
		}
	}
	return false
}
