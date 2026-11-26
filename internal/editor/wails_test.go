//go:build desktop

package editor

import (
	"context"
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/zhuchops/loom/internal/theme"
)

// testBridge is a Bridge around a test editor, without a window.
func testBridge(t *testing.T, input string) (*Bridge, *editor) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	// Sessions and settings change the theme for the whole process.
	was := th.name
	t.Cleanup(func() {
		for _, t2 := range theme.All {
			if t2.Name == was {
				applyTheme(t2)
			}
		}
	})
	e := testEditor(t, "vim", input)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	b := newBridge(e, ctx, false, nil)
	t.Cleanup(b.ticker.Stop)
	b.frame.colors, b.frame.inks = deriveColors(), map[[2]color.NRGBA]color.NRGBA{}
	b.started = true
	b.resize(80, 24)
	b.m.view.draw(e)
	return b, e
}

func TestWebKeys(t *testing.T) {
	cases := []struct {
		ev   KeyEvent
		want string
		ok   bool
	}{
		{KeyEvent{Key: "ArrowLeft"}, "left", true},
		{KeyEvent{Key: "ArrowLeft", Shift: true}, "left+shift", true},
		{KeyEvent{Key: "a", Char: "a", Ctrl: true}, "ctrl+a", true},
		{KeyEvent{Key: "ф", Char: "a", Ctrl: true}, "ctrl+a", true}, // a Russian layout
		{KeyEvent{Key: "ä", Char: "a", Alt: true}, "alt+a", true},   // Option types a letter on macOS
		{KeyEvent{Key: "a", Char: "a"}, "", false},                  // typing arrives as text
		{KeyEvent{Key: " ", Char: " ", Ctrl: true}, "ctrl+space", true},
	}
	if runtime.GOOS == "darwin" {
		cases = append(cases,
			struct {
				ev   KeyEvent
				want string
				ok   bool
			}{KeyEvent{Key: "ArrowRight", Meta: true}, "end", true},
			struct {
				ev   KeyEvent
				want string
				ok   bool
			}{KeyEvent{Key: "Backspace", Meta: true}, "super+backspace", true},
			struct {
				ev   KeyEvent
				want string
				ok   bool
			}{KeyEvent{Key: "s", Char: "s", Meta: true}, "", false}) // the menu bar's
	}
	for _, c := range cases {
		k, ok := webKey(c.ev)
		if ok != c.ok {
			t.Fatalf("%+v: handled %v", c.ev, ok)
		}
		if !ok {
			continue
		}
		in := terminalKey(k)
		got := in.key
		if in.shift {
			got += "+shift"
		}
		if got != c.want {
			t.Fatalf("%+v became %q, want %q", c.ev, got, c.want)
		}
	}
}

func TestGridRowsAreRunsOfOneStyle(t *testing.T) {
	b, e := testBridge(t, "hello 世界 world\n")
	e.cfg.LineNumbers = false
	b.m.view.draw(e)
	runs := b.encodeRow(b.m.view.screen, 0, b.cols)
	var text strings.Builder
	at := 0
	for _, r := range runs {
		s, x, cells := r[0].(string), r[1].(int), r[2].(int)
		if x < at {
			t.Fatalf("runs overlap: %v", runs)
		}
		at = x + cells
		text.WriteString(s)
		if strings.ContainsAny(s, "世界") && (s != "世" && s != "界" || cells != 2) {
			t.Fatalf("a wide character shares a run or has the wrong width: %q %d", s, cells)
		}
	}
	if !strings.Contains(text.String(), "hello") || !strings.Contains(text.String(), "world") {
		t.Fatalf("row text: %q", text.String())
	}
	if _, err := json.Marshal(runs); err != nil {
		t.Fatal(err)
	}
}

func TestChromeDescribesTheWindow(t *testing.T) {
	b, e := testBridge(t, "package main\n")
	e.b.path = e.root + "/main.go"
	ui := b.chrome()
	data, err := json.Marshal(ui)
	if err != nil {
		t.Fatal(err)
	}
	if ui["title"] == nil || ui["status"] == nil {
		t.Fatalf("chrome: %s", data)
	}
	status := ui["status"].(map[string]any)
	if mode := status["mode"].([]string); mode[0] != "NORMAL" {
		t.Fatalf("mode badge %v", mode)
	}
	e.openPalette()
	press(e, "s p l")
	m := b.chrome()["menu"].(map[string]any)
	rows := m["rows"].([]any)
	if len(rows) == 0 || m["field"].(map[string]any)["before"] != "spl" {
		t.Fatalf("palette: %v", m)
	}
	first := rows[0].(map[string]any)
	if first["hits"] == nil || !strings.Contains(strings.ToLower(first["label"].(string)), "split") {
		t.Fatalf("first row: %v", first)
	}
	b.act("menu-pick", 0, "")
	if e.menu != nil {
		t.Fatal("picking a row left the palette open")
	}
}

func TestFieldsTakeTheMouseFromThePage(t *testing.T) {
	b, e := testBridge(t, "")
	e.openMenu(&menu{title: "Rename Symbol", input: "fetchUser", submit: func(string) {}})
	b.act("field", 5, "menu-press")
	b.act("field", 9, "menu-drag")
	if got := e.menu.field.selected(e.menu.input); got != "User" {
		t.Fatalf("selected %q", got)
	}
	f := b.chrome()["menu"].(map[string]any)["field"].(map[string]any)
	if f["before"] != "fetch" || f["sel"] != "User" {
		t.Fatalf("field state %v", f)
	}
}

func TestSettingsWindow(t *testing.T) {
	b, e := testBridge(t, "")
	b.toggleSettings()
	s := b.settingsState()
	if len(s["pages"].([]string)) != len(settingPages) || len(s["rows"].([]any)) == 0 {
		t.Fatalf("settings: %v", s)
	}
	b.settingsKey(keyInput{key: "tab"})
	if b.settings.page != 1 {
		t.Fatalf("Tab went to page %d", b.settings.page)
	}
	// Find the font size and make it bigger with its + button.
	size := -1
	for _, r := range b.settingsState()["rows"].([]any) {
		row := r.(map[string]any)
		if row["title"] == "Size" && size < 0 {
			size = row["i"].(int) // the code font's; the interface one comes later
		}
	}
	if size < 0 {
		t.Fatal("no Size row on the Fonts page")
	}
	before := e.cfg.FontSize
	b.act("settings-step", size, "1")
	if e.cfg.FontSize <= before {
		t.Fatalf("font size %v after %v: %s", e.cfg.FontSize, before, b.settings.failure)
	}
	b.act("settings-fonts", 0, `[{"name":"Menlo","mono":true},{"name":"Avenir Next","mono":false}]`)
	b.openFontPicker(settingByKey["font_family"])
	p := b.settingsState()["picker"].(map[string]any)
	if n := len(p["choices"].([]any)); n != 2 { // the default and Menlo; Avenir isn't monospaced
		t.Fatalf("code font choices: %v", p["choices"])
	}
	b.act("settings-font", -1, "Menlo")
	if e.cfg.FontFamily != "Menlo" {
		t.Fatalf("font family %q", e.cfg.FontFamily)
	}
	b.settingsKey(keyInput{key: "esc"})
	if b.settings != nil {
		t.Fatal("Esc left the settings open")
	}
}

func TestDialogAnswers(t *testing.T) {
	b, e := testBridge(t, "")
	buf := e.b
	buf.path = e.root + "/x.txt"
	buf.text = []rune("changed")
	buf.indexLines()
	b.closeTab(buf)
	if b.dialog == nil || len(b.dialog.Choices) != 3 {
		t.Fatalf("closing a changed tab asked %+v", b.dialog)
	}
	b.Key(KeyEvent{Key: "Escape"}) // queued
	c := <-b.desk.calls
	c.f()
	if b.dialog != nil {
		t.Fatal("Escape left the question up")
	}
	_ = tea.KeyPressMsg{}
}

func TestFileMarks(t *testing.T) {
	c := deriveColors()
	for _, path := range []string{"a.ex", "test/a_test.exs", "mix.exs", "page.html.heex", "main.dingo", "main.go"} {
		if mark, _ := fileMarkOf(&c, path); mark == "" {
			t.Errorf("%s has no icon", path)
		}
	}
}

func TestDesktopOpensWhatALaterLaunchSends(t *testing.T) {
	t.Chdir(t.TempDir()) // opening a project moves the process there; come back after
	b, e := testBridge(t, "")
	state, err := os.MkdirTemp("/tmp", "ls") // short: a socket path has a length limit
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(state) })
	t.Setenv("XDG_STATE_HOME", state)
	h := e.hub
	notes := filepath.Join(e.root, "NOTES.txt")
	if err := os.WriteFile(notes, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.openRequested(openRequest{Cwd: e.root, Paths: []string{"NOTES.txt"}})
	if h.current.b.path != resolveOpenPath("/", notes) {
		t.Fatalf("buffer %q", h.current.b.path)
	}
	// A new folder becomes a session.
	n := len(h.live)
	fresh := filepath.Join(t.TempDir(), "fresh")
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	b.openRequested(openRequest{Cwd: "/", Paths: []string{fresh}})
	if len(h.live) != n+1 || filepath.Base(h.current.root) != "fresh" {
		t.Fatalf("a new folder did not become a session: %d sessions", len(h.live))
	}
	// The socket carries a request from a second launch to the first.
	got := make(chan openRequest, 1)
	listenForRequests(func(r openRequest) { got <- r }, nil)
	if !forwardToRunning([]string{"a.txt", "dir"}, false) {
		t.Fatal("a running Loom did not take the launch")
	}
	if r := <-got; len(r.Paths) != 2 || r.Cwd == "" {
		t.Fatalf("request = %+v", r)
	}
	if info, err := os.Stat(ipcSocket()); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("socket: %v %v", info, err)
	}
}

func TestTabDroppedOnTheTextSplits(t *testing.T) {
	b, e := testBridge(t, "one\n")
	path := filepath.Join(e.root, "two.txt")
	if err := os.WriteFile(path, []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.open(path)
	b.m.view.draw(e)
	pane := b.m.view.panes[0]
	b.act("tab-over", pane.x+pane.w-2, strconv.Itoa(pane.y+pane.h/2)) // the right edge
	if b.m.view.drop.leaf == nil {
		t.Fatal("no drop zone over the text")
	}
	b.act("tab-drop", -1, "")
	if n := len(e.layout.leaves()); n != 2 {
		t.Fatalf("%d panes after the drop", n)
	}
}
