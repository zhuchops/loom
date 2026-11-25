package markdown

import (
	"strings"
	"testing"
)

func TestHTML(t *testing.T) {
	src := "# Title *x*\n\nSome **bold**, _em_, `co*de*`, ~~gone~~ and [a link](https://example.com \"t\").\nNext line.  \nAfter break.\n\n" +
		"- one\n- two\n  - nested\n- [x] done\n\n1. first\n2. second\n\n" +
		"> quoted\n> more\n\n> [!WARNING]\n> careful\n\n" +
		"```go\nfunc main() {}\n```\n\n| a | b |\n|:--|--:|\n| 1 | 2 |\n\n---\n\n" +
		"<script>alert(1)</script>\n\n<p align=\"center\"><img src=\"x.png\" alt=\"logo\" onerror=\"x()\"></p>\n\n![shot](a.png) https://go.dev, snake_case_name\n"
	out := HTML([]rune(src))
	for _, want := range []string{
		`<h1 id="title-x" data-l="0-0">Title <em>x</em></h1>`,
		`<strong>bold</strong>`, `<em>em</em>`, `<code>co*de*</code>`, `<del>gone</del>`,
		`<a data-href="https://example.com">a link</a>`, `<br>`,
		`<ul data-l=`, `<li data-l="7-8">two<ul`, `<li class="task" data-l="9-9"><span class="check checked"></span>done`,
		`<ol data-l=`, `<blockquote data-l="14-15"><p data-l="14-15">quoted`,
		`class="alert warning"`, `<div class="lang">go</div>`, `<span class="t-keyword">func</span>`,
		`<th>a</th>`, `<td style="text-align:right">2</td>`, `<hr`,
		`<span class="img">logo</span>`, `<span class="img">shot</span>`,
		`<a data-href="https://go.dev">https://go.dev</a>,`, `snake_case_name`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s\nin %s", want, out)
		}
	}
	for _, bad := range []string{"<script", "onerror", "<img", "src="} {
		if strings.Contains(out, bad) {
			t.Errorf("unsafe %q in %s", bad, out)
		}
	}
}

func TestHTMLNeverPanics(t *testing.T) {
	for _, s := range []string{"", "\n", "*", "**", "`", "``a", "[", "[a](", "![", "<", "<a", "|", "|a|\n|-|", "- ", "1.", "> ", "```", "~~~\n", "    code", "#", "a\n===", "\\", "_a_b_", "***x***"} {
		HTML([]rune(s))
	}
}
