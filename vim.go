package main

import "strings"

func (e *editor) vimCommands() {
	e.add("select-line", "Select Whole Lines", func(e *editor) { e.selectMode(true) })
	e.add("select-other-end", "Move to Other End of Selection", func(e *editor) {
		if e.b.anchor >= 0 {
			e.b.anchor, e.b.cursor = e.b.cursor, e.b.anchor
		}
	})
	for _, op := range []string{"delete", "change"} {
		e.add(op+"-to-end", "", func(e *editor) {
			b := e.b
			last := min(len(b.lines)-1, b.rowAt(b.cursor)+e.takeCount()-1)
			e.operate(op, span{b.cursor, b.lineEnd(last)}, false)
		})
	}
	e.add("yank-line", "", func(e *editor) {
		b := e.b
		row := b.rowAt(b.cursor)
		e.operate("yank", b.lineSpan(row, min(len(b.lines)-1, row+e.takeCount()-1)), true)
	})
	e.add("replace-character", "Replace Character", func(e *editor) { e.pending = pending{kind: "replace-character"} })
	e.add("join-lines", "Join Lines", func(e *editor) { e.joinLines() })
	e.add("match-bracket", "Jump to Matching Bracket", func(e *editor) { e.matchBracket("") })
	e.add("visual-paste", "Replace Selection with Register", func(e *editor) { e.visualPaste(true) })
	e.add("visual-paste-keep", "Replace Selection and Keep Register", func(e *editor) { e.visualPaste(false) })
	for _, key := range []string{"f", "F", "t", "T"} {
		e.add("find-char-"+key, "Find Character ("+key+")", func(e *editor) {
			e.pending = pending{kind: "find-character", op: key}
		})
	}
	e.add("find-char-next", "Repeat Character Find", func(e *editor) { e.findCharacter("", e.lastFind, false, true) })
	e.add("find-char-prev", "Reverse Character Find", func(e *editor) { e.findCharacter("", e.lastFind, true, true) })
	e.surroundCommands()
}

func (e *editor) selectMode(linewise bool) {
	b := e.b
	if e.mode == "select" && b.anchor >= 0 {
		if b.visualLine == linewise {
			e.normal()
		} else {
			b.visualLine = linewise
		}
		return
	}
	e.mode, b.anchor, b.visualLine = "select", b.cursor, linewise
	if linewise && e.count > 1 {
		row := min(len(b.lines)-1, b.rowAt(b.cursor)+e.takeCount()-1)
		b.cursor = motion(b, "first-nonblank", b.lines[row])
	}
}

func (e *editor) visualPaste(saveSelection bool) {
	b := e.b
	if e.register == "" || b.anchor < 0 {
		return
	}
	s := b.selection()
	value := strings.Repeat(e.register, e.takeCount())
	if b.tags != nil && b.visualLine {
		e.notify("Use linewise yank and paste in the directory editor")
		return
	}
	if b.visualLine && s.end < len(b.text) && !strings.HasSuffix(value, "\n") {
		value += b.newline()
	}
	if b.visualLine && s.end == len(b.text) && (s.end == 0 || b.text[s.end-1] != '\n') && e.linewise {
		value = strings.TrimSuffix(value, b.newline())
	}
	if !b.visualLine && e.linewise {
		value = b.newline() + value
	}
	if saveSelection {
		e.register, e.linewise, e.registerTags = string(b.text[s.start:s.end]), b.visualLine, nil
		if e.linewise && !strings.HasSuffix(e.register, "\n") {
			e.register += b.newline()
		}
	}
	b.replace(s, value)
	b.cursor = s.start
	e.normal()
}

func (e *editor) joinLines() {
	b := e.b
	first := b.rowAt(b.cursor)
	last := first + max(2, e.takeCount()) - 1
	if e.mode == "select" && b.anchor >= 0 {
		first, last = b.rowAt(min(b.anchor, b.cursor)), b.rowAt(max(b.anchor, b.cursor))
		last = max(first+1, last)
	}
	last = min(last, len(b.lines)-1)
	if last > 0 && b.lines[last] == len(b.text) {
		last--
	}
	if first >= last {
		return
	}
	var edits []edit
	for row := first; row < last; row++ {
		start, end := b.lineEnd(row), b.lines[row+1]
		for end < b.lineEnd(row+1) && (b.text[end] == ' ' || b.text[end] == '\t') {
			end++
		}
		space := ""
		if end < b.lineEnd(row+1) && b.text[end] != ')' && start > b.lines[row] && b.text[start-1] != ' ' && b.text[start-1] != '\t' {
			space = " "
		}
		edits = append(edits, edit{span{start, end}, space})
	}
	b.replaceAll(edits, edits[0].start)
	e.normal()
}

func (e *editor) matchBracket(op string) {
	b := e.b
	if e.count > 0 {
		percent := e.takeCount()
		if percent > 100 {
			return
		}
		last := len(b.lines)
		if last > 1 && b.lines[last-1] == len(b.text) {
			last--
		}
		row := (last*percent+99)/100 - 1
		if op == "" {
			b.cursor = motion(b, "first-nonblank", b.lines[row])
		} else {
			current := b.rowAt(b.cursor)
			e.applyMotionOperator(op, b.lineSpan(min(current, row), max(current, row)), true)
		}
		return
	}
	start, end := b.cursor, b.lineEnd(b.rowAt(b.cursor))
	for start < end && !strings.ContainsRune("()[]{}", b.text[start]) {
		start++
	}
	if start >= end {
		return
	}
	open, close := delimiters(b.text[start])
	step := 1
	if b.text[start] == close {
		open, close, step = close, open, -1
	}
	depth := 1
	for at := start + step; at >= 0 && at < len(b.text); at += step {
		switch b.text[at] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				if op == "" {
					b.cursor = at
				} else {
					e.applyMotionOperator(op, span{min(b.cursor, at), max(b.cursor, at) + 1}, false)
				}
				return
			}
		}
	}
}

type characterFind struct {
	target     rune
	back, till bool
}

func (e *editor) findCharacter(op string, find characterFind, reverse, again bool) {
	n := e.takeCount()
	if find.target == 0 {
		return
	}
	b := e.b
	row := b.rowAt(b.cursor)
	step := 1
	if find.back != reverse {
		step = -1
	}
	at := b.cursor
	for {
		at += step
		if at < b.lines[row] || at >= b.lineEnd(row) {
			return
		}
		if b.text[at] != find.target || (again && find.till && at == b.cursor+step) {
			continue
		}
		n--
		if n == 0 {
			break
		}
	}
	if find.till {
		at -= step
	}
	if op == "" {
		b.cursor = at
		return
	}
	s := span{min(b.cursor, at), max(b.cursor, at)}
	if step > 0 {
		s.end++
	}
	e.applyMotionOperator(op, s, false)
}
