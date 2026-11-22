package editor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func configEditor(t *testing.T, main string) (*editor, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	path := filepath.Join(dir, "config.json")
	if main != "" {
		if err := os.WriteFile(path, []byte(main), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e := testEditor(t, "vim", "x")
	e.cfg.path, e.cfg.explicit = path, false
	e.cfg.options = defaultOptions() // testEditor turns some off
	e.b.saved = string(e.b.text)
	t.Cleanup(func() { applyTheme(baselineTheme()); italicComments = false })
	return e, path
}

func fileKeys(t *testing.T, path string) map[string]any {
	t.Helper()
	out := map[string]any{}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return out
	}
	if err != nil || json.Unmarshal(raw, &out) != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return out
}

func TestEveryOptionHasASetting(t *testing.T) {
	for key := range optionField {
		if _, ok := settingByKey[key]; !ok {
			t.Errorf("%s can't be changed in the settings", key)
		}
	}
	for _, s := range settingList {
		if _, ok := optionField[s.key]; !ok {
			t.Errorf("%s has nowhere to be stored", s.key)
		}
		if !slices.ContainsFunc(settingPages, func(p settingPage) bool { return p.id == s.page }) {
			t.Errorf("%s is on no page", s.key)
		}
		if s.needs != "" && settingByKey[s.needs].kind != settingSwitch {
			t.Errorf("%s needs %s, which is not a switch", s.key, s.needs)
		}
	}
	if err := defaultOptions().validate(); err != nil {
		t.Fatal(err)
	}
}

// Every setting is saved where it belongs, survives a restart and resets.
func TestEverySettingRoundTrips(t *testing.T) {
	// What loom 0.17 accepts at the top of config.json.
	known := []string{"relative_numbers", "format_on_save", "organize_imports", "restore_session", "tab_size", "scrolloff", "sticky_scroll",
		"animations", "indent_guides", "indent_style", "profile", "bindings", "languages", "plugins", "auto_complete", "auto_pairs", "theme", "language_servers"}
	for _, s := range settingList {
		t.Run(s.key, func(t *testing.T) {
			e, path := configEditor(t, "")
			before := e.cfg.value(s.key)
			var value any
			switch s.kind {
			case settingSwitch:
				value = !e.cfg.on(s.key)
			case settingChoice:
				value = next(s.choices, e.cfg.text(s.key))
			case settingNumber, settingWeight:
				to := e.cfg.number(s.key) + s.step
				if to > s.max {
					to = e.cfg.number(s.key) - s.step
				}
				value = to
			case settingTheme:
				value = "nord"
			case settingFont:
				value = "Some Font"
			}
			if err := e.setSetting(s.key, value); err != nil {
				t.Fatal(err)
			}
			changed := e.cfg.value(s.key)
			if reflect.DeepEqual(changed, before) {
				t.Fatal("did not change")
			}
			again, err := loadConfig(path, false, "")
			if err != nil || !reflect.DeepEqual(again.value(s.key), changed) {
				t.Fatalf("after a restart: %v (%v), want %v", again.value(s.key), err, changed)
			}
			for key := range fileKeys(t, path) {
				if !slices.Contains(known, key) {
					t.Fatalf("%s in config.json would stop loom 0.17 from starting", key)
				}
			}
			if !e.userSets(s.key) {
				t.Fatal("not marked as changed")
			}
			if err := e.resetSetting(s.key); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(e.cfg.value(s.key), before) || e.userSets(s.key) {
				t.Fatalf("reset left %v", e.cfg.value(s.key))
			}
		})
	}
}

func TestSettingsKeepTheRestOfTheFile(t *testing.T) {
	e, path := configEditor(t, `{"organize_imports": true, "languages": {"go": {"server": ["gopls", "-remote=auto"]}}}`)
	e.reloadConfig()
	if err := e.setLanguageSetting("go", "tab_size", 8); err != nil {
		t.Fatal(err)
	}
	if err := e.setSetting("scrolloff", 3.0); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path, false, "")
	if err != nil || !cfg.OrganizeImports || cfg.Scrolloff != 3 || cfg.Languages["go"].TabSize != 8 || len(cfg.Languages["go"].Server) != 2 {
		t.Fatalf("lost part of the file: %v %+v", err, cfg.Languages["go"])
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("the file's permissions changed")
	}
	if err := e.setLanguageSetting("go", "tab_size", nil); err != nil || e.cfg.Languages["go"].TabSize != 0 || len(e.cfg.Languages["go"].Server) != 2 {
		t.Fatalf("resetting a language setting: %v %+v", err, e.cfg.Languages["go"])
	}
}

func TestSettingsRefuseBadValues(t *testing.T) {
	e, path := configEditor(t, `{"scrolloff": 2}`)
	for key, value := range map[string]any{"font_size": 400.0, "cursor_normal": "triangle", "theme": "no-such-theme"} {
		if e.setSetting(key, value) == nil {
			t.Errorf("%s = %v was accepted", key, value)
		}
	}
	if raw, _ := os.ReadFile(path); string(raw) != `{"scrolloff": 2}` {
		t.Fatalf("a refused change was written: %s", raw)
	}
	if _, err := os.Stat(addedPath(path)); !os.IsNotExist(err) {
		t.Fatal("a refused change created settings.json")
	}

	os.WriteFile(path, []byte(`{"scrolloff": `), 0o600)
	if e.setSetting("tab_size", 2.0) == nil {
		t.Fatal("wrote over a file that isn't valid JSON")
	}
	if raw, _ := os.ReadFile(path); string(raw) != `{"scrolloff": ` {
		t.Fatalf("a broken file was replaced: %s", raw)
	}
}

func TestSettingsFollowASymlink(t *testing.T) {
	e, path := configEditor(t, "")
	real := filepath.Join(t.TempDir(), "dotfiles.json")
	os.WriteFile(real, []byte("{}"), 0o644)
	if err := os.Symlink(real, path); err != nil {
		t.Skip(err)
	}
	if err := e.setSetting("tab_size", 2.0); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Lstat(path); info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the symlink was replaced by a file")
	}
	if raw, _ := os.ReadFile(real); !strings.Contains(string(raw), "tab_size") {
		t.Fatalf("the linked file was not written: %s", raw)
	}
}

func TestProjectSettingsWin(t *testing.T) {
	// A project opens in the blank session, or in one of its own beside others.
	for _, blank := range []bool{true, false} {
		e, _ := configEditor(t, "")
		if blank {
			e.b.text = nil
			e.b.indexLines()
			e.b.saved = ""
			e.enterNoProject()
		}
		project := t.TempDir()
		os.WriteFile(filepath.Join(project, ".loom.json"), []byte(`{"tab_size": 8, "theme": "nord"}`), 0o644)
		t.Chdir(project)
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		e.openProject(project)
		e = e.hub.current
		if e.cfg.TabSize != 8 || th.name != "nord" || !e.projectSets("tab_size") {
			t.Fatalf("blank %v: project settings not applied: %d %s", blank, e.cfg.TabSize, th.name)
		}
		if e.setSetting("tab_size", 2.0) == nil {
			t.Fatal("changed a setting the project decides")
		}
		e.closeProject()
		e = e.hub.current
		if e.cfg.TabSize != 4 || th.name == "nord" || e.projectSets("tab_size") {
			t.Fatalf("blank %v: the project's settings outlived it: %d %s", blank, e.cfg.TabSize, th.name)
		}
	}
}

func TestDefaultThemeIsSaved(t *testing.T) {
	e, path := configEditor(t, `{"theme": "nord"}`)
	e.reloadConfig()
	builtin := defaultTheme().Name
	for _, name := range []string{"gruvbox", "dracula"} {
		if err := e.setSetting("theme", name); err != nil || th.name != name {
			t.Fatalf("theme %s not applied: %v", name, err)
		}
	}
	cfg, err := loadConfig(path, false, "")
	if err != nil || cfg.Theme != "dracula" {
		t.Fatalf("the default theme is %q (%v)", cfg.Theme, err)
	}
	if err := e.resetSetting("theme"); err != nil || th.name != builtin {
		t.Fatalf("reset left %s: %v", th.name, err)
	}
}

func TestSessionDoesNotUndoRelativeNumbers(t *testing.T) {
	e, _ := configEditor(t, `{"relative_numbers": true}`)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	e.cfg.RestoreSession = true
	e.relativeNumbers = false
	e.saveSession()
	e.reloadConfig()
	e.restoreSession(false)
	if !e.relativeNumbers {
		t.Fatal("an old session overrode the setting")
	}
}

func TestRebinding(t *testing.T) {
	e, path := configEditor(t, "")
	normal := func() map[string]string { return e.keys["vim.normal"] }

	if err := e.bindKey("normal", "files", "g f"); err != nil {
		t.Fatal(err)
	}
	if got := keysFor(normal(), "files"); !slices.Equal(got, []string{"g f"}) {
		t.Fatalf("Go to File has %v", got)
	}
	if e.keys["vim.insert"]["ctrl+p"] != "files" {
		t.Fatal("changing Normal mode changed Insert mode")
	}
	if e.bindKey("insert", "files", "f") == nil {
		t.Fatal("took a letter away from typing")
	}
	if e.bindKey("normal", "files", "g") == nil {
		t.Fatal("accepted a key that starts other chords")
	}

	if err := e.bindKey("normal", "open-project", "g P"); err != nil || normal()["g P"] != "open-project" {
		t.Fatalf("binding a newer command: %v", err)
	}
	if raw, _ := os.ReadFile(path); strings.Contains(string(raw), "open-project") {
		t.Fatal("a newer command in config.json would stop loom 0.17 from starting")
	}

	if err := e.resetKey("normal", "files"); err != nil {
		t.Fatal(err)
	}
	if normal()["ctrl+p"] != "files" || normal()["space f"] != "files" || normal()["g f"] == "files" {
		t.Fatalf("reset left %v", keysFor(normal(), "files"))
	}
}

func TestBrokenSettingsKeepTheConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte(`{"tab_size": 2}`), 0o644)
	os.WriteFile(addedPath(path), []byte(`{"font_size": 999}`), 0o644)
	if _, err := readConfig(path, false); err == nil {
		t.Fatal("accepted a broken settings.json")
	}
	if cfg, ok := usableConfig(path, false); !ok || cfg.TabSize != 2 || cfg.FontSize != 13 {
		t.Fatalf("lost config.json with it: %v %v", cfg.TabSize, cfg.FontSize)
	}
}
