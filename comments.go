package main

import "strings"

// toggleComment applies one edit per nonblank line, as a single undo step.
func (e *editor) toggleComment() {
	b := e.b
	if b.tags != nil {
		e.notify("Comments are unavailable in the directory editor")
		return
	}
	var open, close string
	switch e.cfg.languageFor(b.path) {
	case "go", "typescript", "typescriptreact", "javascript", "javascriptreact", "rust", "c", "cpp":
		open = "//"
	case "python", "ruby", "shellscript", "yaml", "toml":
		open = "#"
	case "lua", "sql":
		open = "--"
	case "css":
		open, close = "/*", "*/"
	case "html", "markdown":
		open, close = "<!--", "-->"
	default:
		e.notify("No comment syntax for this file")
		return
	}
	first, last := b.rowAt(b.cursor), b.rowAt(b.cursor)
	if b.anchor >= 0 && e.mode == "select" {
		first, last = b.rowAt(min(b.anchor, b.cursor)), b.rowAt(max(b.anchor, b.cursor))
	} else {
		last = min(len(b.lines)-1, first+e.takeCount()-1)
	}
	type line struct {
		start, end int
		text       string
	}
	var lines []line
	allCommented := true
	for row := first; row <= last; row++ {
		start, end := b.lines[row], b.lineEnd(row)
		for start < end && (b.text[start] == ' ' || b.text[start] == '\t') {
			start++
		}
		if start == end {
			continue
		}
		text := string(b.text[start:end])
		trimmed := strings.TrimRight(text, " \t")
		if !strings.HasPrefix(text, open) || close != "" && !strings.HasSuffix(trimmed, close) {
			allCommented = false
		}
		lines = append(lines, line{start, end, text})
	}
	var edits []edit
	for _, l := range lines {
		if allCommented {
			n := len([]rune(open))
			if l.start+n < l.end && b.text[l.start+n] == ' ' {
				n++
			}
			edits = append(edits, edit{span{l.start, l.start + n}, ""})
			if close != "" {
				end := l.end
				for end > l.start && (b.text[end-1] == ' ' || b.text[end-1] == '\t') {
					end--
				}
				start := end - len([]rune(close))
				if start > l.start+n && b.text[start-1] == ' ' {
					start--
				}
				edits = append(edits, edit{span{start, end}, ""})
			}
		} else {
			edits = append(edits, edit{span{l.start, l.start}, open + " "})
			if close != "" {
				edits = append(edits, edit{span{l.end, l.end}, " " + close})
			}
		}
	}
	if len(edits) == 0 {
		return
	}
	shift := func(at int) int {
		if at < 0 {
			return at
		}
		result := at
		for _, edit := range edits {
			if at < edit.start {
				break
			}
			if at < edit.end {
				return edit.start + result - at
			}
			result += len([]rune(edit.text)) - (edit.end - edit.start)
		}
		return result
	}
	cursor, anchor := shift(b.cursor), shift(b.anchor)
	b.replaceAll(edits, cursor)
	b.anchor = anchor
}
