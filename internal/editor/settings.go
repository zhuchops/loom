package editor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
)

// tabSizeFor and indentStyleFor apply a language's own settings over the global ones.
func (e *editor) tabSizeFor(path string) int {
	if n := e.cfg.Languages[e.cfg.languageFor(path)].TabSize; n > 0 {
		return n
	}
	return max(1, e.cfg.TabSize)
}

func (e *editor) indentStyleFor(path string) string {
	if s := e.cfg.Languages[e.cfg.languageFor(path)].IndentStyle; s != "" {
		return s
	}
	return e.cfg.IndentStyle
}

func (e *editor) formatOnSaveFor(path string) bool {
	if f := e.cfg.Languages[e.cfg.languageFor(path)].FormatOnSave; f != nil {
		return *f
	}
	return e.cfg.FormatOnSave
}

// useTabWidth makes tabs in b draw with b's language setting.
func (e *editor) useTabWidth(b *buffer) { tabWidth = e.tabSizeFor(b.path) }

// reloadConfig re-reads the user and project config without restarting. A
// broken file is reported and the running settings stay as they were.
func (e *editor) reloadConfig() {
	cfg, err := loadConfig(e.cfg.path, e.cfg.explicit, e.root)
	if err != nil {
		e.notify(err.Error())
		return
	}
	cfg.Profile, cfg.Plugins = e.cfg.Profile, e.cfg.Plugins
	keys, err := compileKeys(cfg, e.commands)
	if err != nil {
		e.notify("Config: " + err.Error())
		return
	}
	if err := loadTheme(cfg); err != nil {
		e.notify("Config: " + err.Error())
		return
	}
	e.cfg, e.keys = cfg, keys
	e.relativeNumbers = cfg.RelativeNumbers
	e.useTabWidth(e.b)
	for _, b := range e.buffers {
		b.semantic, b.lexical = nil, nil
		b.semanticVersion, b.highlightVersion = 0, 0
	}
	e.notify("Config reloaded")
}

// configSaved reloads when the file just saved is the user or project config.
func (e *editor) configSaved(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	user, _ := filepath.Abs(e.cfg.path)
	if abs == user || abs == filepath.Join(e.root, ".loom.json") {
		e.reloadConfig()
	}
}

// persistSetting writes one top-level key into the user config file.
func (e *editor) persistSetting(key string, value any) {
	path := e.cfg.path
	if path == "" {
		path = configPath()
	}
	doc := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &doc); err != nil {
			e.notify("Config is not valid JSON: " + err.Error())
			return
		}
	}
	doc[key] = value
	out, err := json.MarshalIndent(doc, "", "  ")
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0o755)
	}
	if err == nil {
		err = os.WriteFile(path, append(out, '\n'), 0o644)
	}
	if err != nil {
		e.notify(err.Error())
		return
	}
	e.cfg.path = path
	e.reloadConfig()
}

// next returns the entry after current in choices, wrapping around.
func next[T comparable](choices []T, current T) T {
	i := slices.Index(choices, current)
	return choices[(i+1)%len(choices)]
}

func (e *editor) settingsMenu(selected int) {
	m := &menu{title: "Settings", hidePreview: true}
	state := func(on bool) string {
		if on {
			return "on"
		}
		return "off"
	}
	flip := func(label, key string, value bool) {
		m.options = append(m.options, option{label, state(value), func() {
			e.persistSetting(key, !value)
		}, nil})
	}
	choose := func(label, key string, hint string, to func() any) {
		m.options = append(m.options, option{label, hint, func() {
			e.persistSetting(key, to())
		}, nil})
	}
	flip("Relative line numbers", "relative_numbers", e.cfg.RelativeNumbers)
	flip("Format on save", "format_on_save", e.cfg.FormatOnSave)
	flip("Organize imports on save", "organize_imports", e.cfg.OrganizeImports)
	flip("Automatic completion", "auto_complete", e.cfg.AutoComplete)
	flip("Automatic bracket pairs", "auto_pairs", e.cfg.AutoPairs)
	flip("Restore session", "restore_session", e.cfg.RestoreSession)
	flip("Sticky scroll and breadcrumbs", "sticky_scroll", e.cfg.StickyScroll)
	choose("Tab size", "tab_size", strconv.Itoa(e.cfg.TabSize), func() any { return next([]int{2, 3, 4, 8}, e.cfg.TabSize) })
	choose("Indent style", "indent_style", e.cfg.IndentStyle, func() any { return next([]string{"auto", "tabs", "spaces"}, e.cfg.IndentStyle) })
	choose("Scroll offset", "scrolloff", strconv.Itoa(e.cfg.Scrolloff), func() any { return next([]int{0, 3, 5, 8, 12}, e.cfg.Scrolloff) })
	m.options = append(m.options,
		option{"Color theme…", th.name, e.themeMenu, nil},
		option{"Open config file", e.cfg.path, e.openConfigFile, nil},
		option{"Reload config", "", e.reloadConfig, nil})
	// Reopen on the same row after a change so several can be flipped in a row.
	for i := range m.options {
		run := m.options[i].run
		label := m.options[i].label
		if i < 10 {
			m.options[i].run = func() {
				run()
				e.settingsMenu(settingsIndex(label))
			}
		}
	}
	e.openMenu(m)
	if selected > 0 && selected < len(m.matches) {
		m.selected = selected
	}
}

var settingsOrder = []string{"Relative line numbers", "Format on save", "Organize imports on save", "Automatic completion", "Automatic bracket pairs", "Restore session", "Sticky scroll and breadcrumbs", "Tab size", "Indent style", "Scroll offset"}

func settingsIndex(label string) int { return slices.Index(settingsOrder, label) }
