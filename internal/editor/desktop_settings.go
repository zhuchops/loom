//go:build desktop

package editor

import (
	"slices"
	"strings"
)

func (d *desktop) toggleSettings() {
	e := d.m.editor
	if d.settings != nil {
		d.settings = nil
		return
	}
	e.menu, e.panel, e.diff, e.notificationDetail = nil, nil, nil, nil
	e.tree.focused, e.prefix = false, ""
	e.invalidateCompletion()
	rescanFonts()
	s := &settingsView{tools: map[string]string{}}
	if keys, err := compileKeys(defaultConfig(), e.commands); err == nil {
		s.builtin = keys
	}
	d.settings = s
	d.toast.text = "" // older news would cover the window
}

// settingsHaveKeys is false while something the settings opened is above them.
func (d *desktop) settingsHaveKeys() bool {
	e := d.m.editor
	return d.settings != nil && d.dialog == nil && e.menu == nil && e.notificationDetail == nil
}

// weightsFor lists the weights the font behind a weight setting really has.
func (d *desktop) weightsFor(key string) []int {
	family := d.codeFamily
	if key == "ui_font_weight" {
		family = d.uiFamily
	}
	if family == nil {
		return nil
	}
	return family.weights()
}

// changeSetting moves a setting one step, or opens what picks its value.
func (d *desktop) changeSetting(set setting, by int) {
	s, e := d.settings, d.m.editor
	if s == nil || !switchOn(e, set) {
		return
	}
	if e.projectSets(set.key) {
		s.failure = "This project's .loom.json sets " + strings.ToLower(set.title) + "."
		return
	}
	var value any
	switch set.kind {
	case settingSwitch:
		value = !e.cfg.on(set.key)
	case settingChoice:
		i := slices.Index(set.choices, e.cfg.text(set.key))
		value = set.choices[(i+by+len(set.choices))%len(set.choices)]
	case settingNumber:
		at := e.cfg.number(set.key)
		to := max(set.min, min(set.max, at+float64(by)*set.step))
		if to == at {
			return
		}
		value = to
	case settingWeight:
		weights := d.weightsFor(set.key)
		if len(weights) < 2 {
			return
		}
		i := slices.Index(weights, shownWeight(weights, int(e.cfg.number(set.key))))
		to := max(0, min(len(weights)-1, i+by))
		if to == i {
			return
		}
		value = float64(weights[to])
	case settingTheme:
		d.stepTheme(by)
		return
	case settingFont:
		d.openFontPicker(set)
		return
	}
	s.report(e.setSetting(set.key, value))
}

func (d *desktop) chooseSetting(key string, value any) {
	if s := d.settings; s != nil {
		s.report(d.m.editor.setSetting(key, value))
	}
}

// pickTheme is setTheme without the notification over the window.
func (d *desktop) pickTheme(name string) {
	s, e := d.settings, d.m.editor
	if s != nil {
		s.report(e.setSetting("theme", name))
	}
}

func (d *desktop) openFontPicker(set setting) {
	p := &fontPicker{key: set.key, mono: set.key == "font_family", reveal: true}
	switch set.key {
	case "font_family":
		p.title = "Code font"
	case "font_fallback":
		p.title = "Fallback font"
	default:
		p.title = "Interface font"
	}
	current := d.m.editor.cfg.text(set.key)
	for i, choice := range d.fontChoices(p) {
		if choice != nil && strings.EqualFold(choice.name, current) {
			p.sel = i
		}
	}
	d.settings.picker = p
}

// fontChoices are the families a picker shows. nil stands for the default.
func (d *desktop) fontChoices(p *fontPicker) []*fontFamily {
	query := strings.ToLower(strings.TrimSpace(p.query))
	var out []*fontFamily
	if query == "" {
		out = append(out, nil)
	}
	for _, family := range fonts().families {
		if !strings.Contains(strings.ToLower(family.name), query) {
			continue
		}
		if t := family.traits(); t.usable && (t.mono || !p.mono) && len(family.weights()) > 0 {
			out = append(out, family)
		}
	}
	return out
}

func (d *desktop) pickFont(choice *fontFamily) {
	s, e := d.settings, d.m.editor
	if s == nil || s.picker == nil {
		return
	}
	key := s.picker.key
	s.picker = nil
	if choice == nil {
		s.report(e.resetSetting(key))
		return
	}
	s.report(e.setSetting(key, choice.name))
}

func (d *desktop) changeLanguage(name, field string, by int) {
	s, e := d.settings, d.m.editor
	choices := languageChoices(field)
	current := languageValue(e.cfg.Languages[name], field)
	i := max(0, slices.IndexFunc(choices, func(c languageChoice) bool { return c.value == current }))
	s.report(e.setLanguageSetting(name, field, choices[(i+by+len(choices))%len(choices)].value))
}

// activate is Enter or a click on a row; arrows pass a direction.
func (d *desktop) activateRow(row settingsRow, by int, enter bool) {
	s := d.settings
	switch row.kind {
	case rowSetting:
		if row.set.kind == settingTheme && enter || row.set.kind == settingFont && !enter {
			return
		}
		d.changeSetting(row.set, by)
	case rowLanguage:
		switch {
		case enter && s.language == row.id, !enter && by < 0:
			s.language = ""
		default:
			s.language = row.id
		}
	case rowLanguageValue:
		d.changeLanguage(row.id, row.field, by)
	case rowModes:
		s.mode = (s.mode + by + len(keyModes)) % len(keyModes)
	case rowBinding:
		if enter {
			s.failure = ""
			s.recording = &keyRecording{id: row.id, title: row.label}
		}
	}
}

func (d *desktop) resetRow(row settingsRow) {
	s, e := d.settings, d.m.editor
	switch row.kind {
	case rowSetting:
		s.report(e.resetSetting(row.set.key))
	case rowLanguageValue:
		s.report(e.setLanguageSetting(row.id, row.field, nil))
	case rowBinding:
		s.report(e.resetKey(keyModes[s.mode].id, row.id))
	}
}

func (d *desktop) saveRecording() {
	s, e := d.settings, d.m.editor
	r := s.recording
	if r == nil || len(r.keys) == 0 {
		return
	}
	if err := e.bindKey(keyModes[s.mode].id, r.id, strings.Join(r.keys, " ")); err != nil {
		s.report(err)
		r.keys = nil // try another
		return
	}
	s.recording, s.failure = nil, ""
}

func (d *desktop) clearBinding() {
	s, e := d.settings, d.m.editor
	if r := s.recording; r != nil {
		s.report(e.bindKey(keyModes[s.mode].id, r.id, ""))
		s.recording = nil
	}
}

func (d *desktop) settingsKey(in keyInput) {
	s, e := d.settings, d.m.editor

	if r := s.recording; r != nil {
		switch {
		case in.key == "":
		case in.key == "esc":
			s.recording, s.failure = nil, ""
		case in.key == "enter" && len(r.keys) > 0:
			d.saveRecording()
		case in.key == "backspace" && len(r.keys) > 0:
			r.keys = r.keys[:len(r.keys)-1]
		case len(r.keys) < 4:
			r.keys = append(r.keys, in.key)
			s.failure = ""
		}
		return
	}

	if p := s.picker; p != nil {
		choices := d.fontChoices(p)
		switch in.key {
		case "esc":
			s.picker = nil
		case "up", "ctrl+p":
			p.sel, p.reveal = max(0, p.sel-1), true
		case "down", "ctrl+n":
			p.sel, p.reveal = max(0, min(len(choices)-1, p.sel+1)), true
		case "pageup":
			p.sel, p.reveal = max(0, p.sel-8), true
		case "pagedown":
			p.sel, p.reveal = max(0, min(len(choices)-1, p.sel+8)), true
		case "enter":
			if p.sel < len(choices) {
				d.pickFont(choices[p.sel])
			}
		default:
			if text, ok := editField(p.query, in); ok {
				p.query, p.sel, p.top = text, 0, 0
			}
		}
		return
	}

	rows := s.rows(e)
	picks := selectableRows(rows)
	s.sel = max(0, min(s.sel, len(picks)-1))
	move := func(by int) {
		s.sel, s.reveal = max(0, min(len(picks)-1, s.sel+by)), true
	}
	switch in.key {
	case "esc":
		if s.query != "" {
			s.focusSearch()
		} else {
			d.settings = nil
		}
	case "tab":
		s.show((s.page + 1) % len(settingPages))
	case "shift+tab":
		s.show((s.page + len(settingPages) - 1) % len(settingPages))
	case "up", "ctrl+p":
		move(-1)
	case "down", "ctrl+n":
		move(1)
	case "pageup":
		move(-8)
	case "pagedown":
		move(8)
	case "enter", "left", "right":
		if len(picks) > 0 {
			by := 1
			if in.key == "left" {
				by = -1
			}
			d.activateRow(rows[picks[s.sel]], by, in.key == "enter")
			s.reveal = true
		}
	case "delete":
		if len(picks) > 0 {
			d.resetRow(rows[picks[s.sel]])
		}
	case "ctrl+q":
		d.requestQuit()
	default:
		if text, ok := editField(s.query, in); ok {
			s.query, s.sel, s.scroll = text, 0, 0
		}
	}
}
