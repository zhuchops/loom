//go:build desktop

package editor

import (
	"image"
	"image/color"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"gioui.org/io/pointer"

	"github.com/zhuchops/loom/internal/theme"
)

func (d *desktop) paintSettings(e *editor) {
	s, u, c := d.settings, d.u, &d.c
	// Lighter than other popups: changes to fonts and colours show behind it.
	d.fill(d.geo.window, fade(d.c.scrim, d.c.scrim.A/2))
	d.zone(zone{r: d.geo.window, click: func() { d.settings = nil }})

	card := d.centred(u(960), u(670))
	d.card(card)
	d.block(card)

	head := image.Rect(card.Min.X, card.Min.Y, card.Max.X, card.Min.Y+u(56))
	cy := (head.Min.Y + head.Max.Y) / 2
	d.text(head.Min.X+u(20), cy, "Settings", textTitle, c.text)
	d.iconButton(head.Max.X-u(26), cy, iconClose, false, func() { d.settings = nil })
	navW := u(190)
	if card.Dx() < u(700) {
		navW = u(128)
	}
	field := image.Rect(head.Min.X+navW+u(16), cy-u(15), head.Max.X-u(50), cy+u(15))
	d.settingsField(field, s.query, "Search all settings", s.picker == nil && s.recording == nil, func() { s.picker = nil })
	d.lineH(card.Min.X, card.Max.X, head.Max.Y)

	foot := image.Rect(card.Min.X, card.Max.Y-u(40), card.Max.X, card.Max.Y)
	body := image.Rect(card.Min.X+navW, head.Max.Y+d.hairline(), card.Max.X-d.hairline(), foot.Min.Y)
	d.lineV(body.Min.X, body.Min.Y, body.Max.Y)

	y := head.Max.Y + u(10)
	step := min(u(34), (foot.Min.Y-y-u(6))/len(settingPages))
	for i, page := range settingPages {
		r := image.Rect(card.Min.X+u(8), y, body.Min.X-u(8), y+step-u(3))
		y += step
		on := i == s.page && s.query == ""
		hot := d.button(r, func() { s.show(i) })
		ink, st := c.dim, textBody
		switch {
		case on:
			d.round(r, 7, c.selected)
			ink, st = c.text, textStrong
		case hot:
			d.round(r, 7, c.hover)
			ink = c.text
		}
		d.text(r.Min.X+u(12), (r.Min.Y+r.Max.Y)/2, d.fit(page.title, r.Dx()-u(20), st), st, ink)
	}

	content := body
	content.Min.X += d.hairline()
	if s.picker != nil {
		d.paintFontPicker(e, content)
	} else {
		banner := s.failure
		if banner != "" && s.recording == nil {
			r := image.Rect(content.Min.X+u(14), content.Min.Y+u(10), content.Max.X-u(14), content.Min.Y+u(42))
			fill := blend(c.raised, c.red, 0.14)
			d.round(r, 7, fill)
			d.icon(iconError, r.Min.X+u(16), (r.Min.Y+r.Max.Y)/2, 14, readable(c.red, fill, 3.5))
			text := strings.Join(strings.Fields(banner), " ")
			d.text(r.Min.X+u(30), (r.Min.Y+r.Max.Y)/2, d.fit(text, r.Dx()-u(44), textBody), textBody, c.text)
			content.Min.Y = r.Max.Y
		}
		d.paintSettingsRows(e, content)
	}
	d.paintSettingsFoot(e, foot)

	if s.recording != nil {
		d.paintRecording(e, card)
	}
}

func (d *desktop) settingsField(r rect, value, placeholder string, active bool, click func()) {
	u, c := d.u, &d.c
	d.round(r, 7, c.well)
	line := c.line
	if active {
		line = blend(c.well, c.accent, 0.55)
	}
	d.border(r, 7, line, d.hairline())
	d.zone(zone{r: r, cursor: pointer.CursorText, click: click})
	cy, x := (r.Min.Y+r.Max.Y)/2, r.Min.X+u(32)
	d.icon(iconSearch, r.Min.X+u(16), cy, 13, c.dim)
	if value == "" {
		d.text(x+u(3), cy, d.fit(placeholder, r.Max.X-x-u(10), textBody), textBody, c.faint)
	} else {
		x += d.text(x, cy, d.fitLeft(value, r.Max.X-x-u(10), textBody), textBody, c.text)
	}
	if active {
		d.caret(x+u(1), cy, u(15))
	}
}

func (d *desktop) paintSettingsFoot(e *editor, r rect) {
	s, u, c := d.settings, d.u, &d.c
	d.lineH(r.Min.X, r.Max.X, r.Min.Y)
	cy := (r.Min.Y + r.Max.Y) / 2
	right := r.Max.X - u(10)
	link := func(label string, run func()) {
		box := image.Rect(right-d.width(label, textSmall)-u(16), cy-u(12), right, cy+u(12))
		ink := c.dim
		if d.button(box, run) {
			d.round(box, 6, c.hover)
			ink = c.text
		}
		d.text(box.Min.X+u(8), cy, label, textSmall, ink)
		right = box.Min.X - u(2)
	}
	open := func(path string) func() {
		return func() {
			d.settings = nil
			e.open(path)
		}
	}
	added := addedPath(e.userConfigPath())
	if len(e.cfg.fromProject) > 0 {
		link("Project file", open(filepath.Join(e.root, ".loom.json")))
	}
	if added != "" {
		link("settings.json", open(added))
	}
	link("config.json", func() {
		d.settings = nil
		e.openConfigFile()
	})
	hint := []string{"↑↓", "move", "←→", "change", "Tab", "page", "esc", "close"}
	switch {
	case s.picker != nil:
		hint = []string{"↑↓", "move", "↵", "use", "esc", "back"}
	case r.Dx() < u(720):
		hint = []string{"↑↓", "move", "esc", "close"}
	}
	d.hints(r.Min.X+u(14), cy, false, hint...)
}

func (d *desktop) settingsRowHeight(row settingsRow, width int) int {
	u := d.u
	switch row.kind {
	case rowHeading:
		return u(42)
	case rowPreview:
		return u(34) + len(previewCode)*d.cellH
	case rowSetting:
		if row.set.kind == settingTheme {
			_, _, h := d.themeGrid(width)
			lines := (len(theme.All) + d.themeColumns(width) - 1) / d.themeColumns(width)
			return u(60) + lines*(h+u(10))
		}
		if row.set.help != "" {
			return u(58)
		}
		return u(46)
	case rowLanguage, rowBinding:
		return u(38)
	case rowLanguageValue:
		return u(40)
	case rowLanguageTool:
		return u(32)
	case rowModes:
		return u(54)
	}
	return u(42)
}

func (d *desktop) paintSettingsRows(e *editor, r rect) {
	s, u := d.settings, d.u
	rows := s.rows(e)
	picks := selectableRows(rows)
	s.sel = max(0, min(s.sel, len(picks)-1))
	selected := -1
	if len(picks) > 0 {
		selected = picks[s.sel]
	}

	width := r.Dx() - u(28)
	tops := make([]int, len(rows)+1)
	tops[0] = u(8)
	for i, row := range rows {
		tops[i+1] = tops[i] + d.settingsRowHeight(row, width)
	}
	total := tops[len(rows)] + u(10)
	if s.reveal && selected >= 0 {
		from := tops[selected]
		// The first row brings what is above it along.
		if s.sel == 0 {
			from = 0
		} else if rows[selected-1].kind == rowHeading {
			from = tops[selected-1]
		}
		if from < s.scroll {
			s.scroll = from
		} else if end := tops[selected+1] + u(6); end > s.scroll+r.Dy() {
			s.scroll = min(from, end-r.Dy())
		}
	}
	s.reveal = false
	last := max(0, total-r.Dy())
	s.scroll = max(0, min(s.scroll, last))
	wheelStep := max(1, u(4))
	wheel := func(_, dy int) { s.scroll = max(0, min(last, s.scroll+dy*wheelStep)) }

	d.clipped(r, func() {
		d.zone(zone{r: r, wheel: wheel, step: wheelStep})
		pick := 0
		for i, row := range rows {
			index := pick
			if row.selectable() {
				pick++
			}
			box := image.Rect(r.Min.X+u(14), r.Min.Y+tops[i]-s.scroll, r.Min.X+u(14)+width, r.Min.Y+tops[i+1]-s.scroll)
			if box.Max.Y <= r.Min.Y || box.Min.Y >= r.Max.Y {
				continue
			}
			d.paintSettingsRow(e, row, box, index, i == selected, wheel, wheelStep)
		}
		d.scrollbar(r, s.scroll, r.Dy(), total)
	})
}

func (d *desktop) paintSettingsRow(e *editor, row settingsRow, r rect, index int, selected bool, wheel func(int, int), step int) {
	s, u, c := d.settings, d.u, &d.c
	cy := (r.Min.Y + r.Max.Y) / 2
	x, right := r.Min.X+u(12), r.Max.X-u(12)

	act := func(run func()) func() {
		return func() {
			s.sel, s.failure = index, ""
			run()
		}
	}
	control := func(box rect, run func()) bool {
		d.zone(zone{r: box, click: act(run), cursor: pointer.CursorPointer, wheel: wheel, step: step})
		return d.hot(box)
	}

	switch row.kind {
	case rowHeading:
		d.text(x, r.Max.Y-u(13), strings.ToUpper(row.label), textCaption, c.faint)
		return
	case rowNote:
		d.text(x, cy, d.fit(row.label, right-x, textSmall), textSmall, c.dim)
		return
	case rowPreview:
		d.paintCodePreview(e, r)
		return
	case rowLanguageTool:
		d.text(x+u(22), cy, row.label, textSmall, c.dim)
		ink := c.dim
		if strings.HasSuffix(row.detail, "installed") {
			ink = readable(c.green, c.raised, 4)
		}
		d.textRight(right, cy, d.fit(row.detail, right-x-u(160), textSmall), textSmall, ink)
		return
	}

	line := r.Inset(d.hairline())
	whole := act(func() {})
	switch {
	case row.kind == rowLanguage, row.kind == rowBinding:
		whole = act(func() { d.activateRow(row, 1, true) })
	case row.kind == rowSetting && (row.set.kind == settingSwitch || row.set.kind == settingFont):
		whole = act(func() { d.activateRow(row, 1, true) })
	}
	d.zone(zone{r: line, click: whole, wheel: wheel, step: step})
	hot := d.hot(line)
	isTheme := row.kind == rowSetting && row.set.kind == settingTheme
	if !isTheme {
		switch {
		case selected:
			d.round(line, 8, c.hover)
			d.round(image.Rect(line.Min.X, cy-u(9), line.Min.X+u(3), cy+u(9)), 2, c.accent)
		case hot:
			d.round(line, 8, blend(c.raised, c.text, 0.035))
		}
	}
	reset := func(at int) {
		box := image.Rect(at-u(44), cy-u(11), at, cy+u(11))
		ink := c.dim
		if control(box, func() { d.resetRow(row) }) {
			d.round(box, 5, c.key)
			ink = c.text
		}
		d.textCenter((box.Min.X+box.Max.X)/2, cy, "Reset", textSmall, ink)
	}

	switch row.kind {
	case rowSetting:
		if isTheme {
			d.paintThemes(e, row, r, selected, control)
			return
		}
		set := row.set
		locked, live := e.projectSets(set.key), switchOn(e, set)
		ink := c.text
		if locked || !live {
			ink = c.faint
		}
		controlLeft := d.paintSettingControl(e, set, right, cy, control)
		if locked {
			w := d.chip(0, cy, "Project", false)
			controlLeft -= w + u(8)
			d.chip(controlLeft, cy, "Project", true)
		} else if e.userSets(set.key) && (hot || selected) && controlLeft-x > u(200) {
			reset(controlLeft - u(6))
			controlLeft -= u(52)
		}
		room, title := controlLeft-x-u(12), cy
		help := set.help
		if set.kind == settingFont {
			if name := e.cfg.text(set.key); name != "" {
				if family := fonts().find(name); family == nil {
					help = name + " isn't installed, so the default is used"
				} else if set.key == "font_family" && !family.traits().mono {
					help = name + " isn't monospaced, so the default is used"
				}
			}
		}
		if set.help != "" {
			title = cy - u(9)
			d.text(x, cy+u(10), d.fit(help, room, textSmall), textSmall, c.dim)
		}
		used := d.text(x, title, d.fit(set.title, room, textBody), textBody, ink)
		if e.userSets(set.key) && !locked && used+u(16) < room {
			k := u(2.5)
			d.round(image.Rect(x+used+u(8)-k, title-k, x+used+u(8)+k, title+k), 3, c.accent)
		}

	case rowLanguage:
		lang := e.cfg.Languages[row.id]
		icon := iconRight
		if s.language == row.id {
			icon = iconDown
		}
		d.icon(icon, x+u(6), cy, 11, c.dim)
		name := languageName(row.id)
		x += u(22)
		x += d.text(x, cy, name, textBody, c.text) + u(12)
		var changed []string
		if lang.TabSize > 0 {
			changed = append(changed, "tab "+strconv.Itoa(lang.TabSize))
		}
		if lang.IndentStyle != "" {
			changed = append(changed, lang.IndentStyle)
		}
		if lang.FormatOnSave != nil {
			state := "format off"
			if *lang.FormatOnSave {
				state = "format on"
			}
			changed = append(changed, state)
		}
		summary, ink := "", c.faint
		if len(changed) > 0 {
			summary, ink = strings.Join(changed, ", "), c.accent
		} else {
			for i, ext := range lang.Extensions {
				if i == 4 {
					break
				}
				summary += "." + ext + " "
			}
		}
		d.textRight(right, cy, d.fit(strings.TrimSpace(summary), right-x, textSmall), textSmall, ink)

	case rowLanguageValue:
		d.text(x+u(22), cy, row.label, textBody, c.text)
		choices := languageChoices(row.field)
		current := languageValue(e.cfg.Languages[row.id], row.field)
		on := slices.IndexFunc(choices, func(o languageChoice) bool { return o.value == current })
		labels := make([]string, len(choices))
		for i, o := range choices {
			labels[i] = o.label
		}
		if on < 0 { // a tab size written by hand
			if n, ok := current.(int); ok {
				labels = append(labels, strconv.Itoa(n))
				on = len(labels) - 1
			}
		}
		d.segments(right, cy, labels, on, control, func(i int) {
			if i < len(choices) {
				s.report(e.setLanguageSetting(row.id, row.field, choices[i].value))
			}
		})

	case rowModes:
		d.text(x, cy, row.label, textBody, c.text)
		labels := make([]string, len(keyModes))
		for i, m := range keyModes {
			labels[i] = m.title
		}
		d.segments(right, cy, labels, s.mode, control, func(i int) { s.mode = i })

	case rowBinding:
		var parts []string
		for _, seq := range keysFor(e.keys[e.cfg.Profile+"."+keyModes[s.mode].id], row.id) {
			if len(parts) < 3 {
				parts = append(parts, keyLabel(seq))
			}
		}
		left := right
		if len(parts) == 0 {
			left -= d.textRight(right, cy, "—", textSmall, c.faint)
		} else {
			left = d.keys(right, cy, strings.Join(parts, "  "))
		}
		changed := s.keysChanged(e, row.id)
		if changed && (hot || selected) {
			reset(left - u(8))
			left -= u(54)
		}
		used := d.text(x, cy, d.fit(row.label, left-x-u(20), textBody), textBody, c.text)
		if changed {
			k := u(2.5)
			d.round(image.Rect(x+used+u(8)-k, cy-k, x+used+u(8)+k, cy+k), 3, c.accent)
		}
	}
}

// paintSettingControl draws the control at the right of a row and returns
// where it starts.
func (d *desktop) paintSettingControl(e *editor, set setting, right, cy int, control func(rect, func()) bool) int {
	u, c := d.u, &d.c
	switch set.kind {
	case settingSwitch:
		on := e.cfg.on(set.key) && switchOn(e, set)
		d.toggle(right, cy, on)
		return right - u(28)

	case settingChoice:
		labels := make([]string, len(set.choices))
		for i, v := range set.choices {
			labels[i] = humanize(v)
		}
		return d.segments(right, cy, labels, slices.Index(set.choices, e.cfg.text(set.key)), control, func(i int) {
			if e.projectSets(set.key) {
				d.changeSetting(set, 1) // which says why it can't
				return
			}
			d.chooseSetting(set.key, set.choices[i])
		})

	case settingNumber, settingWeight:
		var value string
		less, more, width := true, true, u(62)
		if set.kind == settingWeight {
			weights := d.weightsFor(set.key)
			w := shownWeight(weights, int(e.cfg.number(set.key)))
			i := slices.Index(weights, w)
			less, more = i > 0, i >= 0 && i < len(weights)-1
			value, width = weightName(w), u(96)
		} else {
			v := e.cfg.number(set.key)
			less, more = v > set.min, v < set.max
			value = formatNumber(v)
			switch set.unit {
			case "×":
				value += "×"
			case "":
			default:
				value += " " + set.unit
			}
		}
		box := image.Rect(right-width-2*u(28), cy-u(14), right, cy+u(14))
		d.round(box, 7, c.well)
		d.border(box, 7, c.line, d.hairline())
		side := func(r rect, id iconID, by int, enabled bool) {
			ink := c.faint
			if control(r, func() { d.changeSetting(set, by) }) && enabled {
				d.round(r.Inset(u(2)), 5, c.hover)
			}
			if enabled {
				ink = c.text
			} else {
				ink = fade(ink, 120)
			}
			d.icon(id, (r.Min.X+r.Max.X)/2, cy, 12, ink)
		}
		side(image.Rect(box.Min.X, box.Min.Y, box.Min.X+u(28), box.Max.Y), iconMinus, -1, less)
		side(image.Rect(box.Max.X-u(28), box.Min.Y, box.Max.X, box.Max.Y), iconPlus, 1, more)
		d.textCenter((box.Min.X+box.Max.X)/2, cy, value, textBody, c.text)
		return box.Min.X

	case settingFont:
		name, ink := e.cfg.text(set.key), c.text
		st := textBody
		family := fonts().find(name)
		switch {
		case name == "" && set.key == "font_fallback":
			name, ink = "Automatic", c.dim
		case name == "" && set.key == "font_family" && d.codeFamily != nil:
			name, ink = d.codeFamily.name+" (default)", c.dim
		case name == "" && set.key == "ui_font_family" && d.uiFamily != nil:
			name, ink = d.uiFamily.name+" (default)", c.dim
		case name == "":
			name, ink = "Default", c.dim
		case family == nil:
			ink = readable(c.red, c.well, 4)
		default:
			st.family = family.typeface()
		}
		box := image.Rect(right-u(210), cy-u(14), right, cy+u(14))
		fill := c.well
		if control(box, func() { d.changeSetting(set, 1) }) {
			fill = c.bg
		}
		d.round(box, 7, fill)
		d.border(box, 7, c.line, d.hairline())
		d.text(box.Min.X+u(10), cy, d.fit(name, box.Dx()-u(38), st), st, ink)
		d.icon(iconDown, box.Max.X-u(15), cy, 11, c.dim)
		return box.Min.X
	}
	return right
}

func (d *desktop) segments(right, cy int, labels []string, on int, control func(rect, func()) bool, pick func(int)) int {
	u, c := d.u, &d.c
	total := u(4)
	widths := make([]int, len(labels))
	for i, label := range labels {
		widths[i] = max(u(32), d.width(label, textSmall)+u(20))
		total += widths[i]
	}
	box := image.Rect(right-total, cy-u(14), right, cy+u(14))
	d.round(box, 7, c.well)
	d.border(box, 7, c.line, d.hairline())
	x := box.Min.X + u(2)
	for i, label := range labels {
		r := image.Rect(x, box.Min.Y+u(2), x+widths[i], box.Max.Y-u(2))
		x = r.Max.X
		hot := control(r, func() { pick(i) })
		ink := c.dim
		switch {
		case i == on:
			d.round(r, 5, c.accent)
			ink = c.onAccent
		case hot:
			d.round(r, 5, c.hover)
			ink = c.text
		}
		d.textCenter((r.Min.X+r.Max.X)/2, cy, label, textSmall, ink)
	}
	return box.Min.X
}

func (d *desktop) themeColumns(width int) int {
	return max(1, min(4, (width-d.u(24)+d.u(10))/(d.u(148)+d.u(10))))
}

func (d *desktop) themeGrid(width int) (cols, w, h int) {
	cols = d.themeColumns(width)
	w = (width - d.u(24) - (cols-1)*d.u(10)) / cols
	return cols, w, d.u(106)
}

func (d *desktop) stepTheme(by int) {
	at := max(0, slices.IndexFunc(theme.All, func(t theme.Theme) bool { return t.Name == th.name }))
	d.pickTheme(theme.All[(at+by+len(theme.All))%len(theme.All)].Name)
}

func (d *desktop) paintThemes(e *editor, row settingsRow, r rect, selected bool, control func(rect, func()) bool) {
	u, c := d.u, &d.c
	x, y, right := r.Min.X+u(12), r.Min.Y, r.Max.X-u(12)
	ink := c.text
	locked := e.projectSets("theme")
	if locked {
		ink = c.faint
	}
	d.text(x, y+u(18), row.set.title, textBody, ink)
	d.text(x, y+u(37), row.set.help, textSmall, c.dim)
	switch {
	case locked:
		w := d.chip(0, y+u(22), "Project", false)
		d.chip(right-w, y+u(22), "Project", true)
	case e.userSets("theme"):
		box := image.Rect(right-u(44), y+u(11), right, y+u(33))
		col := c.dim
		if control(box, func() { d.resetRow(row) }) {
			d.round(box, 5, c.key)
			col = c.text
		}
		d.textCenter((box.Min.X+box.Max.X)/2, (box.Min.Y+box.Max.Y)/2, "Reset", textSmall, col)
	case selected:
		d.hints(right-u(84), y+u(22), false, "←→", "choose")
	}

	cols, w, h := d.themeGrid(r.Dx())
	for i, t := range theme.All {
		tx, ty := x+i%cols*(w+u(10)), y+u(54)+i/cols*(h+u(10))
		box := image.Rect(tx, ty, tx+w, ty+h)
		hot := control(box, func() {
			if locked {
				d.settings.failure = "This project's .loom.json sets the theme."
				return
			}
			d.pickTheme(t.Name)
		})
		d.themeSwatch(box, t)
		switch {
		case t.Name == th.name:
			d.border(box, 8, c.accent, u(2))
			k := u(8)
			mx, my := box.Max.X-u(14), box.Max.Y-u(14)
			d.round(image.Rect(mx-k, my-k, mx+k, my+k), 8, c.accent)
			d.icon(iconCheck, mx, my, 11, c.onAccent)
		case hot:
			d.border(box, 8, c.dim, d.hairline())
		default:
			d.border(box, 8, c.strongLine, d.hairline())
		}
	}
}

// themeSwatch is a tiny editor in the colours of t.
func (d *desktop) themeSwatch(r rect, t theme.Theme) {
	u := d.u
	bg := nrgba(terminalColor(t.BG), color.NRGBA{R: 0x1e, G: 0x22, B: 0x2a, A: 255})
	fg := nrgba(terminalColor(t.FG), color.NRGBA{R: 0xc5, G: 0xcb, B: 0xd5, A: 255})
	col := func(v theme.Color) color.NRGBA { return nrgba(terminalColor(v), fg) }
	side := nrgba(terminalColor(t.Panel), blend(bg, black, 0.14))
	d.round(r, 8, bg)
	name := image.Rect(r.Min.X, r.Max.Y-u(28), r.Max.X, r.Max.Y)
	d.round(name, 8, side)
	d.fill(image.Rect(name.Min.X, name.Min.Y, name.Max.X, name.Min.Y+u(8)), side)
	bar := image.Rect(r.Min.X, r.Min.Y, r.Min.X+u(26), name.Min.Y)
	d.round(bar, 8, side)
	d.fill(image.Rect(bar.Min.X+u(10), bar.Min.Y, bar.Max.X, bar.Max.Y), side)
	for i := range 4 {
		y := r.Min.Y + u(float32(14+i*13))
		d.round(image.Rect(bar.Min.X+u(6), y-u(2), bar.Max.X-u(6), y+u(2)), 2, blend(side, fg, 0.3))
	}
	lines := [][]struct {
		w float32
		c theme.Color
	}{
		{{54, t.Comment}},
		{{18, t.Red}, {28, t.Accent}, {14, t.FG}},
		{{12, t.Aqua}, {20, t.Yellow}, {22, t.Orange}},
		{{16, t.Purple}, {26, t.Type}},
	}
	for i, line := range lines {
		x, y := bar.Max.X+u(8), r.Min.Y+u(float32(14+i*14))
		if i >= 2 {
			x += u(10)
		}
		for _, token := range line {
			end := min(r.Max.X-u(6), x+u(token.w))
			d.round(image.Rect(x, y-u(2.5), end, y+u(2.5)), 3, col(token.c))
			x = end + u(5)
		}
	}
	title := d.fit(themeTitle(t.Name), name.Dx()-u(36), textSmall)
	d.text(name.Min.X+u(10), (name.Min.Y+name.Max.Y)/2, title, textSmall, readable(fg, side, 5))
}

// paintCodePreview shows code exactly as the editor would draw it.
func (d *desktop) paintCodePreview(e *editor, r rect) {
	u, c := d.u, &d.c
	box := image.Rect(r.Min.X, r.Min.Y+u(4), r.Max.X, r.Max.Y-u(6))
	d.round(box, 8, c.bg)
	d.border(box, 8, c.line, d.hairline())
	d.clipped(box.Inset(u(4)), func() {
		for i, line := range previewCode {
			y, at := box.Min.Y+u(12)+i*d.cellH, 0
			for _, part := range line {
				fg := th.fg
				if part.kind != "" {
					if hex := semanticColor(part.kind); hex != 0 {
						fg = rgb(hex)
					}
				}
				ink := d.cellFG(fg, c.bg)
				italic := part.kind == "comment" && e.cfg.ItalicComments
				runes := []rune(part.text)
				for j := 0; j < len(runes); {
					end := j + 1
					if runes[j] < 0x80 && d.look.spacing == 0 {
						for end < len(runes) && runes[end] < 0x80 && (d.look.ligatures || !ligatureBreak(byte(runes[end-1]), byte(runes[end]))) {
							end++
						}
					}
					if text := string(runes[j:end]); strings.TrimSpace(text) != "" {
						d.glyph(box.Min.X+u(14)+d.cellX(at+j), y, text, ink, false, italic)
					}
					j = end
				}
				at += len(runes)
			}
		}
	})
}

func (d *desktop) paintFontPicker(e *editor, r rect) {
	s, u, c := d.settings, d.u, &d.c
	p := s.picker
	cy := r.Min.Y + u(30)
	back := image.Rect(r.Min.X+u(12), cy-u(13), r.Min.X+u(78), cy+u(13))
	if d.button(back, func() { s.picker = nil }) {
		d.round(back, 6, c.hover)
	}
	p1 := d.pen(back.Min.X+u(14), cy, 12, c.dim)
	p1.line(10, 4, 6, 8, 10, 12)
	d.text(back.Min.X+u(26), cy, "Back", textBody, c.dim)
	d.text(back.Max.X+u(10), cy, p.title, textStrong, c.text)

	field := image.Rect(r.Min.X+u(14), r.Min.Y+u(54), r.Max.X-u(14), r.Min.Y+u(84))
	d.settingsField(field, p.query, "Search installed fonts", true, func() {})

	list := image.Rect(r.Min.X, field.Max.Y+u(8), r.Max.X, r.Max.Y-u(4))
	choices := d.fontChoices(p)
	rowH := u(38)
	count := max(1, list.Dy()/rowH)
	p.sel = max(0, min(p.sel, len(choices)-1))
	if p.reveal {
		p.top = max(p.sel-count+1, min(p.sel, p.top))
		p.reveal = false
	}
	last := max(0, len(choices)-count)
	p.top = max(0, min(p.top, last))
	wheel := func(_, dy int) { p.top = max(0, min(last, p.top+dy)) }
	d.zone(zone{r: list, wheel: wheel, step: rowH})
	if len(choices) == 0 {
		d.textCenter((list.Min.X+list.Max.X)/2, list.Min.Y+u(40), "No installed font matches", textBody, c.dim)
	}
	current := e.cfg.text(p.key)
	sample := "The quick brown fox"
	if p.mono {
		sample = "if x != 0 { => }"
	}
	for i := p.top; i < len(choices) && i < p.top+count; i++ {
		y, choice := list.Min.Y+(i-p.top)*rowH, choices[i]
		line := image.Rect(list.Min.X+u(10), y+u(1), list.Max.X-u(12), y+rowH-u(1))
		d.zone(zone{r: line, cursor: pointer.CursorPointer, wheel: wheel, step: rowH, click: func() { d.pickFont(choice) }})
		switch {
		case i == p.sel:
			d.round(line, 7, c.selected)
		case d.hot(line):
			d.round(line, 7, c.hover)
		}
		mid, x, right := y+rowH/2, line.Min.X+u(12), line.Max.X-u(12)
		name, isCurrent := "Default", current == ""
		st := textStyle{size: 14}
		if choice != nil {
			name, isCurrent = choice.name, strings.EqualFold(choice.name, current)
			st.family = choice.typeface()
		} else if p.key == "font_fallback" {
			name = "Automatic"
		}
		if isCurrent {
			d.icon(iconCheck, right-u(6), mid, 14, c.accent)
		}
		right -= u(22)
		x += d.text(x, mid, d.fit(name, (right-x)/2, textBody), textBody, c.text)
		if choice != nil && st.family != "" && right-x > u(60) {
			d.textRight(right, mid, d.fit(sample, right-x-u(16), st), st, c.dim)
		}
	}
	d.scrollbar(list, p.top, count, len(choices))
}

func (d *desktop) paintRecording(e *editor, over rect) {
	s, u, c := d.settings, d.u, &d.c
	r := s.recording
	d.round(over, 10, c.scrim)
	d.zone(zone{r: d.geo.window, click: func() { s.recording, s.failure = nil, "" }})
	card := d.centred(u(460), u(208))
	d.card(card)
	d.block(card)
	x, right := card.Min.X+u(20), card.Max.X-u(20)
	d.text(x, card.Min.Y+u(28), d.fit(r.title, right-x, textTitle), textTitle, c.text)
	d.text(x, card.Min.Y+u(51), "Press the new keys, then Enter. "+keyModes[s.mode].title+" mode.", textSmall, c.dim)

	well := image.Rect(x, card.Min.Y+u(68), right, card.Min.Y+u(112))
	d.round(well, 7, c.well)
	d.border(well, 7, blend(c.well, c.accent, 0.55), d.hairline())
	cy := (well.Min.Y + well.Max.Y) / 2
	if len(r.keys) == 0 {
		d.textCenter((well.Min.X+well.Max.X)/2, cy, "Waiting for keys…", textBody, c.faint)
	} else {
		label := keyLabel(strings.Join(r.keys, " "))
		d.keys((well.Min.X+well.Max.X+d.keysWidth(label))/2, cy, label)
	}

	note, ink := "", c.dim
	if len(r.keys) > 0 {
		seq := strings.Join(r.keys, " ")
		if id := e.keys[e.cfg.Profile+"."+keyModes[s.mode].id][seq]; id != "" && id != r.id {
			name := e.commands[id].title
			if name == "" {
				name = humanize(id)
			}
			note, ink = "Takes these keys from "+name, readable(c.yellow, c.raised, 4.5)
		}
	}
	if s.failure != "" {
		note, ink = s.failure, readable(c.red, c.raised, 4.5)
	}
	d.text(x, card.Min.Y+u(131), d.fit(note, right-x, textSmall), textSmall, ink)

	by := card.Max.Y - u(30)
	next := d.textButton(right, by, "Save", len(r.keys) > 0, func() { d.saveRecording() }) - u(8)
	d.textButton(next, by, "Cancel", false, func() { s.recording, s.failure = nil, "" })
	remove := "Remove keys"
	d.textButton(x+d.width(remove, textStrong)+u(22), by, remove, false, d.clearBinding)
}
