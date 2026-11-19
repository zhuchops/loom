package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestProjectConfigIsRestricted(t *testing.T) {
	user, root := t.TempDir(), t.TempDir()
	write := func(dir, name, text string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	userPath := write(user, "config.json", `{"tab_size": 2}`)
	write(root, ".loom.json", `{"tab_size": 8, "languages": {"python": {"tab_size": 3}}}`)
	cfg, err := loadConfig(userPath, true, root)
	if err != nil || cfg.TabSize != 8 || cfg.Languages["python"].TabSize != 3 {
		t.Fatalf("project settings not applied: %v %+v", err, cfg.TabSize)
	}
	write(root, ".loom.json", `{"languages": {"go": {"server": ["evil"]}}}`)
	if _, err := loadConfig(userPath, true, root); err == nil {
		t.Fatal("project config may not set a language server")
	}
	write(root, ".loom.json", `{"plugins": [{"name": "x", "command": ["x"]}]}`)
	if _, err := loadConfig(userPath, true, root); err == nil {
		t.Fatal("project config may not set plugins")
	}
}

func TestReloadAndSettings(t *testing.T) {
	e := testEditor(t, "vim", "x")
	path := filepath.Join(t.TempDir(), "config.json")
	e.cfg.path, e.cfg.explicit = path, true
	os.WriteFile(path, []byte(`{"scrolloff": 2, "bindings": {"modal": {"Q": "quit"}}}`), 0o644)
	e.reloadConfig()
	if e.cfg.Scrolloff != 2 || e.keys["vim.normal"]["Q"] != "quit" {
		t.Fatalf("reload ignored the file: %d %v", e.cfg.Scrolloff, e.keys["vim.normal"]["Q"])
	}
	os.WriteFile(path, []byte(`{"scrolloff": 99}`), 0o644)
	e.reloadConfig()
	if e.cfg.Scrolloff != 2 {
		t.Fatal("a broken config must leave the running settings alone")
	}
	os.WriteFile(path, []byte(`{"scrolloff": 2}`), 0o644)
	e.persistSetting("tab_size", 8)
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), `"tab_size": 8`) || e.cfg.TabSize != 8 {
		t.Fatalf("setting not saved: %s", raw)
	}
}

func TestLanguageSettings(t *testing.T) {
	e := testEditor(t, "vim", "")
	lang := e.cfg.Languages["python"]
	lang.TabSize, lang.IndentStyle = 2, "spaces"
	e.cfg.Languages["python"] = lang
	if e.tabSizeFor("a.py") != 2 || e.tabSizeFor("a.go") != 4 || e.indentStyleFor("a.py") != "spaces" {
		t.Fatal("per-language settings ignored")
	}
}

func TestMoveLinesAndAltKeys(t *testing.T) {
	e := testEditor(t, "vim", "a\nb\nc\n")
	press(e, "alt+down")
	if got := string(e.b.text); got != "b\na\nc\n" {
		t.Fatalf("move down: %q", got)
	}
	press(e, "alt+up")
	if got := string(e.b.text); got != "a\nb\nc\n" {
		t.Fatalf("move up: %q", got)
	}
	if k := terminalKey(tea.Key{Code: tea.KeyUp, Mod: tea.ModAlt}); k.key != "alt+up" {
		t.Fatalf("alt+up arrives as %q", k.key)
	}
	if k := terminalKey(tea.Key{Code: tea.KeyEscape}); k.key != "esc" {
		t.Fatal("esc broken")
	}
}

func TestScrolloffKeepsContext(t *testing.T) {
	e := testEditor(t, "vim", strings.Repeat("x\n", 100))
	v := &renderer{screen: newTerminalFrame(80, 24)}
	for range 40 {
		press(e, "j")
		v.draw(e)
	}
	row := e.b.rowAt(e.b.cursor)
	if visible := e.viewRows - (row - e.b.top) - 1; visible < e.cfg.Scrolloff {
		t.Fatalf("only %d lines below the cursor (top %d row %d)", visible, e.b.top, row)
	}
}
