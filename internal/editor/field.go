package editor

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// lineField is the caret and selection of a one-line text input whose text
// is kept elsewhere: a picker's query, the rename prompt, the find bar. Both
// ends are counted in runes from the end of the text, so code that sets the
// text directly leaves the caret at its end.
type lineField struct {
	caretBack  int
	anchorBack int  // the other end of the selection
	selecting  bool // there is a selection, from the anchor to the caret
	scroll     int  // first rune shown, kept by the drawing code

	// Clicks in quick succession select a word, then everything.
	clickAt     time.Time
	clicks, hit int
}

func clampRunes(back, n int) int { return max(0, min(back, n)) }

// caret is the rune index of the caret in text.
func (f *lineField) caret(text string) int {
	n := utf8.RuneCountInString(text)
	f.caretBack = clampRunes(f.caretBack, n)
	return n - f.caretBack
}

// selection is the selected rune range of text, if any.
func (f *lineField) selection(text string) (lo, hi int, ok bool) {
	if !f.selecting {
		return 0, 0, false
	}
	n := utf8.RuneCountInString(text)
	a, c := n-clampRunes(f.anchorBack, n), f.caret(text)
	if a == c {
		return 0, 0, false
	}
	return min(a, c), max(a, c), true
}

// selected is the selected text.
func (f *lineField) selected(text string) string {
	lo, hi, ok := f.selection(text)
	if !ok {
		return ""
	}
	return string([]rune(text)[lo:hi])
}

// place moves the caret to rune index at. With extend the selection grows
// from where the caret was; without it the selection goes away.
func (f *lineField) place(text string, at int, extend bool) {
	n := utf8.RuneCountInString(text)
	if extend && !f.selecting {
		f.selecting, f.anchorBack = true, clampRunes(f.caretBack, n)
	}
	if !extend {
		f.selecting = false
	}
	f.caretBack = n - max(0, min(at, n))
}

func (f *lineField) selectRange(text string, lo, hi int) {
	f.place(text, lo, false)
	f.place(text, hi, true)
}

func (f *lineField) selectAll(text string) { f.selectRange(text, 0, utf8.RuneCountInString(text)) }

// selectWord selects the word around rune index at, or the run of spaces or
// punctuation there.
func (f *lineField) selectWord(text string, at int) {
	r := []rune(text)
	at = max(0, min(at, len(r)))
	if at == len(r) && at > 0 {
		at--
	}
	if len(r) == 0 {
		return
	}
	same := func(c rune) bool {
		return isWordRune(c) == isWordRune(r[at]) && unicode.IsSpace(c) == unicode.IsSpace(r[at])
	}
	lo, hi := at, at
	for lo > 0 && same(r[lo-1]) {
		lo--
	}
	for hi < len(r) && same(r[hi]) {
		hi++
	}
	f.selectRange(text, lo, hi)
}

// press handles a mouse press at rune index at: one click places the caret
// (Shift extends the selection), two select a word, three the whole text.
func (f *lineField) press(text string, at int, shift bool) {
	now := time.Now()
	if now.Sub(f.clickAt) < 450*time.Millisecond && f.hit == at && !shift {
		f.clicks++
	} else {
		f.clicks = 1
	}
	f.clickAt, f.hit = now, at
	switch f.clicks {
	case 1:
		f.place(text, at, shift)
	case 2:
		f.selectWord(text, at)
	default:
		f.selectAll(text)
	}
}

// drag extends the selection to rune index at while the button is held.
func (f *lineField) drag(text string, at int) {
	if f.clicks <= 1 {
		f.place(text, at, true)
		return
	}
	// After a double click, dragging selects whole words.
	lo, hi, ok := f.selection(text)
	if !ok {
		lo, hi = f.caret(text), f.caret(text)
	}
	r := []rune(text)
	if at < lo {
		f.selectRange(text, hi, wordBefore(r, at+1))
	} else {
		f.selectRange(text, lo, max(hi, wordAfter(r, at)))
	}
}

// replace puts with in place of runes lo to hi and leaves the caret after it.
func (f *lineField) replace(text *string, lo, hi int, with string) {
	r := []rune(*text)
	lo, hi = max(0, min(lo, len(r))), max(0, min(hi, len(r)))
	*text = string(r[:lo]) + with + string(r[hi:])
	f.selecting = false
	f.caretBack = len(r) - hi
}

// parts splits text for drawing: before the selection, the selection, after.
// Without a selection the caret splits it and the middle is empty.
func (f *lineField) parts(text string) (before, middle, after string) {
	r := []rune(text)
	lo, hi, ok := f.selection(text)
	if !ok {
		lo = f.caret(text)
		hi = lo
	}
	return string(r[:lo]), string(r[lo:hi]), string(r[hi:])
}

var singleLine = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ")

// edit applies a key to text. handled reports whether the key belongs to the
// field and changed whether the text changed.
func (f *lineField) edit(text *string, in keyInput) (handled, changed bool) {
	r := []rune(*text)
	at := f.caret(*text)
	lo, hi, sel := f.selection(*text)
	before := *text
	cut := func(from, to int) {
		if sel {
			from, to = lo, hi
		}
		if from != to {
			f.replace(text, min(from, to), max(from, to), "")
		}
	}
	move := func(to, collapse int) {
		if sel && !in.shift && collapse >= 0 {
			to = collapse // an arrow first drops the selection at its side
		}
		f.place(*text, to, in.shift)
	}
	switch in.key {
	case "left", "ctrl+b":
		move(at-1, lo)
	case "right", "ctrl+f":
		move(at+1, hi)
	case "home", "ctrl+a":
		move(0, -1)
	case "end", "ctrl+e":
		move(len(r), -1)
	case "alt+left", "ctrl+left", "alt+b":
		move(wordBefore(r, at), -1)
	case "alt+right", "ctrl+right", "alt+f":
		move(wordAfter(r, at), -1)
	case "backspace", "ctrl+h":
		cut(at-1, at)
	case "delete", "ctrl+d":
		cut(at, at+1)
	case "alt+backspace", "ctrl+w", "ctrl+backspace":
		cut(wordBefore(r, at), at)
	case "alt+delete", "alt+d", "ctrl+delete":
		cut(at, wordAfter(r, at))
	case "ctrl+u", "super+backspace":
		cut(0, at)
	case "ctrl+k", "super+delete":
		cut(at, len(r))
	default:
		typed := singleLine.Replace(in.paste)
		if in.paste == "" {
			ch, ok := inputRune(in.key)
			if !ok || !unicode.IsPrint(ch) {
				return false, false
			}
			typed = string(ch)
		}
		if sel {
			f.replace(text, lo, hi, typed)
		} else {
			f.replace(text, at, at, typed)
		}
	}
	return true, *text != before
}
