package editor

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// findBar is the find and replace panel at the top right of the active pane.
type findBar struct {
	find, replace string
	field         int  // 0 edits the find text, 1 the replacement
	showReplace   bool // the second row is visible
	transient     bool // opened by "/": Enter jumps to a match and closes it
	origin        int  // matches are searched from here as the query is typed
	// Caret and selection of each row. Prefilled find text starts selected,
	// so typing replaces it.
	findField, replaceField lineField
}

type barHit struct {
	hitBox
	action string
}

func (e *editor) openFindBar(replace bool) {
	if e.directory != nil || e.b.dashboard || e.b.tags != nil {
		return
	}
	b := e.b
	bar := &findBar{showReplace: replace, transient: !replace, origin: b.cursor}
	text := ""
	if b.anchor >= 0 && e.mode == "select" {
		s := b.selection()
		if sel := string(b.text[s.start:s.end]); !strings.Contains(sel, "\n") && utf8.RuneCountInString(sel) <= 200 {
			text, bar.origin = sel, s.start
		}
	} else if replace {
		// Like Zed, start from the word under the cursor.
		start, end := b.cursor, b.cursor
		for start > 0 && wordClass(b.text[start-1]) == 1 {
			start--
		}
		for end < len(b.text) && wordClass(b.text[end]) == 1 {
			end++
		}
		text, bar.origin = string(b.text[start:end]), start
	}
	if e.mode == "select" {
		e.normal()
	}
	bar.find = text
	bar.findField.selectAll(text)
	e.bar = bar
	if bar.find == "" && !replace {
		e.query = ""
	}
	e.barSync(bar.find != "")
}

// barSync applies the typed query, and optionally jumps to the nearest match.
func (e *editor) barSync(jump bool) {
	bar := e.bar
	e.query = bar.find
	e.searchVisible = bar.find != ""
	if !jump || bar.find == "" {
		return
	}
	res := e.matches(e.b)
	if i := res.at(bar.origin); i >= 0 {
		e.b.cursor = res.spans[i].start
		e.clampModalCursor()
	}
}

func (e *editor) closeFindBar(keepHighlight bool) {
	e.bar = nil
	e.searchVisible = keepHighlight && e.query != ""
}

func (e *editor) barStep(reverse bool) {
	e.find(reverse)
	e.bar.origin = e.b.cursor
}

func (e *editor) barReplaceCurrent() {
	b := e.b
	res := e.matches(b)
	i := res.at(b.cursor)
	if i < 0 {
		return
	}
	m := res.spans[i]
	text := e.replacement(res, i, e.bar.replace)
	b.replaceAll([]edit{{m, text}}, m.start+utf8.RuneCountInString(text))
	e.clampModalCursor()
	e.bar.origin = b.cursor
	if next := e.matches(b); next.at(b.cursor) >= 0 {
		b.cursor = next.spans[next.at(b.cursor)].start
		e.clampModalCursor()
	}
}

func (e *editor) barReplaceAll() {
	b := e.b
	res := e.matches(b)
	if len(res.spans) == 0 {
		return
	}
	edits := make([]edit, len(res.spans))
	for i, m := range res.spans {
		edits[i] = edit{m, e.replacement(res, i, e.bar.replace)}
	}
	n := len(edits)
	b.replaceAll(edits, res.spans[0].start)
	e.clampModalCursor()
	e.bar.origin = b.cursor
	noun := "matches"
	if n == 1 {
		noun = "match"
	}
	e.notify("Replaced " + strconv.Itoa(n) + " " + noun)
}

// barAction runs a button or key of the find bar.
func (e *editor) barAction(action string) {
	bar := e.bar
	switch action {
	case "case":
		e.searchOpts.caseSensitive = !e.searchOpts.caseSensitive
		e.barSync(true)
	case "word":
		e.searchOpts.wholeWord = !e.searchOpts.wholeWord
		e.barSync(true)
	case "regex":
		e.searchOpts.regex = !e.searchOpts.regex
		e.barSync(true)
	case "next":
		e.barStep(false)
	case "prev":
		e.barStep(true)
	case "close":
		e.closeFindBar(false)
	case "replace":
		bar.transient = false
		if bar.showReplace {
			e.barReplaceCurrent()
		}
	case "all":
		if bar.showReplace {
			e.barReplaceAll()
		}
	case "toggle-replace":
		bar.showReplace, bar.transient = !bar.showReplace, false
		if !bar.showReplace {
			bar.field = 0
		}
	case "focus-find":
		bar.field = 0
	case "focus-replace":
		if bar.showReplace {
			bar.field = 1
		}
	}
}

func (e *editor) barText() *string {
	if e.bar.field == 1 {
		return &e.bar.replace
	}
	return &e.bar.find
}

// barField is the caret and selection of the row being edited.
func (e *editor) barField() *lineField {
	if e.bar.field == 1 {
		return &e.bar.replaceField
	}
	return &e.bar.findField
}

// findInput handles a key while the find bar is open. It reports whether the
// key was consumed; saving still works.
func (e *editor) findInput(in keyInput) bool {
	bar := e.bar
	if in.key == "ctrl+s" {
		return false
	}
	text := e.barText()
	edited := false
	switch in.key {
	case "esc":
		e.closeFindBar(false)
		return true
	case "enter":
		if bar.field == 1 {
			e.barAction("replace")
		} else if bar.transient {
			// Typing already moved to the first match, so Enter confirms it.
			res := e.matches(e.b)
			if i := res.at(e.b.cursor); i < 0 || res.spans[i].start != e.b.cursor {
				e.barStep(false)
			}
			e.closeFindBar(true)
		} else {
			e.barStep(false)
		}
	case "tab", "shift+tab":
		if bar.showReplace {
			bar.field = 1 - bar.field
		}
	case "down", "ctrl+n":
		e.barStep(false)
	case "up", "ctrl+p":
		e.barStep(true)
	case "ctrl+t":
		e.barAction("case")
	case "ctrl+o":
		e.barAction("word")
	case "ctrl+e":
		e.barAction("regex")
	case "ctrl+r":
		e.barAction("toggle-replace")
	case "ctrl+a":
		bar.transient = false
		e.barAction("all")
	default:
		_, edited = e.barField().edit(text, in)
	}
	if edited && bar.field == 0 {
		e.barSync(true)
	}
	return true
}
