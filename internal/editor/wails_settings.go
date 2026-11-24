package editor

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/zhuchops/loom/internal/theme"
)

// The settings window. The rows come from the shared model
// (desktop_settings_model.go); this file applies changes and describes the
// window to the web view.

// webWeights are the weights offered for any font: the web view draws what
// the font has and makes up the rest.
var webWeights = []int{300, 400, 500, 600, 700, 800}

func (b *Bridge) toggleSettings() {
	e := b.m.editor
	if b.settings != nil {
		b.settings = nil
		return
	}
	e.menu, e.panel, e.diff, e.notificationDetail = nil, nil, nil, nil
	e.tree.focused, e.prefix = false, ""
	e.invalidateCompletion()
	s := &settingsView{tools: map[string]string{}}
	if keys, err := compileKeys(defaultConfig(), e.commands); err == nil {
		s.builtin = keys
	}
	b.settings = s
}

// settingsHaveKeys is false while something the settings opened is above them.
func (b *Bridge) settingsHaveKeys() bool {
	e := b.m.editor
	return b.settings != nil && b.dialog == nil && e.menu == nil && e.notificationDetail == nil
}

func (b *Bridge) weightsFor(string) []int { return webWeights }

// changeSetting moves a setting one step, or opens what picks its value.
func (b *Bridge) changeSetting(set setting, by int) {
	s, e := b.settings, b.m.editor
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
		weights := b.weightsFor(set.key)
		i := slices.Index(weights, shownWeight(weights, int(e.cfg.number(set.key))))
		to := max(0, min(len(weights)-1, i+by))
		if to == i {
			return
		}
		value = float64(weights[to])
	case settingTheme:
		b.stepTheme(by)
		return
	case settingFont:
		b.openFontPicker(set)
		return
	}
	s.report(e.setSetting(set.key, value))
}

func (b *Bridge) stepTheme(by int) {
	at := max(0, slices.IndexFunc(theme.All, func(t theme.Theme) bool { return t.Name == th.name }))
	b.pickTheme(theme.All[(at+by+len(theme.All))%len(theme.All)].Name)
}

// pickTheme is setTheme without the notification over the window.
func (b *Bridge) pickTheme(name string) {
	if s := b.settings; s != nil {
		s.report(b.m.editor.setSetting("theme", name))
	}
}

func (b *Bridge) openFontPicker(set setting) {
	p := &fontPicker{key: set.key, mono: set.key == "font_family", reveal: true}
	switch set.key {
	case "font_family":
		p.title = "Code font"
	case "font_fallback":
		p.title = "Fallback font"
	default:
		p.title = "Interface font"
	}
	current := b.m.editor.cfg.text(set.key)
	for i, choice := range b.fontChoices(p) {
		if choice != "" && strings.EqualFold(choice, current) {
			p.sel = i
		}
	}
	b.settings.picker = p
}

// fontChoices are the families the web view found installed; "" stands for
// the default.
func (b *Bridge) fontChoices(p *fontPicker) []string {
	query := strings.ToLower(strings.TrimSpace(p.query))
	var out []string
	if query == "" {
		out = append(out, "")
	}
	for _, f := range b.fonts {
		if strings.Contains(strings.ToLower(f.Name), query) && (f.Mono || !p.mono) {
			out = append(out, f.Name)
		}
	}
	return out
}

// webFont is a font family the page found installed.
type webFont struct {
	Name string `json:"name"`
	Mono bool   `json:"mono"`
}

func (b *Bridge) pickFont(name string) {
	s, e := b.settings, b.m.editor
	if s == nil || s.picker == nil {
		return
	}
	key := s.picker.key
	s.picker = nil
	if name == "" {
		s.report(e.resetSetting(key))
		return
	}
	s.report(e.setSetting(key, name))
}

func (b *Bridge) changeLanguage(name, field string, by int) {
	s, e := b.settings, b.m.editor
	choices := languageChoices(field)
	current := languageValue(e.cfg.Languages[name], field)
	i := max(0, slices.IndexFunc(choices, func(c languageChoice) bool { return c.value == current }))
	s.report(e.setLanguageSetting(name, field, choices[(i+by+len(choices))%len(choices)].value))
}

// activateRow is Enter or a click on a row; arrows pass a direction.
func (b *Bridge) activateRow(row settingsRow, by int, enter bool) {
	s := b.settings
	switch row.kind {
	case rowSetting:
		if row.set.kind == settingTheme && enter || row.set.kind == settingFont && !enter {
			return
		}
		b.changeSetting(row.set, by)
	case rowLanguage:
		switch {
		case enter && s.language == row.id, !enter && by < 0:
			s.language = ""
		default:
			s.language = row.id
		}
	case rowLanguageValue:
		b.changeLanguage(row.id, row.field, by)
	case rowModes:
		s.mode = (s.mode + by + len(keyModes)) % len(keyModes)
	case rowBinding:
		if enter {
			s.failure = ""
			s.recording = &keyRecording{id: row.id, title: row.label}
		}
	}
}

func (b *Bridge) resetRow(row settingsRow) {
	s, e := b.settings, b.m.editor
	switch row.kind {
	case rowSetting:
		s.report(e.resetSetting(row.set.key))
	case rowLanguageValue:
		s.report(e.setLanguageSetting(row.id, row.field, nil))
	case rowBinding:
		s.report(e.resetKey(keyModes[s.mode].id, row.id))
	}
}

func (b *Bridge) saveRecording() {
	s, e := b.settings, b.m.editor
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

func (b *Bridge) clearBinding() {
	s, e := b.settings, b.m.editor
	if r := s.recording; r != nil {
		s.report(e.bindKey(keyModes[s.mode].id, r.id, ""))
		s.recording = nil
	}
}

func (b *Bridge) settingsKey(in keyInput) {
	s, e := b.settings, b.m.editor
	if r := s.recording; r != nil {
		switch {
		case in.key == "":
		case in.key == "esc":
			s.recording, s.failure = nil, ""
		case in.key == "enter" && len(r.keys) > 0:
			b.saveRecording()
		case in.key == "backspace" && len(r.keys) > 0:
			r.keys = r.keys[:len(r.keys)-1]
		case len(r.keys) < 4:
			r.keys = append(r.keys, in.key)
			s.failure = ""
		}
		return
	}
	if p := s.picker; p != nil {
		choices := b.fontChoices(p)
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
				b.pickFont(choices[p.sel])
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
	move := func(by int) { s.sel, s.reveal = max(0, min(len(picks)-1, s.sel+by)), true }
	switch in.key {
	case "esc":
		if s.query != "" {
			s.focusSearch()
		} else {
			b.settings = nil
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
			b.activateRow(rows[picks[s.sel]], by, in.key == "enter")
			s.reveal = true
		}
	case "delete":
		if len(picks) > 0 {
			b.resetRow(rows[picks[s.sel]])
		}
	case "ctrl+q":
		b.requestQuit()
	default:
		if text, ok := editField(s.query, in); ok {
			s.query, s.sel, s.scroll = text, 0, 0
		}
	}
}

// settingsState describes the settings window.
func (b *Bridge) settingsState() map[string]any {
	s, e := b.settings, b.m.editor
	c := &b.frame.colors
	pages := make([]string, len(settingPages))
	for i, p := range settingPages {
		pages[i] = p.title
	}
	out := map[string]any{"page": s.page, "pages": pages, "query": s.query, "searching": s.picker == nil && s.recording == nil}
	if s.failure != "" && s.recording == nil {
		out["failure"] = strings.Join(strings.Fields(s.failure), " ")
	}
	var links [][]string
	if len(e.cfg.fromProject) > 0 {
		links = append(links, []string{"Project file", "project"})
	}
	if addedPath(e.userConfigPath()) != "" {
		links = append(links, []string{"settings.json", "user"})
	}
	links = append(links, []string{"config.json", "config"})
	out["links"] = links

	if p := s.picker; p != nil {
		choices := b.fontChoices(p)
		p.sel = max(0, min(p.sel, len(choices)-1))
		current := e.cfg.text(p.key)
		list := make([]any, len(choices))
		for i, name := range choices {
			label := name
			if name == "" {
				label = "Default"
				if p.key == "font_fallback" {
					label = "Automatic"
				}
			}
			list[i] = []any{label, name, name == "" && current == "" || name != "" && strings.EqualFold(name, current)}
		}
		sample := "The quick brown fox"
		if p.mono {
			sample = "if x != 0 { => }"
		}
		out["picker"] = map[string]any{"title": p.title, "query": p.query, "sel": p.sel, "choices": list, "sample": sample, "reveal": p.reveal}
		p.reveal = false
		out["hints"] = []string{"↑↓", "move", "↵", "use", "esc", "back"}
		return out
	}
	out["hints"] = []string{"↑↓", "move", "←→", "change", "Tab", "page", "esc", "close"}

	rows := s.rows(e)
	picks := selectableRows(rows)
	s.sel = max(0, min(s.sel, len(picks)-1))
	selected := -1
	if len(picks) > 0 {
		selected = picks[s.sel]
	}
	list := make([]any, 0, len(rows))
	for i, row := range rows {
		r := b.settingsRowState(row)
		r["i"] = i
		if i == selected {
			r["sel"] = true
		}
		list = append(list, r)
	}
	out["rows"] = list
	if s.reveal {
		out["reveal"] = selected
		s.reveal = false
	}
	if r := s.recording; r != nil {
		rec := map[string]any{"title": r.title, "mode": keyModes[s.mode].title, "ready": len(r.keys) > 0}
		if len(r.keys) > 0 {
			seq := strings.Join(r.keys, " ")
			rec["keys"] = keyLabel(seq)
			if id := e.keys[e.cfg.Profile+"."+keyModes[s.mode].id][seq]; id != "" && id != r.id {
				name := e.commands[id].title
				if name == "" {
					name = humanize(id)
				}
				rec["note"] = []string{"Takes these keys from " + name, hexColor(readable(c.yellow, c.raised, 4.5))}
			}
		}
		if s.failure != "" {
			rec["note"] = []string{s.failure, hexColor(readable(c.red, c.raised, 4.5))}
		}
		out["recording"] = rec
	}
	return out
}

func (b *Bridge) settingsRowState(row settingsRow) map[string]any {
	s, e := b.settings, b.m.editor
	c := &b.frame.colors
	switch row.kind {
	case rowHeading:
		return map[string]any{"k": "heading", "label": strings.ToUpper(row.label)}
	case rowNote:
		return map[string]any{"k": "note", "label": row.label}
	case rowPreview:
		var lines []any
		for _, line := range previewCode {
			var parts []any
			for _, part := range line {
				fg := th.fg
				if part.kind != "" {
					if hex := semanticColor(part.kind); hex != 0 {
						fg = rgb(hex)
					}
				}
				parts = append(parts, []any{part.text, hexColor(b.cellFG(fg, c.bg)), part.kind == "comment" && e.cfg.ItalicComments})
			}
			lines = append(lines, parts)
		}
		return map[string]any{"k": "preview", "lines": lines}
	case rowLanguageTool:
		return map[string]any{"k": "tool", "label": row.label, "detail": row.detail, "ok": strings.HasSuffix(row.detail, "installed")}
	case rowLanguage:
		lang := e.cfg.Languages[row.id]
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
		summary := strings.Join(changed, ", ")
		if summary == "" {
			for i, ext := range lang.Extensions {
				if i == 4 {
					break
				}
				summary += "." + ext + " "
			}
		}
		return map[string]any{"k": "language", "name": languageName(row.id), "open": s.language == row.id, "summary": strings.TrimSpace(summary), "changed": len(changed) > 0}
	case rowLanguageValue:
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
		return map[string]any{"k": "langValue", "label": row.label, "labels": labels, "on": on}
	case rowModes:
		labels := make([]string, len(keyModes))
		for i, m := range keyModes {
			labels[i] = m.title
		}
		return map[string]any{"k": "modes", "label": row.label, "labels": labels, "on": s.mode}
	case rowBinding:
		var parts []string
		for _, seq := range keysFor(e.keys[e.cfg.Profile+"."+keyModes[s.mode].id], row.id) {
			if len(parts) < 3 {
				parts = append(parts, keyLabel(seq))
			}
		}
		return map[string]any{"k": "binding", "label": row.label, "keys": strings.Join(parts, "  "), "changed": s.keysChanged(e, row.id)}
	}

	set := row.set
	r := map[string]any{"k": "setting", "title": set.title, "help": set.help, "locked": e.projectSets(set.key), "live": switchOn(e, set), "user": e.userSets(set.key)}
	switch set.kind {
	case settingSwitch:
		r["control"], r["on"] = "switch", e.cfg.on(set.key) && switchOn(e, set)
	case settingChoice:
		labels := make([]string, len(set.choices))
		for i, v := range set.choices {
			labels[i] = humanize(v)
		}
		r["control"], r["labels"], r["on"] = "choice", labels, slices.Index(set.choices, e.cfg.text(set.key))
	case settingNumber:
		v := e.cfg.number(set.key)
		value := formatNumber(v)
		switch set.unit {
		case "×":
			value += "×"
		case "":
		default:
			value += " " + set.unit
		}
		r["control"], r["value"], r["less"], r["more"] = "number", value, v > set.min, v < set.max
	case settingWeight:
		weights := b.weightsFor(set.key)
		w := shownWeight(weights, int(e.cfg.number(set.key)))
		i := slices.Index(weights, w)
		r["control"], r["value"], r["less"], r["more"], r["wide"] = "number", weightName(w), i > 0, i >= 0 && i < len(weights)-1, true
	case settingFont:
		name := e.cfg.text(set.key)
		r["control"], r["family"] = "font", name
		if name == "" {
			r["family"] = ""
			switch set.key {
			case "font_fallback":
				name = "Automatic"
			default:
				name = "Default"
			}
			r["default"] = true
		} else if !slices.ContainsFunc(b.fonts, func(f webFont) bool { return strings.EqualFold(f.Name, name) }) && len(b.fonts) > 0 {
			r["missing"] = true
			r["help"] = name + " isn't installed, so the default is used"
		}
		r["value"] = name
	case settingTheme:
		r["k"] = "theme"
		var themes []any
		for _, t := range theme.All {
			themes = append(themes, b.themeSwatch(t))
		}
		r["themes"] = themes
	}
	return r
}

// themeSwatch is the colours a theme card is drawn in.
func (b *Bridge) themeSwatch(t theme.Theme) map[string]any {
	bg := nrgba(terminalColor(t.BG), b.frame.colors.bg)
	fg := nrgba(terminalColor(t.FG), b.frame.colors.text)
	col := func(v theme.Color) string { return hexColor(nrgba(terminalColor(v), fg)) }
	side := nrgba(terminalColor(t.Panel), blend(bg, black, 0.14))
	return map[string]any{
		"name": t.Name, "title": themeTitle(t.Name), "current": t.Name == th.name,
		"bg": hexColor(bg), "side": hexColor(side), "bar": hexColor(blend(side, fg, 0.3)), "ink": hexColor(readable(fg, side, 5)),
		"lines": [][]any{
			{[]any{54, col(t.Comment)}},
			{[]any{18, col(t.Red)}, []any{28, col(t.Accent)}, []any{14, hexColor(fg)}},
			{[]any{12, col(t.Aqua)}, []any{20, col(t.Yellow)}, []any{22, col(t.Orange)}},
			{[]any{16, col(t.Purple)}, []any{26, col(t.Type)}},
		},
	}
}

// settingsAct handles clicks in the settings window.
func (b *Bridge) settingsAct(name string, i int, arg string) bool {
	s, e := b.settings, b.m.editor
	if !strings.HasPrefix(name, "settings") {
		return false
	}
	if name == "settings-fonts" {
		var list []webFont
		if json.Unmarshal([]byte(arg), &list) == nil {
			b.fonts = list
		}
		return true
	}
	if s == nil {
		return true
	}
	rows := s.rows(e)
	row := func() (settingsRow, bool) {
		if i < 0 || i >= len(rows) {
			return settingsRow{}, false
		}
		if k := slices.Index(selectableRows(rows), i); k >= 0 {
			s.sel = k
		}
		s.failure = ""
		return rows[i], true
	}
	switch name {
	case "settings-close":
		b.settings = nil
	case "settings-page":
		if i >= 0 && i < len(settingPages) {
			s.show(i)
		}
	case "settings-search-clear":
		s.focusSearch()
	case "settings-row":
		if r, ok := row(); ok {
			switch {
			case r.kind == rowLanguage, r.kind == rowBinding,
				r.kind == rowSetting && (r.set.kind == settingSwitch || r.set.kind == settingFont):
				b.activateRow(r, 1, true)
			}
		}
	case "settings-step":
		if r, ok := row(); ok {
			by := 1
			if arg == "-1" {
				by = -1
			}
			b.changeSetting(r.set, by)
		}
	case "settings-choose":
		r, ok := row()
		k, err := strconv.Atoi(arg)
		if !ok || err != nil {
			return true
		}
		switch r.kind {
		case rowSetting:
			if e.projectSets(r.set.key) {
				b.changeSetting(r.set, 1) // which says why it can't
			} else if k >= 0 && k < len(r.set.choices) {
				s.report(e.setSetting(r.set.key, r.set.choices[k]))
			}
		case rowLanguageValue:
			if choices := languageChoices(r.field); k >= 0 && k < len(choices) {
				s.report(e.setLanguageSetting(r.id, r.field, choices[k].value))
			}
		case rowModes:
			if k >= 0 && k < len(keyModes) {
				s.mode = k
			}
		}
	case "settings-reset":
		if r, ok := row(); ok {
			b.resetRow(r)
		}
	case "settings-theme":
		if e.projectSets("theme") {
			s.failure = "This project's .loom.json sets the theme."
		} else {
			b.pickTheme(arg)
		}
	case "settings-font":
		b.pickFont(arg)
	case "settings-font-back":
		s.picker = nil
	case "settings-record":
		switch arg {
		case "save":
			b.saveRecording()
		case "clear":
			b.clearBinding()
		default:
			s.recording, s.failure = nil, ""
		}
	case "settings-link":
		b.settings = nil
		switch arg {
		case "project":
			e.open(filepath.Join(e.root, ".loom.json"))
		case "user":
			e.open(addedPath(e.userConfigPath()))
		default:
			e.openConfigFile()
		}
	}
	return true
}
