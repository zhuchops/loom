package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func testEditor(t *testing.T, profile, input string) *editor {
	t.Helper()
	cfg := defaultConfig()
	cfg.Profile = profile
	cfg.AutoComplete = false
	b, err := loadBuffer("")
	if err != nil {
		t.Fatal(err)
	}
	b.text = []rune(input)
	b.indexLines()
	e, err := newEditor(cfg, b, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func press(e *editor, keys string) {
	for _, k := range strings.Fields(keys) {
		e.handle(keyInput{key: k})
	}
}

func TestSharedBindingsInEveryMode(t *testing.T) {
	for _, profile := range []string{"vim", "helix"} {
		for _, mode := range []string{"normal", "select"} {
			t.Run(profile+"/"+mode, func(t *testing.T) {
				e := testEditor(t, profile, "hello")
				e.mode = mode
				press(e, "space p")
				if e.menu == nil || e.menu.title != "Command Palette" {
					t.Fatal("palette failed")
				}
				press(e, "esc space e")
				if !e.tree.focused || len(e.tree.rows) == 0 || e.tree.rows[0].path != e.root {
					t.Fatal("explorer failed")
				}
			})
		}
	}
}

func TestChordValidation(t *testing.T) {
	cfg := defaultConfig()
	cfg.Bindings["vim.select"]["space"] = "save"
	b, _ := loadBuffer("")
	if _, err := newEditor(cfg, b, t.TempDir()); err == nil {
		t.Fatal("accepted a shadowed chord")
	}
}

func TestVimSurroundAndMacro(t *testing.T) {
	e := testEditor(t, "vim", "hello world")
	press(e, "y s i w )")
	if got := string(e.b.text); got != "(hello) world" {
		t.Fatalf("add: %q", got)
	}
	press(e, "c s ) ]")
	if got := string(e.b.text); got != "[hello] world" {
		t.Fatalf("change: %q", got)
	}
	press(e, "d s ]")
	if got := string(e.b.text); got != "hello world" {
		t.Fatalf("delete: %q", got)
	}
	press(e, "q a A ! esc q @ a")
	if got := string(e.b.text); got != "hello world!!" {
		t.Fatalf("macro: %q", got)
	}
}

func TestHelixSelectionSurroundAndMacro(t *testing.T) {
	e := testEditor(t, "helix", "one two three")
	press(e, "w w m s )")
	if got := string(e.b.text); got != "one (two) three" {
		t.Fatalf("surround: %q", got)
	}
	press(e, "m r ) ] m d ]")
	if got := string(e.b.text); got != "one two three" {
		t.Fatalf("change/delete: %q", got)
	}
	press(e, "Q A ! esc Q q")
	if got := string(e.b.text); got != "one two three!!" {
		t.Fatalf("macro: %q", got)
	}
}

func TestCompletionEditsWithUTF16(t *testing.T) {
	e := testEditor(t, "vim", "package p\n𐐀Pr")
	b := e.b
	b.cursor = len(b.text)
	item := completionItem{
		Label:      "Println",
		TextEdit:   &textEdit{Insert: &lspRange{position{1, 2}, position{1, 4}}, NewText: "Println"},
		Additional: []textEdit{{Range: &lspRange{position{1, 0}, position{1, 0}}, NewText: "import \"fmt\"\n"}},
	}
	if err := applyCompletion(b, item); err != nil {
		t.Fatal(err)
	}
	if got := string(b.text); got != "package p\nimport \"fmt\"\n𐐀Println" || b.cursor != len(b.text) {
		t.Fatalf("got %q at %d", got, b.cursor)
	}
	b.history(false)
	if got := string(b.text); got != "package p\n𐐀Pr" {
		t.Fatalf("undo: %q", got)
	}
	item.TextEdit.Insert.Start.Character = 1
	if err := applyCompletion(b, item); err == nil {
		t.Fatal("accepted a split surrogate pair")
	}
}

func TestRawShortcutNames(t *testing.T) {
	for _, test := range []struct {
		key  tea.Key
		name string
	}{
		{tea.Key{Code: 'p', Mod: tea.ModCtrl}, "ctrl+p"},
		{tea.Key{Code: 'k', Mod: tea.ModCtrl}, "ctrl+k"},
		{tea.Key{Code: 's', Mod: tea.ModCtrl}, "ctrl+s"},
		{tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl}, "ctrl+space"},
		{tea.Key{Code: tea.KeyBackspace}, "backspace"},
		{tea.Key{Code: 'v', Text: "V", Mod: tea.ModShift}, "V"},
		{tea.Key{Code: tea.KeyPgDown}, "pagedown"},
	} {
		if got := terminalKey(test.key).key; got != test.name {
			t.Errorf("got %q, want %q", got, test.name)
		}
	}
}

func TestVimChangeLineAndEmptyBuffer(t *testing.T) {
	e := testEditor(t, "vim", "foo\r\nbar")
	press(e, "c c X esc j d d")
	if got := string(e.b.text); got != "X\r\n" {
		t.Fatalf("got %q", got)
	}
	press(e, "g g d d")
	if len(e.b.text) != 0 {
		t.Fatalf("got %q", string(e.b.text))
	}
	press(e, "i left right esc")
}
