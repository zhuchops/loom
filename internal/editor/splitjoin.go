package editor

import (
	"go/parser"
	"go/token"
	"strings"
	"unicode"
	"unicode/utf8"
)

func (e *editor) indentUnit(base string) string {
	switch e.cfg.IndentStyle {
	case "tabs":
		return "\t"
	case "spaces":
		return strings.Repeat(" ", e.cfg.TabSize)
	}
	if e.cfg.languageFor(e.b.path) == "go" || e.cfg.languageFor(e.b.path) == "makefile" || strings.ContainsRune(base, '\t') {
		return "\t"
	}
	return strings.Repeat(" ", e.cfg.TabSize)
}

type argumentGroup struct {
	open, close int
	commas      []int
	// Comments, templates, and other constructs that need a language parser
	// make the group ineligible for whitespace rewriting.
	unsafe bool
}

// Find the innermost matching group at the cursor. Commas belonging to a
// nested group or a quoted string are excluded from its separator list.
func groupAt(text []rune, cursor int, goFile bool) (argumentGroup, bool) {
	var stack []argumentGroup
	for i := 0; i < len(text); i++ {
		r := text[i]
		switch {
		case r == '\'' || r == '"' || r == '`':
			start := i
			for i++; i < len(text); i++ {
				if text[i] == '\\' && r != '`' {
					i++
				} else if text[i] == r {
					break
				}
			}
			if i >= len(text) {
				return argumentGroup{}, false
			}
			if !goFile && (r == '`' || i-start >= 2 && text[start+1] == r && text[start+2] == r) {
				for j := range stack {
					stack[j].unsafe = true
				}
			}
		case r == '#' || r == '/' && i+1 < len(text) && (text[i+1] == '/' || text[i+1] == '*'):
			for j := range stack {
				stack[j].unsafe = true
			}
			if r == '/' && text[i+1] == '*' {
				i += 2
				for i+1 < len(text) && (text[i] != '*' || text[i+1] != '/') {
					i++
				}
				i++
			} else {
				for i+1 < len(text) && text[i+1] != '\n' {
					i++
				}
			}
		case strings.ContainsRune("([{", r):
			stack = append(stack, argumentGroup{open: i})
		case strings.ContainsRune(")]}", r):
			if len(stack) == 0 {
				continue
			}
			group := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if pairCloser(text[group.open]) != r {
				stack = stack[:0]
				continue
			}
			if group.open <= cursor && cursor <= i {
				group.close = i
				return group, true
			}
		case r == ',' && len(stack) > 0:
			inner := &stack[len(stack)-1]
			inner.commas = append(inner.commas, i)
		case (r == ';' || r == '/' && !goFile) && len(stack) > 0:
			for j := range stack {
				stack[j].unsafe = true
			}
		}
	}
	return argumentGroup{}, false
}

func trimArgument(text []rune, bounds span) span {
	for bounds.start < bounds.end && unicode.IsSpace(text[bounds.start]) {
		bounds.start++
	}
	for bounds.end > bounds.start && unicode.IsSpace(text[bounds.end-1]) {
		bounds.end--
	}
	return bounds
}

func (e *editor) toggleSplitJoin() {
	b := e.b
	goFile := e.cfg.languageFor(b.path) == "go"
	group, ok := groupAt(b.text, b.cursor, goFile)
	if !ok || group.unsafe {
		e.notify("No supported bracketed list at the cursor")
		return
	}
	separators := append(group.commas, group.close)
	args := make([]span, 0, len(separators))
	start := group.open + 1
	for _, separator := range separators {
		arg := trimArgument(b.text, span{start, separator})
		if arg.start == arg.end {
			if separator != group.close {
				e.notify("Cannot split or join a list with an empty item")
				return
			}
		} else {
			if b.rowAt(arg.start) != b.rowAt(arg.end-1) {
				e.notify("Toggle nested multiline items first")
				return
			}
			args = append(args, arg)
		}
		start = separator + 1
	}
	// Avoid turning ordinary blocks, index expressions, or grouping into lists.
	if len(args) == 0 || len(group.commas) == 0 && b.text[group.open] != '(' {
		e.notify("No arguments to split or join")
		return
	}
	if goFile && (b.text[group.open] == '[' || b.text[group.open] == '(' && len(group.commas) == 0 && !goCallBefore(b.text, group.open)) {
		e.notify("Split or join Go arguments or composite literal elements")
		return
	}

	openRow := b.rowAt(group.open)
	base := string(b.text[b.lines[openRow]:group.open])
	base = base[:len(base)-len(strings.TrimLeft(base, " \t"))]
	newline := b.newline()
	itemIndent := base + e.indentUnit(base)
	joining := openRow != b.rowAt(group.close)
	trailing := len(group.commas) == len(args)
	edits := make([]edit, 0, len(args)+1)
	for i := 0; i <= len(args); i++ {
		start, end := group.open+1, group.close
		if i > 0 {
			start = args[i-1].end
		}
		if i < len(args) {
			end = args[i].start
		}
		value := ""
		switch {
		case i == len(args):
			if goFile && !joining || !goFile && trailing {
				value = ","
			}
			if !joining {
				value += newline + base
			}
		case joining:
			if i > 0 {
				value = ", "
			}
		default:
			if i > 0 {
				value = ","
			}
			value += newline + itemIndent
		}
		if value != string(b.text[start:end]) {
			edits = append(edits, edit{span{start, end}, value})
		}
	}
	if len(edits) == 0 {
		return
	}
	if goFile && !validGoSplitJoin(b, edits) {
		e.notify("This list cannot be split or joined without changing Go syntax")
		return
	}

	// Preserve the cursor inside an argument when whitespace before it moves.
	cursor, delta := b.cursor, 0
	for _, change := range edits {
		if cursor < change.start {
			break
		}
		width := utf8.RuneCountInString(change.text)
		if cursor < change.end {
			cursor = change.start + width
			break
		}
		delta += width - (change.end - change.start)
	}
	b.replaceAll(edits, cursor+delta)
	e.normal()
}

// For a syntactically valid Go buffer, reject a rewrite that would make it
// invalid, including cases involving semicolon insertion or type lists.
func validGoSplitJoin(b *buffer, edits []edit) bool {
	if _, err := parser.ParseFile(token.NewFileSet(), b.path, string(b.text), parser.SkipObjectResolution); err != nil {
		return true
	}
	var source strings.Builder
	last := 0
	for _, change := range edits {
		source.WriteString(string(b.text[last:change.start]))
		source.WriteString(change.text)
		last = change.end
	}
	source.WriteString(string(b.text[last:]))
	_, err := parser.ParseFile(token.NewFileSet(), b.path, source.String(), parser.SkipObjectResolution)
	return err == nil
}

// A single parenthesized Go expression cannot take a trailing comma.
func goCallBefore(text []rune, open int) bool {
	prefix := strings.TrimRight(string(text[:open]), " \t")
	last, _ := utf8.DecodeLastRuneInString(prefix)
	if last == ')' || last == ']' {
		return true
	}
	start := len(prefix)
	for start > 0 {
		r, width := utf8.DecodeLastRuneInString(prefix[:start])
		if wordClass(r) != 1 {
			break
		}
		start -= width
	}
	word := prefix[start:]
	return word != "" && !token.Lookup(word).IsKeyword()
}
