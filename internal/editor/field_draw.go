package editor

// fieldHit is where a text field was drawn, for the mouse.
type fieldHit struct {
	hitBox // start is the first rune shown
	field  *lineField
	text   *string
	focus  func() // gives the field the keyboard, or nil
}

// index is the rune boundary under column x: a click on a character puts
// the caret before it.
func (h fieldHit) index(x int) int {
	r := []rune(*h.text)
	col := h.x
	for i := min(h.start, len(r)); i < len(r); i++ {
		w := max(1, cellWidth(string(r[i])))
		if x-col < (w+1)/2 {
			return i
		}
		col += w
	}
	return len(r)
}

// drawField writes a one-line input in width cells at x, y, scrolled so the
// caret stays in sight, with its selection marked. It returns the caret's
// column.
func (v *renderer) drawField(x, y, width int, text *string, f *lineField, style cellStyle, focus func()) int {
	r := []rune(*text)
	at := f.caret(*text)
	cells := func(from, to int) int { return cellWidth(string(r[from:to])) }
	room := max(1, width-1) // a cell for the caret after the last character
	f.scroll = max(0, min(f.scroll, at))
	for f.scroll < at && cells(f.scroll, at) > room {
		f.scroll++
	}
	for f.scroll > 0 && cells(f.scroll-1, len(r)) <= room {
		f.scroll--
	}
	start, end := f.scroll, f.scroll
	for end < len(r) && cells(start, end+1) <= width {
		end++
	}
	v.write(x, y, width, string(r[start:end]), style)
	if lo, hi, ok := f.selection(*text); ok {
		lo, hi = max(lo, start), min(hi, end)
		if lo < hi {
			v.write(x+cells(start, lo), y, cells(lo, hi), string(r[lo:hi]), style.Background(selectStyle))
		}
	}
	v.fields = append(v.fields, fieldHit{hitBox{x, y, width, 1, start}, f, text, focus})
	return x + cells(start, max(start, at))
}

// fieldMouse places the caret and selects in a text field. It reports
// whether the event was the field's.
func (v *renderer) fieldMouse(x, y int, press, down, shift bool) bool {
	if press {
		v.fieldDrag = nil
		for _, h := range v.fields {
			if h.contains(x, y) {
				if h.focus != nil {
					h.focus()
				}
				h.field.press(*h.text, h.index(x), shift)
				v.fieldDrag = &h
				return true
			}
		}
		return false
	}
	if h := v.fieldDrag; h != nil {
		if down {
			h.field.drag(*h.text, h.index(x))
		} else {
			v.fieldDrag = nil
		}
		return true
	}
	return false
}
