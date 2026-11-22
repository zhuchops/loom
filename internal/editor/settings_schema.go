package editor

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// options are the settings that hold a single value. They sit at the top
// level of the config files; settingList describes each of them.
type options struct {
	Theme string `json:"theme"`
	// Animations eases pane resizes and splits, the cursor and scrolling.
	Animations bool `json:"animations"`

	FontFamily     string  `json:"font_family"`
	FontFallback   string  `json:"font_fallback"`
	FontSize       float64 `json:"font_size"`
	FontWeight     int     `json:"font_weight"`
	FontBoldWeight int     `json:"font_bold_weight"`
	FontLigatures  bool    `json:"font_ligatures"`
	LineHeight     float64 `json:"line_height"`
	LetterSpacing  float64 `json:"letter_spacing"`
	ItalicComments bool    `json:"italic_comments"`
	UIFontFamily   string  `json:"ui_font_family"`
	UIFontSize     float64 `json:"ui_font_size"`
	UIFontWeight   int     `json:"ui_font_weight"`

	LineNumbers     bool `json:"line_numbers"`
	RelativeNumbers bool `json:"relative_numbers"`
	HighlightLine   bool `json:"highlight_line"`
	// IndentGuides draws vertical guides in leading whitespace.
	IndentGuides bool `json:"indent_guides"`
	// TabSize is the display width of a tab and the width of one indent level
	// made of spaces. IndentStyle is "auto" (tabs in Go and Makefiles, spaces
	// elsewhere), "tabs" or "spaces".
	TabSize     int    `json:"tab_size"`
	IndentStyle string `json:"indent_style"`
	// Scrolloff keeps this many lines visible above and below the cursor.
	Scrolloff int `json:"scrolloff"`
	// Breadcrumbs names the scopes around the cursor in the terminal's status
	// line. The key is from when it also pinned those lines to the pane.
	Breadcrumbs  bool `json:"sticky_scroll"`
	SmoothScroll bool `json:"smooth_scroll"`
	AutoComplete bool `json:"auto_complete"`
	AutoPairs    bool `json:"auto_pairs"`
	HoverDocs    bool `json:"hover_docs"`

	CursorNormal string `json:"cursor_normal"`
	CursorInsert string `json:"cursor_insert"`
	CursorBlink  bool   `json:"cursor_blink"`
	CursorGlide  bool   `json:"cursor_glide"`

	PathBar   bool `json:"path_bar"`
	StatusBar bool `json:"status_bar"`
	Scrollbar bool `json:"scrollbar"`

	FormatOnSave    bool `json:"format_on_save"`
	OrganizeImports bool `json:"organize_imports"`
	RestoreSession  bool `json:"restore_session"`
	ReopenProject   bool `json:"reopen_project"`
}

func defaultOptions() options {
	return options{
		Animations: true,

		FontSize: 13, FontWeight: 400, FontBoldWeight: 700, FontLigatures: true, LineHeight: 1.6,
		UIFontSize: 13, UIFontWeight: 500,

		LineNumbers: true, HighlightLine: true, IndentGuides: true, TabSize: 4, IndentStyle: "auto",
		Scrolloff: 5, Breadcrumbs: true, SmoothScroll: true, AutoComplete: true, AutoPairs: true, HoverDocs: true,

		CursorNormal: "block", CursorInsert: "bar", CursorGlide: true,

		PathBar: true, StatusBar: true, Scrollbar: true,

		FormatOnSave: true, RestoreSession: true, ReopenProject: true,
	}
}

type settingKind uint8

const (
	settingSwitch settingKind = iota
	settingNumber
	settingChoice
	settingTheme
	settingFont
	settingWeight
)

type settingPage struct{ id, title string }

var settingPages = []settingPage{
	{"appearance", "Appearance"},
	{"fonts", "Fonts"},
	{"editor", "Editor"},
	{"cursor", "Cursor"},
	{"interface", "Interface"},
	{"files", "Files"},
	{"languages", "Languages"},
	{"keyboard", "Keyboard"},
}

type setting struct {
	key         string
	page, group string
	title, help string
	kind        settingKind

	min, max, step float64
	unit           string
	choices        []string

	// desktop settings have no effect in a terminal.
	desktop bool
	// terminal settings have no effect in the desktop app.
	terminal bool
	// added settings came after 0.17. They are written to settings.json,
	// because older versions refuse a config.json with keys they don't know.
	added bool
	// needs names a switch that has to be on for this one to matter.
	needs string
}

var cursorShapes = []string{"block", "bar", "underline"}

var settingList = []setting{
	{key: "theme", page: "appearance", title: "Color theme", help: "Used every time Loom starts", kind: settingTheme},
	{key: "animations", page: "appearance", group: "Motion", title: "Animations", help: "Motion in panes, the cursor and scrolling", kind: settingSwitch},

	{key: "font_family", page: "fonts", group: "Code", title: "Font", help: "The monospaced fonts installed on this computer", kind: settingFont, desktop: true, added: true},
	{key: "font_size", page: "fonts", group: "Code", title: "Size", kind: settingNumber, min: 8, max: 32, step: 0.5, unit: "pt", desktop: true, added: true},
	{key: "font_weight", page: "fonts", group: "Code", title: "Weight", kind: settingWeight, min: 100, max: 900, step: 100, desktop: true, added: true},
	{key: "font_bold_weight", page: "fonts", group: "Code", title: "Bold weight", kind: settingWeight, min: 100, max: 900, step: 100, desktop: true, added: true},
	{key: "line_height", page: "fonts", group: "Code", title: "Line height", help: "A multiple of the font size", kind: settingNumber, min: 1.1, max: 2.4, step: 0.05, unit: "×", desktop: true, added: true},
	{key: "letter_spacing", page: "fonts", group: "Code", title: "Letter spacing", help: "Ligatures only show at 0", kind: settingNumber, min: -1, max: 4, step: 0.25, unit: "pt", desktop: true, added: true},
	{key: "font_ligatures", page: "fonts", group: "Code", title: "Ligatures", help: "Join pairs such as => and != in fonts that have them", kind: settingSwitch, desktop: true, added: true},
	{key: "italic_comments", page: "fonts", group: "Code", title: "Italic comments", help: "Needs a font with an italic style", kind: settingSwitch, added: true},
	{key: "font_fallback", page: "fonts", group: "Code", title: "Fallback font", help: "For characters the code font doesn't have", kind: settingFont, desktop: true, added: true},
	{key: "ui_font_family", page: "fonts", group: "Interface", title: "Font", help: "Menus, tabs, the sidebar and panels", kind: settingFont, desktop: true, added: true},
	{key: "ui_font_size", page: "fonts", group: "Interface", title: "Size", help: "Scales the interface with it", kind: settingNumber, min: 11, max: 18, step: 0.5, unit: "pt", desktop: true, added: true},
	{key: "ui_font_weight", page: "fonts", group: "Interface", title: "Weight", kind: settingWeight, min: 100, max: 900, step: 100, desktop: true, added: true},

	{key: "line_numbers", page: "editor", group: "Display", title: "Line numbers", kind: settingSwitch, added: true},
	{key: "relative_numbers", page: "editor", group: "Display", title: "Relative line numbers", help: "Count from the cursor line", kind: settingSwitch, needs: "line_numbers"},
	{key: "highlight_line", page: "editor", group: "Display", title: "Highlight current line", kind: settingSwitch, added: true},
	{key: "indent_guides", page: "editor", group: "Display", title: "Indent guides", kind: settingSwitch},
	{key: "tab_size", page: "editor", group: "Indentation", title: "Tab size", kind: settingNumber, min: 1, max: 16, step: 1},
	{key: "indent_style", page: "editor", group: "Indentation", title: "Indent with", help: "Auto uses tabs for Go and Makefiles", kind: settingChoice, choices: []string{"auto", "tabs", "spaces"}},
	{key: "scrolloff", page: "editor", group: "Scrolling", title: "Scroll offset", help: "Lines kept visible around the cursor", kind: settingNumber, min: 0, max: 50, step: 1},
	{key: "sticky_scroll", page: "editor", group: "Display", title: "Breadcrumbs", help: "The scopes around the cursor, in the status line", kind: settingSwitch, terminal: true},
	{key: "smooth_scroll", page: "editor", group: "Scrolling", title: "Smooth scrolling", kind: settingSwitch, desktop: true, added: true, needs: "animations"},
	{key: "auto_complete", page: "editor", group: "Typing", title: "Automatic completion", kind: settingSwitch},
	{key: "auto_pairs", page: "editor", group: "Typing", title: "Automatic bracket pairs", kind: settingSwitch},
	{key: "hover_docs", page: "editor", group: "Typing", title: "Documentation on hover", help: "Problems are shown either way", kind: settingSwitch, added: true},

	{key: "cursor_normal", page: "cursor", group: "Shape", title: "Normal and Visual modes", kind: settingChoice, choices: cursorShapes, desktop: true, added: true},
	{key: "cursor_insert", page: "cursor", group: "Shape", title: "Insert mode", kind: settingChoice, choices: cursorShapes, desktop: true, added: true},
	{key: "cursor_blink", page: "cursor", group: "Behavior", title: "Blink", kind: settingSwitch, desktop: true, added: true},
	{key: "cursor_glide", page: "cursor", group: "Behavior", title: "Glide between positions", kind: settingSwitch, desktop: true, added: true, needs: "animations"},

	{key: "path_bar", page: "interface", group: "Window", title: "Path bar", help: "The file's path under the tabs", kind: settingSwitch, desktop: true, added: true},
	{key: "status_bar", page: "interface", group: "Window", title: "Status bar", help: "Mode, problems and cursor position", kind: settingSwitch, desktop: true, added: true},
	{key: "scrollbar", page: "interface", group: "Window", title: "Editor scrollbar", kind: settingSwitch, desktop: true, added: true},

	{key: "format_on_save", page: "files", group: "Saving", title: "Format on save", kind: settingSwitch},
	{key: "organize_imports", page: "files", group: "Saving", title: "Organize imports on save", kind: settingSwitch},
	{key: "reopen_project", page: "files", group: "Startup", title: "Reopen the last project", help: "Off starts with no project open", kind: settingSwitch, added: true},
	{key: "restore_session", page: "files", group: "Startup", title: "Restore session", help: "Tabs, splits and cursor positions of a project", kind: settingSwitch},
}

var settingByKey = func() map[string]setting {
	m := make(map[string]setting, len(settingList))
	for _, s := range settingList {
		m[s.key] = s
	}
	return m
}()

// optionField finds the field of options that each key is stored in.
var optionField = func() map[string]int {
	m := map[string]int{}
	t := reflect.TypeFor[options]()
	for i := range t.NumField() {
		m[t.Field(i).Tag.Get("json")] = i
	}
	return m
}()

func (o options) value(key string) any {
	i, ok := optionField[key]
	if !ok {
		return nil
	}
	return reflect.ValueOf(o).Field(i).Interface()
}

func (o options) on(key string) bool {
	v, _ := o.value(key).(bool)
	return v
}

func (o options) number(key string) float64 {
	switch v := o.value(key).(type) {
	case int:
		return float64(v)
	case float64:
		return v
	}
	return 0
}

func (o options) text(key string) string {
	v, _ := o.value(key).(string)
	return v
}

func formatNumber(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func (o options) validate() error {
	for _, s := range settingList {
		switch s.kind {
		case settingNumber, settingWeight:
			if v := o.number(s.key); v < s.min || v > s.max {
				return fmt.Errorf("%s must be between %s and %s", s.key, formatNumber(s.min), formatNumber(s.max))
			}
		case settingChoice:
			if !slices.Contains(s.choices, o.text(s.key)) {
				last := len(s.choices) - 1
				return fmt.Errorf("%s must be %s or %s", s.key, strings.Join(s.choices[:last], ", "), s.choices[last])
			}
		}
	}
	return nil
}
