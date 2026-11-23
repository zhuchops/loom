//go:build desktop

package editor

import (
	"image"
	"slices"
	"strconv"
	"strings"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op"
)

func (d *desktop) paintFind(e *editor) {
	bar, u, c := e.bar, d.u, &d.c
	if bar == nil {
		return
	}
	area := d.geo.editor
	if p := d.activePane(); p != nil {
		area = d.boxRect(p.hitBox, area)
	}
	width := min(u(480), area.Dx()-u(16))
	if width < u(230) {
		area = d.geo.editor
		width = min(u(480), area.Dx()-u(16))
	}
	height := u(44)
	if bar.showReplace {
		height += u(36)
	}
	x0, y0 := area.Max.X-width-u(14), area.Min.Y+u(8)
	card := image.Rect(x0, y0, x0+width, y0+height)
	d.card(card)
	d.block(card)
	cy := y0 + u(22)
	toggle := iconRight
	if bar.showReplace {
		toggle = iconDown
	}
	d.iconButton(x0+u(18), cy, toggle, false, func() { e.barAction("toggle-replace") })

	field := func(r rect, value *string, f *lineField, placeholder, action string, active bool, reserve int) {
		d.round(r, 6, c.well)
		line := c.line
		if active {
			line = c.accent
		}
		d.border(r, 6, line, d.hairline())
		x, mid := r.Min.X+u(9), (r.Min.Y+r.Max.Y)/2
		d.paintField(image.Rect(r.Min.X, r.Min.Y, r.Max.X-reserve, r.Max.Y), x, mid, r.Max.X-u(11)-reserve,
			func() string { return *value }, f, active, textBody, u(15), placeholder, func() { e.barAction(action) })
	}

	right := card.Max.X - u(6)
	button := func(id iconID, action string) {
		d.iconButton(right-u(12), cy, id, false, func() { e.barAction(action) })
		right -= u(25)
	}
	button(iconClose, "close")
	button(iconDown, "next")
	button(iconUp, "prev")

	find := image.Rect(x0+u(34), cy-u(14), right-u(4), cy+u(14))
	options := []struct {
		label, action string
		on            bool
	}{{".*", "regex", e.searchOpts.regex}, {"ab", "word", e.searchOpts.wholeWord}, {"Aa", "case", e.searchOpts.caseSensitive}}
	count, bad := e.findCount(e.matches(e.b))
	countW := d.width(count, textSmall)
	reserve := len(options)*u(25) + countW + u(12)
	if find.Dx()-reserve < u(80) {
		count, reserve = "", len(options)*u(25)+u(8)
	}
	field(find, &bar.find, &bar.findField, "Find", "focus-find", bar.field == 0, reserve)
	x := find.Max.X - u(4)
	for _, o := range options {
		r := image.Rect(x-u(23), cy-u(10.5), x, cy+u(10.5))
		col := c.dim
		hot := d.button(r, func() { e.barAction(o.action) })
		switch {
		case o.on:
			d.round(r, 5, c.selected)
			col = c.accent
		case hot:
			d.round(r, 5, c.hover)
		}
		d.textCenter((r.Min.X+r.Max.X)/2, cy, o.label, textSmall, col)
		x = r.Min.X - u(2)
	}
	if count != "" {
		col := c.dim
		if bad {
			col = readable(c.red, c.well, 4)
		}
		d.textRight(x-u(4), cy, count, textSmall, col)
	}

	if bar.showReplace {
		cy2 := cy + u(36)
		end := d.textButton(card.Max.X-u(10), cy2, "All", false, func() { e.barAction("all") }) - u(6)
		end = d.textButton(end, cy2, "Replace", false, func() { e.barAction("replace") }) - u(6)
		field(image.Rect(x0+u(34), cy2-u(14), end, cy2+u(14)), &bar.replace, &bar.replaceField, "Replace", "focus-replace", bar.field == 1, 0)
	}
}

// besideLine places a popover of size w by h below the cell, or above it when
// there is more room there.
func (d *desktop) besideLine(x, y, w, h int, preferAbove bool) rect {
	g := &d.geo
	top, bottom := g.title.Max.Y+d.u(2), g.editor.Max.Y-d.u(2)
	below, above := y+d.cellH+d.u(2), y-d.u(2)
	onTop := below+h > bottom && above-top > bottom-below
	if preferAbove && above-h >= top {
		onTop = true
	}
	x = max(d.u(6), min(x, g.window.Max.X-w-d.u(6)))
	if onTop {
		h = min(h, above-top)
		return image.Rect(x, above-h, x+w, above)
	}
	h = min(h, bottom-below)
	return image.Rect(x, below, x+w, below+h)
}

func (d *desktop) paintCompletion(e *editor) {
	v, u, c := &d.m.view, d.u, &d.c
	if e.mode != "insert" || !v.screen.cursorVisible {
		return
	}
	comp, sig := &e.completion, e.signature
	cx, cy := d.geo.grid.Min.X+d.cellX(v.screen.cursorX), d.geo.grid.Min.Y+v.screen.cursorY*d.cellH

	listAbove := false
	if len(comp.items) > 0 {
		rowH := u(27)
		count := min(9, len(comp.items))
		width := u(260)
		for i, item := range comp.items {
			if i == 60 {
				break
			}
			width = max(width, min(u(460), d.width(item.Label, textCode)+min(u(220), d.width(item.Detail, textSmall))+u(52)))
		}
		width = min(width, d.geo.window.Dx()-u(12))
		r := d.besideLine(cx-u(14), cy, width, count*rowH+u(8), false)
		listAbove = r.Max.Y <= cy
		count = max(1, (r.Dy()-u(8))/rowH)
		first := max(0, comp.selected-count+1)
		d.card(r)
		wheel := func(_, dy int) { comp.selected = max(0, min(len(comp.items)-1, comp.selected+dy)) }
		d.zone(zone{r: r, wheel: wheel, step: rowH})
		for i := first; i < len(comp.items) && i < first+count; i++ {
			item := comp.items[i]
			y := r.Min.Y + u(4) + (i-first)*rowH
			row := image.Rect(r.Min.X+u(4), y, r.Max.X-u(4), y+rowH)
			d.zone(zone{r: row, cursor: pointer.CursorPointer, wheel: wheel, step: rowH, click: func() {
				if i < len(comp.items) {
					comp.selected = i
					e.completionInput("enter")
					e.scheduleLanguage()
					e.scheduleGit()
				}
			}})
			switch {
			case i == comp.selected:
				d.round(row, 6, c.selected)
			case d.hot(row):
				d.round(row, 6, c.hover)
			}
			mid, x, right := y+rowH/2, row.Min.X+u(10), row.Max.X-u(10)
			x += d.text(x, mid, d.fit(item.Label, right-x, textCode), textCode, c.text) + u(16)
			if item.Detail != "" && right-x > u(30) {
				d.textRight(right, mid, d.fit(item.Detail, right-x, textSmall), textSmall, c.dim)
			}
		}
		d.scrollbar(r, first, count, len(comp.items))

		if comp.selected < len(comp.items) {
			if docs := strings.TrimSpace(comp.items[comp.selected].docs()); docs != "" {
				w := u(330)
				x := r.Max.X + u(6)
				if x+w > d.geo.window.Max.X {
					x = r.Min.X - u(6) - w
				}
				if x >= 0 {
					d.prose(image.Rect(x, r.Min.Y, x+w, d.geo.editor.Max.Y), "", docs, new(int))
				}
			}
		}
	}

	if sig.label != "" && sig.start >= 0 && sig.start <= sig.end && sig.end <= len(sig.label) {
		clean := func(s string) string { return strings.Map(signatureDisplayRune, s) }
		before, active, after := clean(sig.label[:sig.start]), clean(sig.label[sig.start:sig.end]), clean(sig.label[sig.end:])
		st, limit := textCode, min(u(640), d.geo.window.Dx()-u(24))
		// Long overloads lose their beginning so the active argument stays visible.
		before = d.fitLeft(before, max(u(40), limit-u(24)-d.width(active, st)), st)
		bw, aw := d.width(before, st), d.width(active, st)
		after = d.fit(after, limit-u(24)-bw-aw, st)
		w := min(limit, bw+aw+d.width(after, st)+u(24))
		r := d.besideLine(cx-u(14), cy, w, u(30), !listAbove)
		d.card(r)
		d.block(r)
		mid, x := (r.Min.Y+r.Max.Y)/2, r.Min.X+u(12)
		x += d.text(x, mid, before, st, c.dim)
		x += d.text(x, mid, active, st, readable(c.yellow, c.raised, 4.5))
		d.text(x, mid, after, st, c.dim)
	}
}

type proseLine struct {
	text  string
	style textStyle
}

// prose shows documentation inside bounds, as tall as it needs to be. Code
// fences are set in the code font.
func (d *desktop) prose(bounds rect, title, body string, scroll *int) {
	lines, _, height := d.proseLayout(body, bounds.Dx(), title != "")
	bounds.Max.Y = min(bounds.Max.Y, bounds.Min.Y+height)
	d.paintProse(bounds, title, lines, scroll)
}

func (d *desktop) proseLayout(body string, width int, titled bool) ([]proseLine, int, int) {
	u := d.u
	var lines []proseLine
	code, used := false, 0
	for _, raw := range strings.Split(strings.TrimSpace(body), "\n") {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "```") {
			code = !code
			continue
		}
		if trimmed == "" && len(lines) > 0 && lines[len(lines)-1].text == "" {
			continue
		}
		st := textBody
		switch {
		case code, strings.HasPrefix(trimmed, "func ") && strings.Contains(trimmed, "("):
			st = textCode
		case strings.HasPrefix(trimmed, "#"):
			st, raw = textStrong, strings.TrimLeft(trimmed, "# ")
		case trimmed == "---":
			raw = ""
		}
		for _, line := range d.wrap(raw, width-u(28), st) {
			lines = append(lines, proseLine{line, st})
			used = max(used, d.width(line, st))
		}
	}
	height := min(14, len(lines))*u(20) + u(16)
	if titled {
		height += u(22)
	}
	return lines, used + u(28), height
}

func (d *desktop) paintProse(r rect, title string, lines []proseLine, scroll *int) {
	u, c, lineH := d.u, &d.c, d.u(20)
	if r.Dy() < lineH {
		return
	}
	d.card(r)
	body := image.Rect(r.Min.X, r.Min.Y+u(8), r.Max.X, r.Max.Y-u(4))
	if title != "" {
		d.text(r.Min.X+u(14), body.Min.Y+u(9), strings.ToUpper(title), textCaption, c.faint)
		body.Min.Y += u(22)
	}
	count := max(1, body.Dy()/lineH)
	last := max(0, len(lines)-count)
	*scroll = max(0, min(*scroll, last))
	d.zone(zone{r: r, wheel: func(_, dy int) { *scroll = max(0, min(last, *scroll+dy)) }, step: lineH})
	for i := *scroll; i < len(lines) && i < *scroll+count; i++ {
		d.text(r.Min.X+u(14), body.Min.Y+(i-*scroll)*lineH+lineH/2, lines[i].text, lines[i].style, c.text)
	}
	d.scrollbar(body, *scroll, count, len(lines))
}

func (d *desktop) paintHover(e *editor) {
	h := &d.m.view.diagnosticHover
	if h.buffer == nil || h.message == "" || len(e.completion.items) > 0 {
		return
	}
	title := h.title
	switch {
	case title == "Hover Documentation":
		title = ""
	case title == "":
		title = "Problem"
	}
	lines, w, height := d.proseLayout(h.message, min(d.u(520), d.geo.window.Dx()-d.u(24)), title != "")
	w = max(w, d.u(160))
	x := d.geo.grid.Min.X + d.cellX(h.x) - d.u(14)
	y := d.geo.grid.Min.Y + h.y*d.cellH
	d.paintProse(d.besideLine(x, y, w, height, false), title, lines, &h.scroll)
}

func (d *desktop) paintWhichKey(e *editor) {
	doc := e
	if e.directory != nil {
		doc = e.directory.doc
	}
	if doc.prefix == "" {
		return
	}
	type entry struct{ key, title string }
	var entries []entry
	bound := doc.keys[doc.cfg.Profile+"."+doc.mode]
	tabs := false
	for _, seq := range sortedKeys(bound) {
		suffix, ok := strings.CutPrefix(seq, doc.prefix+" ")
		if !ok {
			continue
		}
		id := bound[seq]
		if n, ok := strings.CutPrefix(id, "tab-"); ok {
			// One entry stands in for the nine numbered tab shortcuts.
			if _, err := strconv.Atoi(n); err == nil {
				if !tabs {
					tabs = true
					at := strings.TrimRight(keyLabel(suffix), "0123456789")
					entries = append(entries, entry{at + "1–9", "Go to Tab"})
				}
				continue
			}
		}
		title := doc.commands[id].title
		if title == "" {
			title = humanize(id)
		}
		entries = append(entries, entry{keyLabel(suffix), title})
	}
	if len(entries) == 0 {
		return
	}
	slices.SortStableFunc(entries, func(a, b entry) int { return len(strings.Fields(a.key)) - len(strings.Fields(b.key)) })

	u, c, g := d.u, &d.c, &d.geo
	rowH, colW := u(27), u(250)
	area := image.Rect(g.editor.Min.X, g.title.Max.Y, g.editor.Max.X, g.editor.Max.Y)
	if area.Dx() < 2*colW+u(60) {
		area.Min.X = 0
	}
	cols := max(1, min(4, (area.Dx()-u(40))/colW))
	if len(entries) <= 8 {
		cols = 1
	}
	rows := (len(entries) + cols - 1) / cols
	rows = min(rows, max(1, (area.Dy()-u(70))/rowH))
	w, h := min(cols*colW+u(16), area.Dx()-u(16)), rows*rowH+u(48)
	colW = (w - u(16)) / cols
	x0, y0 := (area.Min.X+area.Max.X-w)/2, area.Max.Y-h-u(12)
	if cols == 1 {
		x0 = area.Max.X - w - u(14)
	}
	card := image.Rect(x0, y0, x0+w, y0+h)
	d.card(card)
	d.block(card)
	d.text(x0+u(16), y0+u(19), chordTitle(doc.prefix), textCaption, c.dim)

	keyW := u(24)
	for _, en := range entries {
		keyW = max(keyW, min(u(74), d.width(en.key, textSmall)+u(10)))
	}
	for i, en := range entries[:min(len(entries), rows*cols)] {
		x := x0 + u(16) + i/rows*colW
		mid := y0 + u(38) + i%rows*rowH + rowH/2
		chipW := d.chip(x, mid, en.key, true)
		tx := x + max(keyW, chipW) + u(10)
		d.text(tx, mid, d.fit(en.title, x+colW-tx-u(8), textBody), textBody, c.text)
	}
}

type toast struct {
	seen  time.Time // of the newest notification already handled
	text  string
	until time.Time
}

func (d *desktop) paintToast(e *editor) {
	t, u, c, now := &d.toast, d.u, &d.c, d.gtx.Now
	if n := len(e.notifications); n > 0 && e.notifications[n-1].at.After(t.seen) {
		last := e.notifications[n-1]
		t.seen, t.text = last.at, last.text
		stay := 3500 * time.Millisecond
		if looksBad(last.text) || len(last.text) > 90 {
			stay = 7 * time.Second
		}
		t.until = now.Add(stay)
	}
	// A cleared message takes its toast with it.
	if t.text == "" || !now.Before(t.until) || e.message == "" || e.notificationDetail != nil {
		return
	}
	d.gtx.Execute(op.InvalidateCmd{At: t.until})

	width := min(u(400), d.geo.window.Dx()-u(24))
	lines := d.wrap(t.text, width-u(72), textBody)
	more := len(lines) > 3
	if more {
		lines = slices.Clone(lines[:3])
		lines[2] = d.fit(lines[2]+" …", width-u(72), textBody)
	}
	used := 0
	for _, line := range lines {
		used = max(used, d.width(line, textBody))
	}
	width = min(width, max(u(220), used+u(76)))
	lineH := u(20)
	height := len(lines)*lineH + u(20)
	r := image.Rect(d.geo.window.Max.X-width-u(14), d.geo.editor.Max.Y-height-u(14), d.geo.window.Max.X-u(14), d.geo.editor.Max.Y-u(14))
	d.card(r)
	d.button(r, func() {
		t.until = time.Time{}
		e.openNotifications()
	})
	id, ink := iconInfo, c.accent
	if looksBad(t.text) {
		id, ink = iconError, readable(c.red, c.raised, 3.5)
	}
	d.icon(id, r.Min.X+u(20), r.Min.Y+u(10)+lineH/2, 15, ink)
	for i, line := range lines {
		d.text(r.Min.X+u(36), r.Min.Y+u(10)+i*lineH+lineH/2, line, textBody, c.text)
	}
	d.iconButton(r.Max.X-u(19), r.Min.Y+u(10)+lineH/2, iconClose, false, func() { t.until = time.Time{} })
}

func looksBad(s string) bool {
	s = strings.ToLower(s)
	for _, word := range []string{"error", "failed", "cannot", "can't", "could not", "not found", "denied", "fatal", "invalid", "no such"} {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}

// dialog asks a question that must be answered before carrying on. The first
// choice is the default and the last one cancels.
type dialog struct {
	title, body string
	choices     []dialogChoice
	selected    int
}

type dialogChoice struct {
	label string
	run   func()
}

func (d *desktop) answer(i int) {
	q := d.dialog
	if q == nil {
		return
	}
	d.dialog = nil
	if run := q.choices[i].run; run != nil {
		run()
	}
}

func (d *desktop) dialogKey(ev key.Event) {
	q := d.dialog
	n := len(q.choices)
	switch ev.Name {
	case key.NameEscape:
		d.answer(n - 1)
	case key.NameReturn, key.NameEnter:
		d.answer(q.selected)
	case key.NameTab, key.NameLeftArrow:
		// Buttons run right to left, the default on the right.
		q.selected = (q.selected + 1) % n
	case key.NameRightArrow:
		q.selected = (q.selected + n - 1) % n
	}
}

func (d *desktop) paintDialog() {
	q, u, c := d.dialog, d.u, &d.c
	if q == nil {
		return
	}
	d.scrim(func() {})
	width := min(u(430), d.geo.window.Dx()-u(24))
	lines := d.wrap(q.body, width-u(48), textBody)
	card := d.centred(width, u(128)+len(lines)*u(20))
	d.card(card)
	d.block(card)
	d.text(card.Min.X+u(24), card.Min.Y+u(32), d.fit(q.title, width-u(48), textTitle), textTitle, c.text)
	for i, line := range lines {
		d.text(card.Min.X+u(24), card.Min.Y+u(62)+i*u(20), line, textBody, c.dim)
	}
	right, cy := card.Max.X-u(18), card.Max.Y-u(30)
	for i, choice := range q.choices {
		right = d.textButton(right, cy, choice.label, i == q.selected, func() { d.answer(i) }) - u(8)
	}
}
