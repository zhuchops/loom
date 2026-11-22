package editor

import (
	"path/filepath"
	"slices"
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
	cfg, err := loadConfig(e.cfg.path, e.cfg.explicit, e.configRoot())
	if err != nil {
		e.notify(err.Error())
		return
	}
	cfg.Profile = e.cfg.Profile
	keys, err := compileKeys(cfg, e.commands)
	if err == nil {
		err = e.useConfig(cfg, keys)
	}
	if err != nil {
		e.notify("Config: " + err.Error())
		return
	}
	e.notify("Config reloaded")
}

// useConfig switches the running editor to cfg.
func (e *editor) useConfig(cfg config, keys map[string]map[string]string) error {
	if err := checkTheme(cfg); err != nil {
		return err
	}
	before, oldTheme := e.cfg, th.name
	if before.Theme != "" && cfg.Theme == "" {
		applyTheme(defaultTheme()) // unless one was picked before, below
	}
	if err := loadTheme(cfg); err != nil {
		return err
	}
	cfg.Profile, cfg.Plugins = before.Profile, before.Plugins
	e.cfg, e.keys = cfg, keys
	// Space n changes this for the session; only a changed setting undoes that.
	if cfg.RelativeNumbers != before.RelativeNumbers {
		e.relativeNumbers = cfg.RelativeNumbers
	}
	italicComments = cfg.ItalicComments
	e.useTabWidth(e.b)
	if e.directory != nil {
		doc := e.directory.doc
		doc.cfg, doc.keys = cfg, keys
		doc.cfg.AutoComplete, doc.cfg.AutoPairs = false, false
	}
	if th.name != oldTheme || cfg.ItalicComments != before.ItalicComments {
		e.refreshHighlights()
	}
	return nil
}

// refreshHighlights rebuilds highlights, which carry colours and styles.
func (e *editor) refreshHighlights() {
	for _, b := range e.buffers {
		b.semantic, b.lexical = nil, nil
		b.semanticVersion, b.highlightVersion = 0, 0
	}
	if e.b.path != "" {
		e.scheduleLanguage()
	}
}

// configSaved reloads when the file just saved is the user or project config.
func (e *editor) configSaved(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	user, _ := filepath.Abs(e.userConfigPath())
	if abs == user || abs == addedPath(user) || abs == filepath.Join(e.root, ".loom.json") {
		e.reloadConfig()
	}
}

// persistSetting saves one setting and reports what went wrong.
func (e *editor) persistSetting(key string, value any) {
	if err := e.setSetting(key, value); err != nil {
		e.notify(err.Error())
	}
}

// next returns the entry after current in choices, wrapping around.
func next[T comparable](choices []T, current T) T {
	i := slices.Index(choices, current)
	return choices[(i+1)%len(choices)]
}

// openSettings shows the settings window, or the list where there is none.
func (e *editor) openSettings() {
	if e.settingsPage != nil {
		e.settingsPage()
		return
	}
	e.settingsMenu(0)
}

// cycleValue is what picking a row in the terminal list changes it to.
func (e *editor) cycleValue(s setting) any {
	switch s.kind {
	case settingSwitch:
		return !e.cfg.on(s.key)
	case settingChoice:
		return next(s.choices, e.cfg.text(s.key))
	}
	steps := map[string][]float64{"tab_size": {2, 3, 4, 8}, "scrolloff": {0, 3, 5, 8, 12}}[s.key]
	return next(steps, e.cfg.number(s.key))
}

// settingsMenu lists the settings that mean something in a terminal. Picking
// a row changes it and leaves the list on that row.
func (e *editor) settingsMenu(selected int) {
	m := &menu{title: "Settings", hidePreview: true}
	for _, s := range settingList {
		if s.desktop {
			continue
		}
		if s.kind == settingTheme {
			m.options = append(m.options, option{s.title, th.name, e.themeMenu, nil})
			continue
		}
		var state string
		switch s.kind {
		case settingSwitch:
			state = "off"
			if e.cfg.on(s.key) {
				state = "on"
			}
		case settingChoice:
			state = e.cfg.text(s.key)
		case settingNumber:
			state = formatNumber(e.cfg.number(s.key))
		}
		if e.projectSets(s.key) {
			state += " (project)"
		}
		index := len(m.options)
		m.options = append(m.options, option{s.title, state, func() {
			e.persistSetting(s.key, e.cycleValue(s))
			e.settingsMenu(index)
		}, nil})
	}
	m.options = append(m.options,
		option{"Keyboard shortcuts", "", e.showKeys, nil},
		option{"Open config file", e.userConfigPath(), e.openConfigFile, nil},
		option{"Reload config", "", e.reloadConfig, nil})
	e.openMenu(m)
	if selected > 0 && selected < len(m.matches) {
		m.selected = selected
	}
}
