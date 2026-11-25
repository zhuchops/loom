package markdown

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode"

	"github.com/zhuchops/loom/internal/syntax"
)

// HTML renders Markdown as an HTML fragment for the desktop app's preview.
// Every block carries data-l="start-end", the source rows it came from, so
// the preview can follow the editor's cursor. Raw HTML in the source is shown
// as text, except a few harmless tags without attributes, and images show
// their description instead of loading: a Markdown file must never run code
// in the editor's page or reach the network when it is opened.
func HTML(text []rune) string {
	src := strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n")
	var b strings.Builder
	htmlBlocks(&b, src, 0)
	return b.String()
}

var (
	mdOrdered  = regexp.MustCompile(`^(\s*)(\d{1,9})([.)])(\s+|$)(.*)$`)
	mdBullet   = regexp.MustCompile(`^(\s*)([-*+])(\s+|$)(.*)$`)
	mdTask     = regexp.MustCompile(`^\[([ xX])\]\s+(.*)$`)
	mdAlert    = regexp.MustCompile(`^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*$`)
	mdHTMLLine = regexp.MustCompile(`^\s*</?[A-Za-z][A-Za-z0-9-]*(\s[^>]*)?/?>`)
)

func indentWidth(s string) int {
	n := 0
	for _, r := range s {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 4 - n%4
		default:
			return n
		}
	}
	return n
}

// dedent removes up to n columns of leading whitespace.
func dedent(s string, n int) string {
	col := 0
	for i, r := range s {
		if col >= n {
			return s[i:]
		}
		switch r {
		case ' ':
			col++
		case '\t':
			col += 4 - col%4
		default:
			return s[i:]
		}
	}
	return ""
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }

func attrLines(from, to int) string { return fmt.Sprintf(` data-l="%d-%d"`, from, to) }

// startsBlock tells whether a line ends a paragraph.
func startsBlock(line string) bool {
	return mdHeading.MatchString(line) || mdFence.MatchString(line) || mdRule.MatchString(line) ||
		mdQuote.MatchString(line) || mdBullet.MatchString(line) && !blank(mdBullet.FindStringSubmatch(line)[4]) ||
		mdOrdered.MatchString(line) && mdOrdered.FindStringSubmatch(line)[2] == "1"
}

func htmlBlocks(b *strings.Builder, src []string, base int) {
	for i := 0; i < len(src); {
		line := src[i]
		row := base + i
		switch {
		case blank(line):
			i++

		case mdFence.MatchString(line):
			m := mdFence.FindStringSubmatch(line)
			fence, lang := m[1], strings.ToLower(m[2])
			indent := indentWidth(line)
			var code []string
			j := i + 1
			for j < len(src) && !strings.HasPrefix(strings.TrimSpace(src[j]), fence[:3]) {
				code = append(code, dedent(src[j], indent))
				j++
			}
			end := min(j, len(src)-1)
			htmlCode(b, code, lang, row, base+end)
			i = j + 1

		case indentWidth(line) >= 4:
			// An indented code block.
			var code []string
			j := i
			for j < len(src) && (indentWidth(src[j]) >= 4 || blank(src[j])) {
				code = append(code, dedent(src[j], 4))
				j++
			}
			for len(code) > 0 && blank(code[len(code)-1]) {
				code = code[:len(code)-1]
				j--
			}
			htmlCode(b, code, "", row, base+j-1)
			i = j

		case mdHeading.MatchString(line):
			m := mdHeading.FindStringSubmatch(line)
			n := len(m[1])
			fmt.Fprintf(b, `<h%d id="%s"%s>%s</h%d>`, n, slug(m[2]), attrLines(row, row), inline(m[2]), n)
			i++

		case mdRule.MatchString(line):
			fmt.Fprintf(b, `<hr%s>`, attrLines(row, row))
			i++

		case mdQuote.MatchString(line):
			var inner []string
			j := i
			for j < len(src) && !blank(src[j]) {
				if m := mdQuote.FindStringSubmatch(src[j]); m != nil {
					inner = append(inner, m[1])
				} else if startsBlock(src[j]) {
					break
				} else {
					inner = append(inner, src[j]) // a lazy continuation
				}
				j++
			}
			class := ""
			if len(inner) > 0 {
				if m := mdAlert.FindStringSubmatch(strings.TrimSpace(inner[0])); m != nil {
					kind := strings.ToLower(m[1])
					class = ` class="alert ` + kind + `"`
					inner[0] = ""
					fmt.Fprintf(b, `<blockquote%s%s><p class="alert-title">%s</p>`, class, attrLines(row, base+j-1), strings.ToUpper(kind[:1])+kind[1:])
					htmlBlocks(b, inner, row)
					b.WriteString(`</blockquote>`)
					i = j
					break
				}
			}
			fmt.Fprintf(b, `<blockquote%s>`, attrLines(row, base+j-1))
			htmlBlocks(b, inner, row)
			b.WriteString(`</blockquote>`)
			i = j

		case mdBullet.MatchString(line) || mdOrdered.MatchString(line):
			i = htmlList(b, src, i, base)

		case i+1 < len(src) && strings.Contains(line, "|") && mdTableSep.MatchString(src[i+1]):
			i = htmlTable(b, src, i, base)

		case mdHTMLLine.MatchString(line):
			// An HTML block, up to a blank line.
			j := i
			var raw []string
			for j < len(src) && !blank(src[j]) {
				raw = append(raw, src[j])
				j++
			}
			fmt.Fprintf(b, `<div class="raw"%s>%s</div>`, attrLines(row, base+j-1), safeHTML(strings.Join(raw, "\n")))
			i = j

		default:
			// A paragraph, or a setext heading.
			j := i + 1
			for j < len(src) && !blank(src[j]) && !startsBlock(src[j]) && !(strings.Contains(src[j], "|") && j+1 < len(src) && mdTableSep.MatchString(src[j+1])) {
				if mdSetextOne.MatchString(src[j]) || mdSetextTwo.MatchString(src[j]) {
					break
				}
				j++
			}
			text := strings.Join(trimEach(src[i:j]), "\n")
			if j < len(src) && (mdSetextOne.MatchString(src[j]) || mdSetextTwo.MatchString(src[j]) && !blank(text)) {
				n := 1
				if mdSetextTwo.MatchString(src[j]) {
					n = 2
				}
				fmt.Fprintf(b, `<h%d id="%s"%s>%s</h%d>`, n, slug(text), attrLines(row, base+j), inline(text), n)
				i = j + 1
				break
			}
			fmt.Fprintf(b, `<p%s>%s</p>`, attrLines(row, base+j-1), inline(text))
			i = j
		}
	}
}

func trimEach(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		// Two trailing spaces are a hard break.
		if strings.HasSuffix(l, "  ") {
			out[i] = strings.TrimSpace(l) + "\\"
		} else {
			out[i] = strings.TrimSpace(l)
		}
	}
	return out
}

// htmlList renders the list starting at src[i] and returns where it ends.
func htmlList(b *strings.Builder, src []string, i, base int) int {
	ordered := !mdBullet.MatchString(src[i])
	marker := func(line string) (indent, content int, text string, ok bool) {
		if ordered {
			m := mdOrdered.FindStringSubmatch(line)
			if m == nil {
				return 0, 0, "", false
			}
			w := indentWidth(m[1])
			return w, w + len(m[2]) + len(m[3]) + max(1, min(len(m[4]), 4)), m[5], true
		}
		m := mdBullet.FindStringSubmatch(line)
		if m == nil || mdRule.MatchString(line) {
			return 0, 0, "", false
		}
		w := indentWidth(m[1])
		return w, w + 1 + max(1, min(len(m[3]), 4)), m[4], true
	}
	top, _, _, _ := marker(src[i])
	start := ""
	if ordered {
		if n := mdOrdered.FindStringSubmatch(src[i])[2]; n != "1" {
			start = ` start="` + strings.TrimLeft(n, "0") + `"`
		}
	}
	tag := "ul"
	if ordered {
		tag = "ol"
	}
	// Collect the items first, to know whether the list is loose.
	type item struct {
		lines    []string
		from, to int
	}
	var items []item
	loose := false
	j := i
	for j < len(src) {
		indent, content, text, ok := marker(src[j])
		if !ok || indent > top+3 || indent < top {
			break
		}
		it := item{lines: []string{text}, from: base + j}
		k := j + 1
		for k < len(src) {
			l := src[k]
			if blank(l) {
				// A blank line ends the item unless more of it follows.
				n := k + 1
				for n < len(src) && blank(src[n]) {
					n++
				}
				if n < len(src) && indentWidth(src[n]) >= content {
					it.lines = append(it.lines, "")
					loose = true
					k++
					continue
				}
				break
			}
			if _, _, _, sib := marker(l); sib && indentWidth(l) < content {
				break
			}
			if indentWidth(l) >= content {
				it.lines = append(it.lines, dedent(l, content))
			} else if startsBlock(l) {
				break
			} else {
				it.lines = append(it.lines, strings.TrimSpace(l)) // lazy continuation
			}
			k++
		}
		it.to = base + k - 1
		items = append(items, it)
		j = k
		// A blank line between items makes the list loose.
		if j < len(src) && blank(src[j]) {
			n := j
			for n < len(src) && blank(src[n]) {
				n++
			}
			if n < len(src) {
				if indent, _, _, ok := marker(src[n]); ok && indent >= top && indent <= top+3 {
					loose = true
					j = n
					continue
				}
			}
			break
		}
	}
	fmt.Fprintf(b, `<%s%s%s>`, tag, start, attrLines(base+i, base+j-1))
	for _, it := range items {
		class := ""
		if mdTask.MatchString(it.lines[0]) {
			class = ` class="task"`
		}
		fmt.Fprintf(b, `<li%s%s>`, class, attrLines(it.from, it.to))
		lines := it.lines
		if m := mdTask.FindStringSubmatch(lines[0]); m != nil {
			check := ""
			if m[1] != " " {
				check = " checked"
			}
			fmt.Fprintf(b, `<span class="check%s"></span>`, check)
			lines = append([]string{m[2]}, lines[1:]...)
		}
		if !loose && len(lines) > 0 {
			// A tight item's first paragraph goes in without <p>.
			k := 0
			for k < len(lines) && !blank(lines[k]) && (k == 0 || !startsBlock(lines[k])) {
				k++
			}
			b.WriteString(inline(strings.Join(trimEach(lines[:k]), "\n")))
			htmlBlocks(b, lines[k:], it.from+k)
		} else {
			htmlBlocks(b, lines, it.from)
		}
		b.WriteString(`</li>`)
	}
	fmt.Fprintf(b, `</%s>`, tag)
	return j
}

// htmlTable renders a GitHub table starting at src[i].
func htmlTable(b *strings.Builder, src []string, i, base int) int {
	head := splitRow(src[i])
	var aligns []string
	for _, c := range splitRow(src[i+1]) {
		c = strings.TrimSpace(c)
		switch {
		case strings.HasPrefix(c, ":") && strings.HasSuffix(c, ":"):
			aligns = append(aligns, "center")
		case strings.HasSuffix(c, ":"):
			aligns = append(aligns, "right")
		default:
			aligns = append(aligns, "")
		}
	}
	j := i + 2
	for j < len(src) && strings.Contains(src[j], "|") && !blank(src[j]) {
		j++
	}
	cell := func(tag string, k int, text string) {
		style := ""
		if k < len(aligns) && aligns[k] != "" {
			style = ` style="text-align:` + aligns[k] + `"`
		}
		fmt.Fprintf(b, `<%s%s>%s</%s>`, tag, style, inline(strings.TrimSpace(text)), tag)
	}
	fmt.Fprintf(b, `<div class="table"%s><table><thead><tr>`, attrLines(base+i, base+j-1))
	for k, c := range head {
		cell("th", k, c)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for r := i + 2; r < j; r++ {
		fmt.Fprintf(b, `<tr%s>`, attrLines(base+r, base+r))
		row := splitRow(src[r])
		for k := range head {
			text := ""
			if k < len(row) {
				text = row[k]
			}
			cell("td", k, text)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table></div>`)
	return j
}

// htmlCode renders a code block, highlighted when the language is known.
func htmlCode(b *strings.Builder, code []string, lang string, from, to int) {
	fmt.Fprintf(b, `<div class="code"%s>`, attrLines(from, to))
	if lang != "" {
		fmt.Fprintf(b, `<div class="lang">%s</div>`, html.EscapeString(lang))
	}
	b.WriteString(`<pre><code>`)
	text := []rune(strings.Join(code, "\n"))
	tokens := syntax.Highlight(syntaxName(lang), "", text)
	at := 0
	for _, t := range tokens {
		if t.Start < at || t.End > len(text) || t.Start >= t.End {
			continue
		}
		b.WriteString(html.EscapeString(string(text[at:t.Start])))
		fmt.Fprintf(b, `<span class="t-%s">%s</span>`, kindClass(t.Kind), html.EscapeString(string(text[t.Start:t.End])))
		at = t.End
	}
	b.WriteString(html.EscapeString(string(text[at:])))
	b.WriteString(`</code></pre></div>`)
}

// syntaxName maps a fence's language to the highlighter's name for it.
func syntaxName(lang string) string {
	switch lang {
	case "golang":
		return "go"
	case "js", "javascript", "jsx", "mjs":
		return "javascript"
	case "ts", "typescript", "tsx":
		return "typescript"
	case "py", "python3":
		return "python"
	case "sh", "bash", "zsh", "shell", "console":
		return "sh"
	case "rs":
		return "rust"
	case "yml":
		return "yaml"
	}
	return lang
}

func kindClass(kind string) string {
	var b strings.Builder
	for _, r := range kind {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(stripInline(s))) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}

// stripInline drops Markdown punctuation, for heading anchors.
func stripInline(s string) string {
	return strings.NewReplacer("`", "", "*", "", "_", "", "[", "", "]", "", "~", "").Replace(s)
}

// safeTags are the HTML tags kept from the source, without their attributes.
var safeTags = map[string]bool{
	"b": true, "strong": true, "i": true, "em": true, "u": true, "s": true, "del": true, "ins": true, "mark": true,
	"code": true, "kbd": true, "sub": true, "sup": true, "br": true, "hr": true, "p": true, "div": true, "span": true,
	"details": true, "summary": true, "small": true, "blockquote": true, "ul": true, "ol": true, "li": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "pre": true, "center": true,
	"table": true, "thead": true, "tbody": true, "tr": true, "th": true, "td": true, "picture": true,
}

var htmlTag = regexp.MustCompile(`<(/?)([A-Za-z][A-Za-z0-9-]*)((?:\s[^<>]*?)?)(/?)>`)
var htmlAlt = regexp.MustCompile(`\balt\s*=\s*("([^"]*)"|'([^']*)')`)
var htmlHref = regexp.MustCompile(`\bhref\s*=\s*("([^"]*)"|'([^']*)')`)

// safeHTML keeps harmless tags, turns images into their description and
// links into Loom's links, and escapes everything else.
func safeHTML(s string) string {
	var b strings.Builder
	at := 0
	for _, m := range htmlTag.FindAllStringSubmatchIndex(s, -1) {
		b.WriteString(html.EscapeString(html.UnescapeString(s[at:m[0]])))
		at = m[1]
		closing, name, attrs := s[m[2]:m[3]] == "/", strings.ToLower(s[m[4]:m[5]]), s[m[6]:m[7]]
		switch {
		case name == "img":
			alt := ""
			if a := htmlAlt.FindStringSubmatch(attrs); a != nil {
				alt = a[2] + a[3]
			}
			b.WriteString(imageChip(alt))
		case name == "a" && closing:
			b.WriteString(`</a>`)
		case name == "a":
			href := ""
			if h := htmlHref.FindStringSubmatch(attrs); h != nil {
				href = h[2] + h[3]
			}
			fmt.Fprintf(&b, `<a data-href="%s">`, html.EscapeString(href))
		case safeTags[name]:
			if closing {
				fmt.Fprintf(&b, `</%s>`, name)
			} else {
				fmt.Fprintf(&b, `<%s>`, name)
			}
		case name == "script" || name == "style":
			// Neither runs: the tags go, and what was between them shows as text.
		default:
			// Anything else is left out, keeping what is between its tags.
		}
	}
	b.WriteString(html.EscapeString(html.UnescapeString(s[at:])))
	return b.String()
}

func imageChip(alt string) string {
	if alt == "" {
		alt = "image"
	}
	return `<span class="img">` + html.EscapeString(alt) + `</span>`
}

// inline renders emphasis, code, links, images and line breaks.
func inline(s string) string {
	var b strings.Builder
	r := []rune(s)
	text := func(t string) { b.WriteString(html.EscapeString(t)) }
	for i := 0; i < len(r); {
		c := r[i]
		switch {
		case c == '\\' && i+1 < len(r) && strings.ContainsRune("\\`*_{}[]()#+-.!|<>~\"'", r[i+1]):
			text(string(r[i+1]))
			i += 2
			continue
		case c == '\\' && i+1 < len(r) && r[i+1] == '\n', c == '\\' && i+1 == len(r):
			b.WriteString("<br>")
			i++
			if i < len(r) && r[i] == '\n' {
				i++
			}
			continue
		case c == '`':
			n := 0
			for i+n < len(r) && r[i+n] == '`' {
				n++
			}
			if end := ticksAt(r, i+n, n); end >= 0 {
				fmt.Fprintf(&b, `<code>%s</code>`, html.EscapeString(strings.TrimSpace(string(r[i+n:end]))))
				i = end + n
				continue
			}
			text(strings.Repeat("`", n))
			i += n
			continue
		case c == '!' && i+1 < len(r) && r[i+1] == '[':
			if label, _, next, ok := linkAt(r, i+1); ok {
				b.WriteString(imageChip(label))
				i = next
				continue
			}
		case c == '[':
			if label, href, next, ok := linkAt(r, i); ok {
				fmt.Fprintf(&b, `<a data-href="%s">%s</a>`, html.EscapeString(href), inline(label))
				i = next
				continue
			}
		case c == '<':
			if end := strings.IndexRune(string(r[i:]), '>'); end > 0 {
				inner := string(r[i+1 : i+end])
				if strings.HasPrefix(inner, "http://") || strings.HasPrefix(inner, "https://") || strings.HasPrefix(inner, "mailto:") {
					fmt.Fprintf(&b, `<a data-href="%s">%s</a>`, html.EscapeString(inner), html.EscapeString(inner))
					i += end + 1
					continue
				}
				if htmlTag.MatchString(string(r[i : i+end+1])) {
					b.WriteString(safeHTML(string(r[i : i+end+1])))
					i += end + 1
					continue
				}
			}
		case (c == 'h' || c == 'w') && (i == 0 || !unicode.IsLetter(r[i-1])):
			rest := string(r[i:])
			if strings.HasPrefix(rest, "https://") || strings.HasPrefix(rest, "http://") || strings.HasPrefix(rest, "www.") {
				n := 0
				for i+n < len(r) && !unicode.IsSpace(r[i+n]) && r[i+n] != '<' {
					n++
				}
				for n > 0 && strings.ContainsRune(".,;:!?)'\"", r[i+n-1]) {
					n--
				}
				url := string(r[i : i+n])
				href := url
				if strings.HasPrefix(url, "www.") {
					href = "https://" + url
				}
				fmt.Fprintf(&b, `<a data-href="%s">%s</a>`, html.EscapeString(href), html.EscapeString(url))
				i += n
				continue
			}
		case c == '*' || c == '_' || c == '~':
			n := 0
			for i+n < len(r) && r[i+n] == c {
				n++
			}
			n = min(n, 3)
			if c == '~' && n < 2 {
				break
			}
			// Intraword underscores stay as they are.
			if c == '_' && i > 0 && (unicode.IsLetter(r[i-1]) || unicode.IsDigit(r[i-1])) {
				break
			}
			if i+n < len(r) && !unicode.IsSpace(r[i+n]) {
				delim := strings.Repeat(string(c), n)
				if end := closeAt(r, i+n, delim); end > 0 {
					inner := inline(string(r[i+n : end]))
					switch {
					case c == '~':
						fmt.Fprintf(&b, `<del>%s</del>`, inner)
					case n == 1:
						fmt.Fprintf(&b, `<em>%s</em>`, inner)
					case n == 2:
						fmt.Fprintf(&b, `<strong>%s</strong>`, inner)
					default:
						fmt.Fprintf(&b, `<strong><em>%s</em></strong>`, inner)
					}
					i = end + n
					continue
				}
			}
		case c == '\n':
			b.WriteString("\n")
			i++
			continue
		}
		text(string(c))
		i++
	}
	return b.String()
}

// ticksAt finds a run of exactly n backticks from r[from], or -1.
func ticksAt(r []rune, from, n int) int {
	for k := from; k+n <= len(r); k++ {
		if r[k] != '`' {
			continue
		}
		run := 0
		for k+run < len(r) && r[k+run] == '`' {
			run++
		}
		if run == n {
			return k
		}
		k += run - 1
	}
	return -1
}

// closeAt finds where delim closes emphasis that opened before from.
func closeAt(r []rune, from int, delim string) int {
	d := []rune(delim)
	for k := from + 1; k+len(d) <= len(r); k++ {
		if r[k] == '`' {
			// Skip code spans.
			if end := ticksAt(r, k+1, 1); end >= 0 {
				k = end
				continue
			}
		}
		if string(r[k:k+len(d)]) == delim && !unicode.IsSpace(r[k-1]) && (k+len(d) == len(r) || r[k+len(d)] != d[0]) {
			if d[0] == '_' && k+len(d) < len(r) && (unicode.IsLetter(r[k+len(d)]) || unicode.IsDigit(r[k+len(d)])) {
				continue
			}
			return k
		}
	}
	return -1
}

// linkAt reads [label](href "title") at r[i], which is '['.
func linkAt(r []rune, i int) (label, href string, next int, ok bool) {
	depth := 0
	k := i
	for ; k < len(r); k++ {
		if r[k] == '\\' {
			k++
			continue
		}
		if r[k] == '[' {
			depth++
		} else if r[k] == ']' {
			depth--
			if depth == 0 {
				break
			}
		}
	}
	if k >= len(r)-1 || r[k+1] != '(' {
		return "", "", 0, false
	}
	label = string(r[i+1 : k])
	depth = 0
	end := k + 1
	for ; end < len(r); end++ {
		if r[end] == '(' {
			depth++
		} else if r[end] == ')' {
			depth--
			if depth == 0 {
				break
			}
		}
	}
	if end >= len(r) {
		return "", "", 0, false
	}
	target := strings.TrimSpace(string(r[k+2 : end]))
	if sp := strings.IndexAny(target, " \t"); sp > 0 {
		target = target[:sp] // drop the title
	}
	target = strings.Trim(target, "<>")
	return label, target, end + 1, true
}
