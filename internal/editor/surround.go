package editor

import (
	"strings"
	"time"
)

type surroundFlash struct {
	buffer       *buffer
	version      int
	outer, inner span
	until        time.Time
}

func (e *editor) surroundCommands() {
	e.add("mini-add-motion", "Add Surrounding with Motion", func(e *editor) {
		e.pending = pending{kind: "operator", op: "mini-add"}
	})
	e.add("mini-add-selection", "Add Surrounding to Selection", func(e *editor) {
		if e.b.anchor >= 0 {
			e.applyMotionOperator("mini-add", e.b.selection(), e.b.visualLine)
		}
	})
	for id, title := range map[string]string{
		"mini-delete":    "Delete Surrounding",
		"mini-replace":   "Replace Surrounding",
		"mini-find":      "Find Surrounding to the Right",
		"mini-find-back": "Find Surrounding to the Left",
		"mini-highlight": "Highlight Surrounding",
	} {
		e.add(id, title, func(e *editor) {
			kind := id
			if id == "mini-replace" {
				kind = "mini-replace-old"
			}
			e.pending = pending{kind: kind}
		})
	}
}

// Send a motion or text object to the requested operator. Adding a surround
// waits for its delimiter and leaves the selection untouched until then.
func (e *editor) applyMotionOperator(op string, s span, linewise bool) {
	if op != "mini-add" {
		e.operate(op, s, linewise)
		return
	}
	if linewise {
		s = e.b.withoutFinalNewline(s)
	}
	e.pending = pending{kind: "mini-add", target: s, version: e.b.version}
	e.count = 0
}

func (e *editor) miniSurround(p pending, id rune) {
	b := e.b
	switch p.kind {
	case "mini-add":
		if b.version != p.version {
			e.notify("Buffer changed before adding surround")
			return
		}
		s := p.target
		left, right := miniDelimiters(id)
		b.wrap(s, left, right, s.start)
		b.cursor = s.start
		e.normal()
	case "mini-replace-old":
		e.pending = pending{kind: "mini-replace-new", old: id}
	default:
		old := id
		if p.kind == "mini-replace-new" {
			old = p.old
		}
		outer := b.selection()
		for i := range e.takeCount() {
			found, ok := enclosingPair(b, old, outer, i > 0)
			if !ok {
				e.notify("Surrounding delimiters not found")
				return
			}
			outer = found
		}
		inner := span{outer.start + 1, outer.end - 1}
		if strings.ContainsRune("([{<", old) {
			for inner.start < inner.end && (b.text[inner.start] == ' ' || b.text[inner.start] == '\t') {
				inner.start++
			}
			for inner.end > inner.start && (b.text[inner.end-1] == ' ' || b.text[inner.end-1] == '\t') {
				inner.end--
			}
		}
		switch p.kind {
		case "mini-find", "mini-find-back":
			first, last := outer.end-1, outer.start
			if p.kind == "mini-find-back" {
				first, last = last, first
			}
			if b.cursor == first {
				first = last
			}
			b.cursor = first
		case "mini-highlight":
			e.surroundFlash = surroundFlash{b, b.version, outer, inner, time.Now().Add(500 * time.Millisecond)}
		default:
			left, right := "", ""
			if p.kind == "mini-replace-new" {
				left, right = miniDelimiters(id)
			}
			b.replaceAll([]edit{{span{outer.start, inner.start}, left}, {span{inner.end, outer.end}, right}}, outer.start)
			e.normal()
		}
	}
}

func (b *buffer) withoutFinalNewline(s span) span {
	if s.end > s.start && b.text[s.end-1] == '\n' {
		s.end = b.lineEnd(b.rowAt(s.end) - 1)
	}
	return s
}

func (b *buffer) wrap(s span, left, right string, cursor int) {
	if s.start == s.end {
		b.replaceAll([]edit{{s, left + right}}, cursor)
		return
	}
	b.replaceAll([]edit{{span{s.start, s.start}, left}, {span{s.end, s.end}, right}}, cursor)
}

func miniDelimiters(id rune) (string, string) {
	open, close := delimiters(id)
	left, right := string(open), string(close)
	if strings.ContainsRune("([{<", id) {
		left += " "
		right = " " + right
	}
	return left, right
}
