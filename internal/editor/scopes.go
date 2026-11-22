package editor

import (
	"regexp"
	"strings"
	"unicode"
)

// scopeRows lists the rows that enclose row, outermost first, judged by
// indentation: each is the nearest earlier line indented less than the last.
func scopeRows(b *buffer, row int) []int {
	indent := func(r int) (int, bool) {
		text := b.text[b.lines[r]:b.lineEnd(r)]
		width := 0
		for _, c := range text {
			switch c {
			case ' ':
				width++
			case '\t':
				width += tabWidth
			default:
				return width, true
			}
		}
		return 0, false // blank
	}
	current := -1
	for r := row; r >= 0 && current < 0; r-- {
		if w, ok := indent(r); ok {
			current = w
			if r != row {
				row = r
			}
		}
	}
	if current <= 0 {
		return nil
	}
	var out []int
	for r := row - 1; r >= 0 && current > 0; r-- {
		w, ok := indent(r)
		if ok && w < current {
			text := strings.TrimSpace(string(b.text[b.lines[r]:b.lineEnd(r)]))
			if text == "}" || text == ")" || text == "]" || text == "end" || strings.HasPrefix(text, "//") || strings.HasPrefix(text, "#") {
				continue
			}
			out = append([]int{r}, out...)
			current = w
		}
	}
	return out
}

var scopeName = regexp.MustCompile(`(?:func|def|class|fn|function|struct|interface|type|impl|trait|enum|module|namespace|mod|object|record|protocol|extension|describe|it|test)\s+(?:\([^)]*\)\s*)?([A-Za-z_$][\w$.<>]*)`)

// scopeLabel names a scope line: the declared name when there is one.
func scopeLabel(line string) string {
	line = strings.TrimSpace(line)
	if m := scopeName.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	line = strings.TrimRightFunc(line, func(r rune) bool { return r == '{' || r == ':' || unicode.IsSpace(r) })
	if r := []rune(line); len(r) > 28 {
		line = string(r[:28]) + "…"
	}
	return line
}

// breadcrumbs is the chain of enclosing scopes at the cursor.
func (e *editor) breadcrumbs(b *buffer) string {
	if b.dashboard || b.tags != nil || len(b.lines) > 200000 {
		return ""
	}
	rows := scopeRows(b, b.rowAt(b.cursor))
	if len(rows) > 4 {
		rows = rows[len(rows)-4:]
	}
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		if name := scopeLabel(string(b.text[b.lines[r]:b.lineEnd(r)])); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, " › ")
}
