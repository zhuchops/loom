package editor

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func (e *editor) typeRune(r rune) {
	b := e.b
	at := b.cursor
	if e.cfg.AutoPairs {
		if _, auto := b.pairs[at]; auto && b.text[at] == r && (!quote(r) || !b.escapedAtCursor()) {
			b.cursor++
			return
		}
		if close := b.autoCloser(r); close != 0 {
			b.replaceAll([]edit{{span{at, at}, string(r) + string(close)}}, at+1)
			// replaceAll builds a fresh pair map for the new text.
			if b.pairs == nil {
				b.pairs = make(map[int]int)
			}
			b.pairs[at+1] = at
			return
		}
	}
	e.insert(string(r))
}

func (b *buffer) autoCloser(r rune) rune {
	at := b.cursor
	if at < len(b.text) && !unicode.IsSpace(b.text[at]) && !strings.ContainsRune(")]},;:", b.text[at]) {
		return 0
	}
	if quote(r) && (at > 0 && wordClass(b.text[at-1]) == 1 || b.escapedAtCursor()) {
		return 0
	}
	return pairCloser(r)
}

func pairCloser(r rune) rune {
	if strings.ContainsRune("([{", r) || quote(r) {
		_, close := delimiters(r)
		return close
	}
	return 0
}

func (e *editor) wrapSelection(r rune) {
	b := e.b
	close := pairCloser(r)
	s := b.selection()
	if close == 0 || b.anchor < 0 || s.start == s.end {
		return
	}
	anchor, cursor := b.anchor, b.cursor
	if b.visualLine {
		e.applyMotionOperator("mini-add", s, true)
		e.miniSurround(e.pending, close)
		return
	}
	b.wrap(s, string(r), string(close), cursor+1)
	b.anchor = anchor + 1
}

func quote(r rune) bool { return r == '\'' || r == '"' || r == '`' }

func (b *buffer) escapedAtCursor() bool {
	at := b.cursor
	i := at
	for i > 0 && b.text[i-1] == '\\' {
		i--
	}
	return (at-i)%2 != 0
}

func (e *editor) deleteAutoPair() bool {
	b := e.b
	open, ok := b.pairs[b.cursor]
	if !e.cfg.AutoPairs || e.mode != "insert" || !ok || open != b.cursor-1 {
		return false
	}
	b.replace(span{open, b.cursor + 1}, "")
	return true
}

// Edits use original offsets. Carry a pair forward only while both of its
// original characters survive. A fresh map keeps undo snapshots immutable.
func shiftAutoPairs(pairs map[int]int, edits []edit) map[int]int {
	if len(pairs) == 0 {
		return nil
	}
	move := func(at int) (int, bool) {
		offset := at
		for _, edit := range edits {
			if at < edit.start {
				break
			}
			if at < edit.end {
				return 0, false
			}
			offset += utf8.RuneCountInString(edit.text) - (edit.end - edit.start)
		}
		return offset, true
	}
	result := make(map[int]int, len(pairs))
	for closer, opener := range pairs {
		nextCloser, closerOK := move(closer)
		nextOpener, openerOK := move(opener)
		if closerOK && openerOK {
			result[nextCloser] = nextOpener
		}
	}
	return result
}
