package editor

import "fmt"

// tailToWidth keeps the end of s that fits in width cells, so the text being
// typed stays visible.
func tailToWidth(s string, width int) string {
	r := []rune(s)
	for len(r) > 0 && cellWidth(string(r)) > width {
		r = r[1:]
	}
	return string(r)
}

// drawFindBar draws the find and replace panel over the top right of the
// active pane, in the style of VS Code and Zed.
func (v *renderer) drawFindBar(e *editor) {
	v.barHits = v.barHits[:0]
	bar := e.bar
	var pane *paneBox
	for i := range v.panes {
		if v.panes[i].leaf == e.activePane && !v.panes[i].b.dashboard {
			pane = &v.panes[i]
			break
		}
	}
	if pane == nil {
		return
	}
	w := min(60, pane.w-2)
	if w < 30 {
		return
	}
	h := 3
	title := "Find"
	if bar.showReplace {
		h, title = 4, "Find and Replace"
	}
	box := hitBox{pane.x + pane.w - w - 1, pane.y, w, h, 0}
	v.frame(box, title)
	v.find = findInfo{on: true, replace: bar.showReplace, box: box}

	res := e.matches(e.b)
	inner := box.w - 2
	showCount := inner >= 46
	countW := 0
	if showCount {
		countW = 10
	}
	toggleW, arrowW := 12, 9
	fieldW := inner - 3 - toggleW - countW - arrowW
	x0, y1, y2 := box.x+1, box.y+1, box.y+2
	hit := func(x, y, w int, action string) { v.barHits = append(v.barHits, barHit{hitBox{x, y, w, 1, 0}, action}) }

	// Chevron: shows or hides the replace row.
	chevron := "▸"
	if bar.showReplace {
		chevron = "▾"
	}
	v.find.chevron = hitBox{x0, y1, 3, 1, 0}
	v.write(x0, y1, 2, chevron, menuStyle.Foreground(th.muted))
	hit(x0, y1, 3, "toggle-replace")

	field := func(x, y int, text, placeholder string, active bool, action string) {
		v.find.fields = append(v.find.fields, findField{x, y, fieldW, text, placeholder, active})
		style := baseStyle.Background(th.panel)
		v.screen.FillArea(x, y, fieldW, 1, ' ', style)
		shown := tailToWidth(text, fieldW-2)
		if text == "" {
			v.write(x+1, y, fieldW-2, placeholder, style.Foreground(th.muted))
		} else {
			v.write(x+1, y, fieldW-2, shown, style)
		}
		if active {
			v.write(x+1+cellWidth(shown), y, 1, "▏", style.Foreground(th.accent))
			v.screen.FillArea(x, y, 1, 1, '▎', style.Foreground(th.accent))
		}
		hit(x, y, fieldW, action)
	}
	fx := x0 + 3
	field(fx, y1, bar.find, "Find", bar.field == 0, "focus-find")

	// Toggles: match case, whole word, regular expression.
	toggle := func(x int, label string, on bool, action string) {
		v.find.toggles = append(v.find.toggles, findToggle{x, y1, label, on})
		style := menuStyle.Foreground(th.muted)
		if on {
			style = menuStyle.Background(th.active).Foreground(th.accent).Bold(true)
		}
		v.write(x, y1, 4, fmt.Sprintf(" %-2s ", label), style)
		hit(x, y1, 4, action)
	}
	tx := fx + fieldW
	toggle(tx, "Aa", e.searchOpts.caseSensitive, "case")
	toggle(tx+4, "ab", e.searchOpts.wholeWord, "word")
	toggle(tx+8, ".*", e.searchOpts.regex, "regex")

	// Match count, like "3 of 12".
	cx := tx + toggleW
	if showCount {
		count, style := "", menuStyle.Foreground(th.muted)
		switch {
		case res.err != nil:
			count, style = "Invalid", menuStyle.Foreground(th.red)
		case bar.find == "":
		case len(res.spans) == 0:
			count, style = "No results", menuStyle.Foreground(th.red)
		default:
			cur := res.at(e.b.cursor)
			if cur >= 0 && res.spans[cur].start != e.b.cursor {
				cur = (cur - 1 + len(res.spans)) % len(res.spans)
			}
			count = fmt.Sprintf("%d of %d", cur+1, len(res.spans))
		}
		v.find.count, v.find.countX = count, cx+max(0, countW-cellWidth(count)-1)
		if res.err != nil || len(res.spans) == 0 && bar.find != "" {
			v.find.countTone = "red"
		}
		v.write(cx+max(0, countW-cellWidth(count)-1), y1, countW, count, style)
	}

	// Previous, next and close.
	ax := cx + countW
	arrow := func(x int, glyph, action string) {
		v.find.arrows = append(v.find.arrows, findToggle{x, y1, glyph, false})
		v.write(x, y1, 3, " "+glyph+" ", menuStyle)
		hit(x, y1, 3, action)
	}
	arrow(ax, "↑", "prev")
	arrow(ax+3, "↓", "next")
	arrow(ax+6, "×", "close")

	if bar.showReplace {
		field(fx, y2, bar.replace, "Replace", bar.field == 1, "focus-replace")
		button := func(x int, label, action string) int {
			style := menuStyle.Foreground(th.accent)
			v.find.buttons = append(v.find.buttons, findToggle{x, y2, label, false})
			v.write(x, y2, cellWidth(label), label, style)
			hit(x, y2, cellWidth(label), action)
			return cellWidth(label)
		}
		bx := tx + 1
		bx += button(bx, " Replace ", "replace") + 1
		button(bx, " All ", "all")
	}
}
