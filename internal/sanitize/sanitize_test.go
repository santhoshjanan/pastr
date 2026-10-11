package sanitize

import (
	"strings"
	"testing"
)

func TestKeepsFormatting(t *testing.T) {
	in := `<div><strong>Bold</strong> <em>it</em> <del>gone</del><br></div><blockquote>q</blockquote><ul><li>a</li></ul><ol><li>b</li></ol>`
	if got := HTML(in); got != in {
		t.Errorf("formatting changed:\n got %s\nwant %s", got, in)
	}
}

func TestHeadingBecomesH2(t *testing.T) {
	if got := HTML(`<h1 class="x">Act Three</h1>`); got != `<h2>Act Three</h2>` {
		t.Errorf("got %s", got)
	}
}

func TestLinks(t *testing.T) {
	got := HTML(`<a href="https://example.com/a" onclick="x()" style="color:red">go</a>`)
	for _, want := range []string{`href="https://example.com/a"`, "nofollow", "noreferrer", "noopener", `data-umami-event="outbound-link"`} {
		if !strings.Contains(got, want) {
			t.Errorf("link missing %s: %s", want, got)
		}
	}
	for _, bad := range []string{"onclick", "style"} {
		if strings.Contains(got, bad) {
			t.Errorf("link kept %s: %s", bad, got)
		}
	}
}

func TestStripsDangerousContent(t *testing.T) {
	payloads := []string{
		`<script>alert(1)</script>`,
		`<img src=x onerror=alert(1)>`,
		`<svg/onload=alert(1)>`,
		`<iframe src="https://evil.example"></iframe>`,
		`<a href="javascript:alert(1)">x</a>`,
		`<a href="JaVaScRiPt:alert(1)">x</a>`,
		`<a href="data:text/html,<script>alert(1)</script>">x</a>`,
		`<a href="vbscript:msgbox(1)">x</a>`,
		`<div style="background:url(javascript:alert(1))">x</div>`,
		`<style>body{display:none}</style>`,
		`<object data="x"></object><embed src="x">`,
		`<form action="https://evil.example"><input name="p"><button>go</button></form>`,
		`<meta http-equiv="refresh" content="0;url=https://evil.example">`,
		`<base href="https://evil.example/">`,
		`<link rel="stylesheet" href="https://evil.example/x.css">`,
		`<math><mtext><script>alert(1)</script></mtext></math>`,
		`<div onmouseover="alert(1)">x</div>`,
		`<<script>script>alert(1)<</script>/script>`,
		`<h1 onclick="alert(1)">x</h1>`,
		`<details open ontoggle=alert(1)>x</details>`,
		`<video><source onerror=alert(1)></video>`,
		`<div data-umami-event="fake">x</div>`,
	}
	banned := []string{"<script", "<img", "<svg", "<iframe", "javascript:", "vbscript:", "data:text", "style=", "<style",
		"<object", "<embed", "<form", "<input", "<button", "<meta", "<base", "<link", "<math", "onmouseover", "onclick",
		"ontoggle", "onerror", "onload", "<details", "<video", `data-umami-event="fake"`}
	for _, p := range payloads {
		got := strings.ToLower(HTML(p))
		for _, b := range banned {
			if strings.Contains(got, b) {
				t.Errorf("payload %q left %q in output: %s", p, b, got)
			}
		}
	}
}

func TestIsBlank(t *testing.T) {
	for _, blank := range []string{"", "   ", "<div><br></div>", "<div>&nbsp;</div>", "<p> \n </p>"} {
		if !IsBlank(HTML(blank)) {
			t.Errorf("%q should be blank", blank)
		}
	}
	if IsBlank(HTML("<div>He was dead all along.</div>")) {
		t.Error("real text reported blank")
	}
}
