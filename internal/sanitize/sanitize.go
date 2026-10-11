// Package sanitize reduces author-supplied HTML to pastr's small formatting allowlist.
package sanitize

import (
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

var (
	policy = newPolicy()
	strict = bluemonday.StrictPolicy()
	// The editor's only heading is <h1>; the page title already owns that level.
	h1Open  = regexp.MustCompile(`(?i)<h1(\s[^>]*)?>`)
	h1Close = regexp.MustCompile(`(?i)</h1\s*>`)
)

func newPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("div", "p", "br", "strong", "b", "em", "i", "del", "s", "h2", "blockquote", "ul", "ol", "li")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.RequireParseableURLs(true)
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true) // also adds rel="noopener"
	return p
}

// HTML returns input with everything outside the allowlist removed.
// It is applied once, when a paste is saved.
func HTML(input string) string {
	s := h1Open.ReplaceAllString(input, "<h2>")
	s = h1Close.ReplaceAllString(s, "</h2>")
	s = policy.Sanitize(s)
	// Added after sanitising, so authors cannot supply tracking attributes of their own.
	return strings.ReplaceAll(s, `<a href=`, `<a data-umami-event="outbound-link" href=`)
}

// IsBlank reports whether sanitised HTML contains no visible text.
func IsBlank(html string) bool {
	text := strict.Sanitize(html)
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, " ", " ")
	return strings.TrimSpace(text) == ""
}
