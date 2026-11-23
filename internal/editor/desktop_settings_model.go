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

// keysChanged reports whether a command's keys differ from the built-in ones.
func (s *settingsView) keysChanged(e *editor, id string) bool {
	if s.builtin == nil {
		return false
	}
	scope := e.cfg.Profile + "." + keyModes[s.mode].id
	return !slices.Equal(keysFor(e.keys[scope], id), keysFor(s.builtin[scope], id))
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

var themeTitles = map[string]string{"nvchad": "NvChad", "tokyo-night": "Tokyo Night", "one-dark": "One Dark", "solarized-dark": "Solarized Dark", "solarized-light": "Solarized Light"}

func themeTitle(name string) string {
	if title, ok := themeTitles[name]; ok {
		return title
	}
	return humanize(name)
}

var weightNames = map[int]string{100: "Thin", 200: "Extra Light", 300: "Light", 400: "Regular", 500: "Medium", 600: "Semibold", 700: "Bold", 800: "Extra Bold", 900: "Black"}

func weightName(w int) string {
	if name, ok := weightNames[w]; ok {
		return name
	}
	return strconv.Itoa(w)
}

// The pieces of the code sample on the Fonts page.
var previewCode = [][]struct{ kind, text string }{
	{{"comment", "// Sum adds up the even numbers"}},
	{{"keyword", "func"}, {"", " "}, {"function", "Sum"}, {"", "(values []"}, {"type", "int"}, {"", ") (total "}, {"type", "int"}, {"", ") {"}},
	{{"", "    "}, {"keyword", "if"}, {"", " len(values) >= "}, {"number", "2"}, {"", " && total != "}, {"number", "0"}, {"", " { } "}},
	{{"", "    "}, {"string", `"0O 1lI -> => Привет"`}},
}
