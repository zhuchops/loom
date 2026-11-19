package editor

import "unicode"

func (e *editor) registerMarkCommands() {
	e.add("mark-set", "Set Mark", func(e *editor) { e.pending = pending{kind: "mark-set"} })
	e.add("mark-line", "Jump to Mark Line", func(e *editor) { e.pending = pending{kind: "mark-line"} })
	e.add("mark-exact", "Jump to Mark", func(e *editor) { e.pending = pending{kind: "mark-exact"} })
}

func (e *editor) markInput(kind string, r rune) {
	b := e.b
	switch kind {
	case "mark-set":
		if unicode.IsLetter(r) {
			if b.marks == nil {
				b.marks = map[rune]int{}
			}
			b.marks[r] = b.cursor
			e.notify("Mark " + string(r) + " set")
		}
		return
	}
	if r == '\'' || r == '`' {
		e.runCommand("jump-back")
		return
	}
	offset, ok := b.marks[r]
	if !ok {
		e.notify("Mark " + string(r) + " is not set")
		return
	}
	from := e.jumpPoint()
	offset = min(offset, len(b.text))
	if kind == "mark-line" {
		row := b.rowAt(offset)
		offset = b.lines[row]
		for offset < b.lineEnd(row) && (b.text[offset] == ' ' || b.text[offset] == '\t') {
			offset++
		}
	}
	b.cursor = offset
	e.clampModalCursor()
	e.rememberJump(from)
}

// markSelection records '< and '> when a selection ends.
func (b *buffer) markSelection() {
	if b.anchor < 0 {
		return
	}
	if b.marks == nil {
		b.marks = map[rune]int{}
	}
	b.marks['<'], b.marks['>'] = min(b.anchor, b.cursor), max(b.anchor, b.cursor)
}

// shiftMarks keeps marks on the text they were set on across edits.
func shiftMarks(marks map[rune]int, edits []edit) {
	for name, at := range marks {
		shift := 0
		for _, e := range edits {
			switch {
			case e.end <= at:
				shift += len([]rune(e.text)) - (e.end - e.start)
			case e.start < at:
				at, shift = e.start, 0
			}
		}
		marks[name] = max(0, at+shift)
	}
}
