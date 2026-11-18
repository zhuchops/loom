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
	for _, op := range []string{"indent", "outdent"} {
		e.add(op+"-op", "", func(e *editor) { e.pending = pending{kind: "operator", op: op} })
		e.add(op+"-selection", op+" selection", func(e *editor) { e.shiftRows(op, e.b.selection(), e.takeCount()) })
	}
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

// shiftRows indents or outdents every row touched by s, like Vim's > and <.
// In Select mode the selection survives the shift; only Esc removes it.
// Normal mode leaves the cursor on the first nonblank of the first row.
func (e *editor) shiftRows(op string, s span, times int) {
	b := e.b
	first := b.rowAt(s.start)
	last := b.rowAt(max(s.start, s.end-1))
	selecting := e.mode == "select" && b.anchor >= 0
	anchor, visualLine := b.anchor, b.visualLine
	var edits []edit
	for row := first; row <= last; row++ {
		start, end := b.lines[row], b.lineEnd(row)
		if start == end {
			continue
		}
		if op == "indent" {
			leading := start
			for leading < end && (b.text[leading] == ' ' || b.text[leading] == '\t') {
				leading++
			}
			unit := strings.Repeat(e.indentUnit(string(b.text[start:leading])), times)
			edits = append(edits, edit{span{start, start}, unit})
			continue
		}
		n := 0
		for range times {
			k := outdentWidth(b.text[start+n : end])
			if k == 0 {
				break
			}
			n += k
		}
		if n > 0 {
			edits = append(edits, edit{span{start, start + n}, ""})
		}
	}
	if len(edits) > 0 {
		cursor := shiftOffset(b.cursor, edits)
		if anchor >= 0 {
			anchor = shiftOffset(anchor, edits)
		}
		b.replaceAll(edits, cursor)
		// restore clears the selection, so put back what the user still has.
		b.anchor, b.visualLine = anchor, visualLine
	}
	if selecting {
		return
	}
	e.normal()
	b.cursor = motion(b, "first-nonblank", b.lines[first])
}

// outdentWidth is how many leading runes one outdent step removes: a tab, or up to four spaces.
func outdentWidth(line []rune) int {
	if len(line) > 0 && line[0] == '\t' {
		return 1
	}
	n := 0
	for n < len(line) && n < 4 && line[n] == ' ' {
		n++
	}
	return n
}

// shiftOffset maps an offset in the old text to the new text after edits, which are in document order.
func shiftOffset(at int, edits []edit) int {
	shift := 0
	for _, ed := range edits {
		if ed.start > at {
			break
		}
		if at < ed.end {
			return ed.start + shift
		}
		shift += len([]rune(ed.text)) - (ed.end - ed.start)
	}
	return at + shift
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
