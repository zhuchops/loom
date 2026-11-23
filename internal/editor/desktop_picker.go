//go:build desktop

package editor

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// listState is the scroll position of the picker; it follows the selection
// whenever that moves.
type listState struct {
	m             *menu
	input         string
	top, selected int
}

type previewCache struct {
	b     *buffer
	spans []highlight
}

var namedChips = []string{"Space", "Esc", "Enter", "Tab", "Backspace", "Delete", "Home", "End", "Pageup", "Pagedown", "Left", "Right", "Up", "Down", "←", "→", "↑", "↓", "↵"}

// isKeys tells key hints ("Space q  :close", "Ctrl+P") from words.
func isKeys(hint string) bool {
	if hint == "" || len(hint) > 48 {
		return false
	}
	for _, group := range strings.Split(hint, "  ") {
		group = strings.TrimSpace(group)
		if len(group) > 1 && group[0] == ':' {
			continue
		}
		for _, k := range strings.Fields(group) {
			switch {
			case utf8.RuneCountInString(k) == 1, slices.Contains(namedChips, k):
			case strings.HasPrefix(k, "Ctrl+"), strings.HasPrefix(k, "Alt+"), strings.HasPrefix(k, "Shift+"):
			case strings.ContainsAny(k, "⌘⇧⌥⌃"):
			case len(k) <= 2 && !strings.ContainsAny(k, "aeiou"):
			default:
				return false
			}
		}
	}
	return true
}

func (d *desktop) chip(x, cy int, s string, draw bool) int {
	w := max(d.u(19), d.keyTextWidth(s, textSmall)+d.u(10))
	if draw {
		r := image.Rect(x, cy-d.u(9.5), x+w, cy+d.u(9.5))
		d.round(r, 5, d.c.key)
		d.border(r, 5, d.c.keyLine, d.hairline())
		d.keyText(x+(w-d.keyTextWidth(s, textSmall))/2, cy, s, textSmall, d.c.dim)
	}
	return w
}

// keys draws a key hint as keycaps ending at right and returns where it starts.
func (d *desktop) keys(right, cy int, hint string) int {
	x := right
	groups := strings.Split(strings.TrimSpace(hint), "  ")
	for i := len(groups) - 1; i >= 0; i-- {
		group := strings.TrimSpace(groups[i])
		if group == "" {
			continue
		}
		if len(group) > 1 && group[0] == ':' {
			x -= d.textRight(x, cy, group, textStyle{size: 11, mono: true}, d.c.faint)
		} else {
			parts := strings.Fields(group)
			for j := len(parts) - 1; j >= 0; j-- {
				w := d.chip(0, cy, parts[j], false)
				x -= w
				d.chip(x, cy, parts[j], true)
				if j > 0 {
					x -= d.u(3)
				}
			}
		}
		if i > 0 {
			x -= d.u(10)
		}
	}
	return x
}

// hints draws alternating keys and what they do.
func (d *desktop) hints(x, cy int, centred bool, pairs ...string) int {
	if centred {
		total := -d.u(16)
		for i := 0; i+1 < len(pairs); i += 2 {
			total += d.chip(0, cy, pairs[i], false) + d.u(5) + d.width(pairs[i+1], textSmall) + d.u(16)
		}
		x -= total / 2
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		x += d.chip(x, cy, pairs[i], true) + d.u(5)
		x += d.text(x, cy, pairs[i+1], textSmall, d.c.dim) + d.u(16)
	}
	return x
}

// scrim dims the window behind a modal surface; clicking it dismisses that.
func (d *desktop) scrim(dismiss func()) {
	r := d.geo.window
	r.Min.Y = d.geo.title.Max.Y
	d.fill(r, d.c.scrim)
	d.fill(d.geo.title, d.c.scrim)
	d.zone(zone{r: d.geo.window, click: dismiss})
}

type pickerKind struct {
	prompt, files, keys, command bool
}

func kindOfPicker(m *menu) pickerKind {
	return pickerKind{
		prompt:  len(m.options) == 0 && m.submit != nil && m.suggestions == nil && m.empty == "",
		files:   m.title == "Go to File" || m.title == "Recent Files" || m.title == "Open Buffers",
		keys:    strings.HasPrefix(m.title, "Keybindings"),
		command: m.title == "Command Palette",
	}
}

func pickerPlaceholder(m *menu, k pickerKind) string {
	switch {
	case k.command:
		return "Type a command…"
	case k.files:
		return "Search files by name…"
	case m.title == "Commit message":
		return "Describe what changed…"
	case strings.HasPrefix(m.title, "Search Project"):
		return "Text to find in this project…"
	case strings.HasPrefix(m.title, "Open Project"):
		return "Search, or type a path…"
	case m.remove != nil:
		return "Switch to a project, or type a path…"
	case strings.HasPrefix(m.title, "Go to Line"):
		return "Line number…"
	case k.prompt:
		return "Type a value…"
	}
	return "Search…"
}

// matchRunes returns which runes of label a fuzzy query matches, in order.
func matchRunes(label, query string) map[int]bool {
	query = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(query), ":")))
	if query == "" {
		return nil
	}
	lower := strings.ToLower(label)
	if at := strings.Index(lower, query); at >= 0 {
		out, start := map[int]bool{}, utf8.RuneCountInString(lower[:at])
		for i := range utf8.RuneCountInString(query) {
			out[start+i] = true
		}
		return out
	}
	out, q, k := map[int]bool{}, []rune(query), 0
	for i, r := range []rune(lower) {
		if k < len(q) && r == q[k] {
			out[i] = true
			k++
		}
	}
	if k < len(q) {
		return nil
	}
	return out
}

// highlighted draws label with the matched letters in the accent colour.
func (d *desktop) highlighted(x, cy, width int, label string, hit map[int]bool, ink color.NRGBA) int {
	r, x0 := []rune(d.fit(label, width, textBody)), x
	for i := 0; i < len(r); {
		j := i
		for j < len(r) && hit[j] == hit[i] {
			j++
		}
		col := ink
		if hit[i] {
			col = d.c.accent
		}
		x += d.text(x, cy, string(r[i:j]), textBody, col)
		i = j
	}
	return x - x0
}

func (d *desktop) paintPicker(e *editor) {
	m, u, c, g := e.menu, d.u, &d.c, &d.geo
	k := kindOfPicker(m)
	d.scrim(func() {
		if e.menu == m {
			e.menu = nil
		}
	})

	// A preview earns its space only when the items have something to show.
	preview := false
	if !m.hidePreview && !k.keys && !k.prompt {
		preview = m.details || slices.ContainsFunc(m.options, func(o option) bool { return o.preview != nil }) || k.files
	}
	room := g.window.Dx() - u(32)
	preview = preview && room >= u(700)
	width := min(u(600), room)
	if preview {
		width = min(u(940), room)
	}

	headH, inputH, rowH, footH := u(34), u(48), u(32), u(34)
	pad := u(6)
	top := g.title.Max.Y + min(u(64), g.window.Dy()/12)
	avail := g.window.Max.Y - top - u(40) - headH - inputH - footH
	body := 0
	if preview {
		body = min(u(420), avail)
	} else if !k.prompt {
		body = min(max(1, min(12, len(m.matches)))*rowH+2*pad, avail)
		if len(m.matches) == 0 {
			body = min(u(72), avail)
		}
	}
	body = max(0, body)
	card := image.Rect((g.window.Dx()-width)/2, top, (g.window.Dx()+width)/2, top+headH+inputH+body+footH)
	d.card(card)
	d.block(card)

	cy := card.Min.Y + headH/2 + u(3)
	title := d.fit(m.title, width-u(120), textSmall)
	d.text(card.Min.X+u(16), cy, title, textSmall, c.dim)
	if !k.prompt && len(m.options) > 0 {
		count := fmt.Sprint(len(m.matches))
		if len(m.matches) < len(m.options) {
			count = fmt.Sprintf("%d of %d", len(m.matches), len(m.options))
		}
		d.textRight(card.Max.X-u(16), cy, count, textSmall, c.faint)
	}

	field := image.Rect(card.Min.X+u(10), card.Min.Y+headH, card.Max.X-u(10), card.Min.Y+headH+inputH-u(8))
	cy = (field.Min.Y + field.Max.Y) / 2
	if m.confirmation {
		d.text(field.Min.X+u(6), cy, "Review the changes, then press Enter to apply them.", textBody, c.text)
	} else {
		d.round(field, 7, c.well)
		d.border(field, 7, blend(c.well, c.accent, 0.55), d.hairline())
		x := field.Min.X + u(12)
		if !k.prompt {
			d.icon(iconSearch, x+u(6), cy, 14, c.dim)
			x += u(22)
		}
		d.paintField(field, x, cy, field.Max.X-u(12), func() string { return m.input }, &m.field, true, textInput, u(18), pickerPlaceholder(m, k), nil)
	}

	split := card.Min.Y + headH + inputH
	foot := card.Max.Y - footH
	if body > 0 {
		d.lineH(card.Min.X, card.Max.X, split)
		listRight := card.Max.X
		if preview {
			listRight = card.Min.X + width*43/100
			d.lineV(listRight, split, foot)
			d.paintPreview(e, m, image.Rect(listRight+d.hairline(), split+d.hairline(), card.Max.X, foot))
		}
		d.paintPickerList(e, m, k, image.Rect(card.Min.X, split+pad, listRight, foot-pad/2), rowH)
	}

	d.lineH(card.Min.X, card.Max.X, foot)
	fy := foot + footH/2
	switch {
	case m.confirmation:
		d.hints(card.Min.X+u(14), fy, false, "↵", "apply", "esc", "cancel")
	case k.prompt:
		d.hints(card.Min.X+u(14), fy, false, "↵", "confirm", "esc", "cancel")
	case m.details:
		d.hints(card.Min.X+u(14), fy, false, "↑↓", "navigate", "↵", "details", "esc", "close")
	case m.remove != nil:
		d.hints(card.Min.X+u(14), fy, false, "↑↓", "navigate", "↵", "switch", "Ctrl+X", "close session", "esc", "cancel")
	default:
		d.hints(card.Min.X+u(14), fy, false, "↑↓", "navigate", "↵", "select", "esc", "close")
	}
}

func (d *desktop) paintPickerList(e *editor, m *menu, k pickerKind, r rect, rowH int) {
	u, c, ls := d.u, &d.c, &d.list
	n, count := len(m.matches), max(1, r.Dy()/rowH)
	if ls.m != m || ls.input != m.input || ls.selected != m.selected {
		if ls.m != m || ls.input != m.input {
			ls.top = 0
		}
		ls.m, ls.input, ls.selected = m, m.input, m.selected
		ls.top = max(m.selected-count+1, min(m.selected, ls.top))
	}
	last := max(0, n-count)
	ls.top = max(0, min(ls.top, last))
	wheel := func(_, dy int) { ls.top = max(0, min(last, ls.top+dy)) }
	d.zone(zone{r: r, wheel: wheel, step: rowH})

	if n == 0 {
		msg := m.empty
		switch {
		case m.submit != nil && strings.TrimSpace(m.input) != "":
			msg = "Press Enter to use “" + m.input + "”"
		case m.input != "":
			msg = "No results for “" + m.input + "”"
		case msg == "" && m.submit != nil:
			msg = "Type, then press Enter"
		case msg == "":
			msg = "Nothing to show"
		}
		d.textCenter((r.Min.X+r.Max.X)/2, r.Min.Y+min(r.Dy()/2, u(48)), d.fit(msg, r.Dx()-u(24), textBody), textBody, c.dim)
		return
	}

	for i := ls.top; i < n && i < ls.top+count; i++ {
		o := m.matches[i]
		y := r.Min.Y + (i-ls.top)*rowH
		row := image.Rect(r.Min.X+u(6), y+d.hairline(), r.Max.X-u(8), y+rowH-d.hairline())
		d.zone(zone{r: row, wheel: wheel, step: rowH, click: func() {
			if e.menu != m {
				return
			}
			m.selected = i
			if !m.confirmation && o.run != nil {
				e.menu = nil
				o.run()
			}
		}})
		switch {
		case i == m.selected:
			d.round(row, 6, c.selected)
		case d.hot(row):
			d.round(row, 6, c.hover)
		}
		x, right, cy := row.Min.X+u(10), row.Max.X-u(10), (row.Min.Y+row.Max.Y)/2
		label, hint := o.label, o.hint
		if k.keys {
			label, hint = o.hint, o.label
		}
		hit := matchRunes(label, m.input)

		detail := ""
		switch {
		case k.files && label == "Dashboard":
			d.logo(x+u(6), cy, 14)
			label, hit = "Welcome", nil
			x += u(22)
		case k.files:
			path, changed := strings.CutSuffix(label, " *")
			if filepath.IsAbs(path) {
				if !e.noProject && within(e.root, path) {
					if rel, err := filepath.Rel(e.root, path); err == nil {
						path = rel
					}
				} else {
					path = shortPath(path)
				}
			}
			d.fileIcon(x+u(6), cy, path)
			x += u(22)
			label = filepath.Base(path)
			if dir := filepath.Dir(path); dir != "." {
				detail = dir
			}
			if changed {
				label += " •"
			}
			hit = matchRunes(label, m.input)
		case m.remove != nil:
			id, ink := glyphFolderOpen, c.accent
			parts := strings.Split(hint, sessionSep)
			switch parts[0] {
			case "":
				id = iconNewFolder
			case "recent":
				id, ink = glyphFolder, c.faint
			}
			d.icon(id, x+u(6), cy, 14, ink)
			x += u(22)
			if hint != "" {
				detail, hint = parts[len(parts)-1], strings.Join(parts[:len(parts)-1], sessionSep)
			}
		case m.title == "Switch Branch":
			d.icon(iconBranch, x+u(6), cy, 13, c.dim)
			x += u(22)
		case strings.HasPrefix(m.title, "Open Project"):
			d.icon(glyphFolder, x+u(6), cy, 14, c.accent)
			x += u(22)
			label = strings.TrimSuffix(label, "/")
			if hint == "browse" {
				hint = ""
			}
		}

		if hint != "" {
			switch {
			case hint == "current":
				d.icon(iconCheck, right-u(6), cy, 14, c.accent)
				right -= u(22)
			case hint == "on" || hint == "off":
				d.toggle(right, cy, hint == "on")
				right -= u(38)
			case isKeys(hint) && (k.keys || k.command || m.title == "Editor Actions" || m.title == "Explorer Actions"):
				limit := row.Min.X + row.Dx()/2
				if x+d.width(label, textBody)+u(12) > limit {
					limit = min(right, x+d.width(label, textBody)+u(12))
				}
				start := right
				groups := strings.Split(hint, "  ")
				// Show as many of the alternatives as the label leaves room for.
				for n := len(groups); n > 0; n-- {
					h := strings.Join(groups[:n], "  ")
					if right-d.keysWidth(h) >= limit {
						start = d.keys(right, cy, h)
						break
					}
				}
				right = start - u(10)
			default:
				lw := d.width(label, textBody)
				space := right - x - lw - u(16)
				if space > u(40) {
					text := strings.Join(strings.Fields(hint), " ")
					if strings.ContainsRune(text, filepath.Separator) && !strings.ContainsRune(text, ' ') {
						text = d.fitLeft(text, space, textSmall)
					} else {
						text = d.fit(text, space, textSmall)
					}
					right -= d.textRight(right, cy, text, textSmall, c.dim) + u(12)
				}
			}
		}
		used := d.highlighted(x, cy, right-x, label, hit, c.text)
		if detail != "" {
			d.text(x+used+u(8), cy, d.fitLeft(detail, max(0, right-x-used-u(8)), textSmall), textSmall, c.faint)
		}
	}
	d.scrollbar(r, ls.top, count, n)
}

func (d *desktop) keysWidth(hint string) int {
	total := 0
	for i, group := range strings.Split(strings.TrimSpace(hint), "  ") {
		if i > 0 {
			total += d.u(10)
		}
		group = strings.TrimSpace(group)
		if len(group) > 1 && group[0] == ':' {
			total += d.width(group, textStyle{size: 11, mono: true})
			continue
		}
		for j, k := range strings.Fields(group) {
			if j > 0 {
				total += d.u(3)
			}
			total += d.chip(0, 0, k, false)
		}
	}
	return total
}

func (d *desktop) toggle(right, cy int, on bool) {
	u := d.u
	track := image.Rect(right-u(28), cy-u(8), right, cy+u(8))
	fill, knob, x := d.c.thumb, d.c.text, track.Min.X+u(8)
	if on {
		fill, knob, x = d.c.accent, white, track.Max.X-u(8)
	}
	d.round(track, 8, fill)
	d.round(image.Rect(x-u(5.5), cy-u(5.5), x+u(5.5), cy+u(5.5)), 6, knob)
}

func (d *desktop) paintPreview(e *editor, m *menu, r rect) {
	u, c := d.u, &d.c
	d.fill(image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y), c.bg)
	d.zone(zone{r: r, wheel: func(_, dy int) { m.previewScroll += dy }, step: u(19)})
	if len(m.matches) == 0 {
		d.textCenter((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2, "Nothing to preview", textBody, c.faint)
		return
	}
	if m.selected < len(m.matches) && m.matches[m.selected].preview == nil {
		d.textCenter((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2, "Nothing to preview", textBody, c.faint)
		return
	}
	p := m.selectedPreview()
	title := p.title
	if filepath.IsAbs(title) {
		if !e.noProject && within(e.root, title) {
			if rel, err := filepath.Rel(e.root, title); err == nil {
				title = rel
			}
		} else {
			title = shortPath(title)
		}
	}
	if title == "Dashboard" {
		title = "Welcome"
	}
	head := r.Min.Y + u(30)
	d.text(r.Min.X+u(14), r.Min.Y+u(15), d.fitLeft(title, r.Dx()-u(28), textSmall), textSmall, c.dim)
	d.lineH(r.Min.X, r.Max.X, head)

	body := image.Rect(r.Min.X, head+u(6), r.Max.X, r.Max.Y-u(4))
	lineH := u(19)
	rows := max(1, body.Dy()/lineH)
	var message []string
	if p.message != "" {
		message = d.wrap(p.message, body.Dx()-u(28), textBody)
	}
	d.clipped(body, func() {
		if p.source == nil {
			m.previewScroll = max(0, min(m.previewScroll, len(message)-rows))
			for i, line := range message[m.previewScroll:min(len(message), m.previewScroll+rows)] {
				d.text(body.Min.X+u(14), body.Min.Y+i*lineH+lineH/2, line, textBody, c.text)
			}
			return
		}
		shown := min(len(message), rows/2)
		for i, line := range message[:shown] {
			d.text(body.Min.X+u(14), body.Min.Y+i*lineH+lineH/2, line, textBody, c.text)
		}
		code := body
		code.Min.Y += shown * lineH
		if shown > 0 {
			code.Min.Y += u(6)
		}
		d.paintSource(e, p, code, lineH, &m.previewScroll)
	})
}

func (d *desktop) paintSource(e *editor, p itemPreview, r rect, lineH int, scroll *int) {
	b, c, u := p.source, &d.c, d.u
	if d.preview.b != b {
		d.preview = previewCache{b: b}
		// Highlighting has to keep up with arrowing through the list.
		if len(b.text) < 200_000 {
			d.preview.spans = highlightSyntax(b, e.cfg.languageFor(b.path))
		}
	}
	count := max(1, r.Dy()/lineH)
	start := min(max(0, len(b.lines)-count), max(0, p.row-count/2))
	*scroll = max(-start, min(*scroll, max(0, len(b.lines)-count)-start))
	start += *scroll
	last := min(len(b.lines), start+count)
	numbers := d.width(strconv.Itoa(last), textCode)
	textX := r.Min.X + u(14) + numbers + u(14)
	at := seekHighlight(d.preview.spans, b.lines[start])
	marks := diagnosticRows(b, start, last)
	tab := e.tabSizeFor(b.path)

	for row := start; row < last; row++ {
		y := r.Min.Y + (row-start)*lineH
		cy := y + lineH/2
		line := image.Rect(r.Min.X, y, r.Max.X, y+lineH)
		num := c.faint
		if row == p.row && (p.row > 0 || p.column > 0) {
			d.fill(line, c.current)
			num = c.text
		}
		if sev := diagnosticAt(marks[row], b.lines[row], b.lineEnd(row)+1); sev != 0 {
			ink := nrgba(diagnosticColor(sev), c.red)
			d.fill(line, blend(c.bg, ink, 0.12))
			d.fill(image.Rect(r.Min.X, y, r.Min.X+u(2), y+lineH), ink)
		}
		d.textRight(textX-u(14), cy, strconv.Itoa(row+1), textCode, num)

		x, column := textX, 0
		var run strings.Builder
		runFG := th.fg
		flush := func() {
			if run.Len() > 0 {
				x += d.text(x, cy, run.String(), textCode, d.cellFG(runFG, c.bg))
				run.Reset()
			}
		}
		end := min(b.lineEnd(row), b.lines[row]+240)
		for off := b.lines[row]; off < end; off++ {
			fg := th.fg
			if style, ok := styleAt(d.preview.spans, &at, off); ok {
				fg = style.fg
			}
			if fg != runFG {
				flush()
				runFG = fg
			}
			switch ch := b.text[off]; ch {
			case '\t':
				n := max(1, tab-column%max(1, tab))
				run.WriteString(strings.Repeat(" ", n))
				column += n
			case '\r':
			default:
				run.WriteRune(ch)
				column++
			}
		}
		flush()
	}
}
