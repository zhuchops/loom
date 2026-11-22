//go:build desktop

package editor

import (
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

// settingsView is the state of the settings window. desktop_settings_paint.go
// draws it.
type settingsView struct {
	page  int
	query string

	sel    int // among the rows that can be selected
	scroll int // pixels
	reveal bool

	picker *fontPicker

	language  string // the language that is unfolded
	tools     map[string]string
	mode      int // index into keyModes
	recording *keyRecording
	builtin   map[string]map[string]string // keys before the user's changes

	failure string
}

type fontPicker struct {
	key, title, query string
	mono              bool
	sel, top          int
	reveal            bool
}

type keyRecording struct {
	id, title string
	keys      []string
}

var keyModes = []struct{ id, title string }{{"normal", "Normal"}, {"select", "Visual"}, {"insert", "Insert"}}

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

func (s *settingsView) show(page int) {
	s.page, s.query = page, ""
	s.sel, s.scroll, s.failure = 0, 0, ""
	s.picker, s.recording = nil, nil
}

func (s *settingsView) focusSearch() {
	s.picker, s.recording = nil, nil
	s.query, s.sel, s.scroll = "", 0, 0
}

func (s *settingsView) report(err error) {
	s.failure = ""
	if err != nil {
		s.failure = err.Error()
	}
}

type rowKind uint8

const (
	rowHeading rowKind = iota
	rowSetting
	rowPreview
	rowLanguage
	rowLanguageValue
	rowLanguageTool
	rowModes
	rowBinding
	rowNote
)

type settingsRow struct {
	kind   rowKind
	label  string
	set    setting
	id     string // the language or the command
	field  string // of the language
	detail string
}

func (r settingsRow) selectable() bool {
	switch r.kind {
	case rowSetting, rowLanguage, rowLanguageValue, rowModes, rowBinding:
		return true
	}
	return false
}

func selectableRows(rows []settingsRow) []int {
	var out []int
	for i, r := range rows {
		if r.selectable() {
			out = append(out, i)
		}
	}
	return out
}

// bindableCommands are the commands with a name to show.
func bindableCommands(e *editor) []string {
	var ids []string
	for id, c := range e.commands {
		if c.title != "" {
			ids = append(ids, id)
		}
	}
	slices.SortFunc(ids, func(a, b string) int {
		if c := strings.Compare(e.commands[a].title, e.commands[b].title); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	return ids
}

// rows lists what the current page or search shows.
func (s *settingsView) rows(e *editor) []settingsRow {
	var rows []settingsRow
	query := strings.ToLower(strings.TrimSpace(s.query))
	matches := func(parts ...string) bool {
		text := strings.ToLower(strings.Join(parts, " "))
		for _, word := range strings.Fields(query) {
			if !strings.Contains(text, word) {
				return false
			}
		}
		return true
	}
	heading := func(label string) { rows = append(rows, settingsRow{kind: rowHeading, label: label}) }
	language := func(name string) {
		rows = append(rows, settingsRow{kind: rowLanguage, id: name})
		if s.language != name {
			return
		}
		for _, field := range []string{"tab_size", "indent_style", "format_on_save"} {
			rows = append(rows, settingsRow{kind: rowLanguageValue, id: name, field: field, label: settingByKey[field].title})
		}
		lang := e.cfg.Languages[name]
		rows = append(rows,
			settingsRow{kind: rowLanguageTool, label: "Language server", detail: s.tool(lang.Server)},
			settingsRow{kind: rowLanguageTool, label: "Formatter", detail: s.tool(lang.Format)})
	}

	if query != "" {
		for _, page := range settingPages {
			first := true
			for _, set := range settingList {
				if set.page == page.id && matches(page.title, set.group, set.title, set.help, set.key, strings.Join(set.choices, " ")) {
					if first {
						heading(page.title)
						first = false
					}
					if set.group != "" && (set.title == "Font" || set.title == "Size" || set.title == "Weight") {
						set.title = set.group + " " + strings.ToLower(set.title) // the group heading isn't shown here
					}
					rows = append(rows, settingsRow{kind: rowSetting, set: set})
				}
			}
		}
		first := true
		for _, name := range sortedKeys(e.cfg.Languages) {
			if matches(languageName(name), name) {
				if first {
					heading("Languages")
					first = false
				}
				language(name)
			}
		}
		first = true
		bound := e.keys[e.cfg.Profile+"."+keyModes[s.mode].id]
		for _, id := range bindableCommands(e) {
			title := e.commands[id].title
			var labels []string
			for _, seq := range keysFor(bound, id) {
				labels = append(labels, keyLabel(seq), seq)
			}
			if matches(title, id, strings.Join(labels, " ")) {
				if first {
					heading("Keyboard · " + keyModes[s.mode].title)
					first = false
				}
				rows = append(rows, settingsRow{kind: rowBinding, id: id, label: title})
			}
		}
		if len(rows) == 0 {
			rows = append(rows, settingsRow{kind: rowNote, label: "Nothing matches “" + s.query + "”"})
		}
		return rows
	}

	switch page := settingPages[s.page].id; page {
	case "languages":
		rows = append(rows, settingsRow{kind: rowNote, label: "A language uses the Editor and Files settings unless it has its own."})
		for _, name := range sortedKeys(e.cfg.Languages) {
			language(name)
		}
	case "keyboard":
		if runtime.GOOS == "darwin" {
			rows = append(rows, settingsRow{kind: rowNote, label: "Click a command to change its keys. The ⌘ shortcuts belong to the menu bar and stay as they are."})
		}
		rows = append(rows, settingsRow{kind: rowModes, label: "Mode"})
		for _, id := range bindableCommands(e) {
			rows = append(rows, settingsRow{kind: rowBinding, id: id, label: e.commands[id].title})
		}
	default:
		group := ""
		if page == "fonts" {
			rows = append(rows, settingsRow{kind: rowPreview})
		}
		for _, set := range settingList {
			if set.page != page {
				continue
			}
			if set.group != group {
				group = set.group
				heading(group)
			}
			rows = append(rows, settingsRow{kind: rowSetting, set: set})
		}
	}
	return rows
}

// tool says what a language would run and whether it is installed.
func (s *settingsView) tool(argv []string) string {
	if len(argv) == 0 {
		return "None"
	}
	status, ok := s.tools[argv[0]]
	if !ok {
		status = "not found"
		if _, err := exec.LookPath(argv[0]); err == nil {
			status = "installed"
		}
		s.tools[argv[0]] = status
	}
	return argv[0] + " · " + status
}

// switchOn reports whether the setting another one needs is on.
func switchOn(e *editor, set setting) bool { return set.needs == "" || e.cfg.on(set.needs) }

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

// shownWeight is the available weight that a saved one ends up as.
func shownWeight(weights []int, want int) int {
	best := want
	for i, w := range weights {
		if i == 0 || absInt(w-want) < absInt(best-want) || absInt(w-want) == absInt(best-want) && want >= 600 {
			best = w
		}
	}
	return best
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

type languageChoice struct {
	label string
	value any
}

func languageChoices(field string) []languageChoice {
	out := []languageChoice{{"Default", nil}}
	switch field {
	case "tab_size":
		for _, n := range []int{2, 3, 4, 8} {
			out = append(out, languageChoice{strconv.Itoa(n), n})
		}
	case "indent_style":
		for _, style := range settingByKey[field].choices {
			out = append(out, languageChoice{humanize(style), style})
		}
	default:
		out = append(out, languageChoice{"On", true}, languageChoice{"Off", false})
	}
	return out
}

func languageValue(lang languageConfig, field string) any {
	switch {
	case field == "tab_size" && lang.TabSize > 0:
		return lang.TabSize
	case field == "indent_style" && lang.IndentStyle != "":
		return lang.IndentStyle
	case field == "format_on_save" && lang.FormatOnSave != nil:
		return *lang.FormatOnSave
	}
	return nil
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

// keysChanged reports whether a command's keys differ from the built-in ones.
func (s *settingsView) keysChanged(e *editor, id string) bool {
	if s.builtin == nil {
		return false
	}
	scope := e.cfg.Profile + "." + keyModes[s.mode].id
	return !slices.Equal(keysFor(e.keys[scope], id), keysFor(s.builtin[scope], id))
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

// editField applies typing to a one-line search field.
func editField(text string, in keyInput) (string, bool) {
	switch {
	case in.paste != "":
		return text + strings.Join(strings.Fields(in.paste), " "), true
	case in.key == "backspace":
		if r := []rune(text); len(r) > 0 {
			return string(r[:len(r)-1]), true
		}
	case in.key == "ctrl+u":
		return "", true
	case in.key == "space":
		return text + " ", true
	default:
		if r, ok := inputRune(in.key); ok {
			return text + string(r), true
		}
	}
	return text, false
}
