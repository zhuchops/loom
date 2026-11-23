//go:build desktop

package editor

import (
	"image"

	"gioui.org/io/pointer"
)

// paintField draws a one-line input whose text runs from x to right around
// cy: the text scrolled to keep the caret in sight, the selection and, when
// active, the caret. Pressing in r places the caret, double and triple
// clicks select a word and everything, and dragging selects. focus runs
// first on a press, for fields that take the keyboard by being clicked.
func (d *desktop) paintField(r rect, x, cy, right int, text func() string, f *lineField, active bool, st textStyle, caretH int, placeholder string, focus func()) {
	c := &d.c
	value := text()
	runes := []rune(value)
	avail := max(1, right-x)
	width := func(from, to int) int { return d.width(string(runes[from:to]), st) }

	at := f.caret(value)
	f.scroll = max(0, min(f.scroll, at))
	for f.scroll < at && width(f.scroll, at) > avail {
		f.scroll++
	}
	for f.scroll > 0 && width(f.scroll-1, len(runes)) <= avail {
		f.scroll--
	}
	end := f.scroll
	for end < len(runes) && width(f.scroll, end+1) <= avail {
		end++
	}
	start := f.scroll
	px := func(i int) int { return x + width(start, max(start, min(i, end))) }

	// The rune boundary nearest to the pointer.
	index := func() int {
		best, gap := start, 1<<30
		for i := start; i <= end; i++ {
			if g := absInt(px(i) - d.mx); g < gap {
				best, gap = i, g
			}
		}
		return best
	}
	d.zone(zone{r: r, cursor: pointer.CursorText,
		click: func() {
			if focus != nil {
				focus()
			}
			f.press(text(), index(), d.pressShift)
		},
		drag: func() { f.drag(text(), index()) },
	})

	if value == "" {
		if active {
			d.caret(x, cy, caretH)
		}
		d.text(x+d.u(2), cy, d.fit(placeholder, avail, st), st, c.faint)
		return
	}
	if lo, hi, ok := f.selection(value); ok && hi > start && lo < end {
		fill := c.selection
		if !active {
			fill = blend(c.selection, c.well, 0.5)
		}
		d.fill(image.Rect(px(lo), cy-caretH/2, px(hi), cy+caretH/2), fill)
	}
	d.text(x, cy, string(runes[start:end]), st, c.text)
	if active {
		d.caret(px(at), cy, caretH)
	}
}
