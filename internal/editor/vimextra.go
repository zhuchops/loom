package editor

import (
	"strings"
	"unicode"
)

// Everyday Vim motions and scrolling commands.
func (e *editor) registerVimExtras() {
	e.add("search-word-next", "Search Word Under Cursor", func(e *editor) { e.searchWord(false) })
	e.add("search-word-prev", "Search Word Under Cursor Backward", func(e *editor) { e.searchWord(true) })
	e.add("paragraph-next", "Next Paragraph", func(e *editor) { e.paragraph(1) })
	e.add("paragraph-prev", "Previous Paragraph", func(e *editor) { e.paragraph(-1) })
	e.add("screen-top", "Top of Screen", func(e *editor) { e.screenRow(0) })
	e.add("screen-middle", "Middle of Screen", func(e *editor) { e.screenRow(1) })
	e.add("screen-bottom", "Bottom of Screen", func(e *editor) { e.screenRow(2) })
	e.add("half-page-down", "Half Page Down", func(e *editor) { e.halfPage(1) })
	e.add("half-page-up", "Half Page Up", func(e *editor) { e.halfPage(-1) })
	e.add("scroll-center", "Center Cursor Line", func(e *editor) { e.scrollTo(0.5) })
	e.add("scroll-top", "Cursor Line to Top", func(e *editor) { e.scrollTo(0) })
	e.add("scroll-bottom", "Cursor Line to Bottom", func(e *editor) { e.scrollTo(1) })
	e.add("reselect", "Reselect Last Selection", func(e *editor) {
		b := e.b
		lo, hi := b.marks['<'], b.marks['>']
		if _, ok := b.marks['<']; !ok {
			return
		}
		e.mode, b.anchor, b.cursor = "select", min(lo, len(b.text)), min(hi, len(b.text))
	})
	e.add("toggle-case", "Toggle Case", func(e *editor) {
		b := e.b
		n := min(e.takeCount(), b.lineEnd(b.rowAt(b.cursor))-b.cursor)
		if n <= 0 {
			return
		}
		start := b.cursor
		b.replace(span{start, start + n}, swapCase(string(b.text[start:start+n])))
		b.cursor = start + n
		e.clampModalCursor()
	})
	e.add("toggle-case-selection", "Toggle Case of Selection", func(e *editor) { e.transformSelection(swapCase) })
	e.add("upper-selection", "Uppercase Selection", func(e *editor) { e.transformSelection(strings.ToUpper) })
	e.add("lower-selection", "Lowercase Selection", func(e *editor) { e.transformSelection(strings.ToLower) })
}

func swapCase(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case unicode.IsUpper(r):
			return unicode.ToLower(r)
		case unicode.IsLower(r):
			return unicode.ToUpper(r)
		}
		return r
	}, s)
}

func (e *editor) transformSelection(fn func(string) string) {
	b := e.b
	s := b.selection()
	b.replace(s, fn(string(b.text[s.start:s.end])))
	e.normal()
	b.cursor = s.start
	e.clampModalCursor()
}

// searchWord is * and #: search for the whole word under the cursor.
func (e *editor) searchWord(reverse bool) {
	b := e.b
	at := b.cursor
	for at < b.lineEnd(b.rowAt(at)) && wordClass(b.text[at]) != 1 {
		at++
	}
	if at >= len(b.text) || wordClass(b.text[at]) != 1 {
		e.notify("No word under the cursor")
		return
	}
	start, end := wordStartAt(b, at), at
	for end < len(b.text) && wordClass(b.text[end]) == 1 {
		end++
	}
	e.query = string(b.text[start:end])
	e.searchOpts = searchOptions{caseSensitive: true, wholeWord: true}
	b.cursor = start
	e.find(reverse)
	e.takeCount()
}

func (e *editor) paragraph(dir int) {
	b := e.b
	blank := func(row int) bool { return strings.TrimSpace(string(b.text[b.lines[row]:b.lineEnd(row)])) == "" }
	row := b.rowAt(b.cursor)
	for range e.takeCount() {
		for row+dir >= 0 && row+dir < len(b.lines) && blank(row) {
			row += dir
		}
		for row+dir >= 0 && row+dir < len(b.lines) && !blank(row) {
			row += dir
		}
	}
	b.cursor = b.lines[row]
	e.clampModalCursor()
}

func (e *editor) firstNonBlank(row int) int {
	b := e.b
	at := b.lines[row]
	for at < b.lineEnd(row) && (b.text[at] == ' ' || b.text[at] == '\t') {
		at++
	}
	return at
}

func (e *editor) screenRow(which int) {
	b := e.b
	rows := min(e.pageRows()+1, len(b.lines)-b.top)
	row := b.top
	switch which {
	case 1:
		row += (rows - 1) / 2
	case 2:
		row += rows - 1
	}
	e.takeCount()
	b.cursor = e.firstNonBlank(min(row, len(b.lines)-1))
	e.clampModalCursor()
}

func (e *editor) halfPage(dir int) {
	b := e.b
	n := max(1, (e.pageRows()+1)/2)
	row := max(0, min(len(b.lines)-1, b.rowAt(b.cursor)+dir*n))
	col := b.cursor - b.lines[b.rowAt(b.cursor)]
	b.cursor = min(b.lines[row]+col, b.lineEnd(row))
	b.top = max(0, min(len(b.lines)-1, b.top+dir*n))
	e.takeCount()
	e.clampModalCursor()
}

// scrollTo puts the cursor line at a fraction of the window (0 top, 1 bottom).
func (e *editor) scrollTo(at float64) {
	b := e.b
	h := e.pageRows() + 1
	b.top = max(0, b.rowAt(b.cursor)-int(float64(h-1)*at))
}
