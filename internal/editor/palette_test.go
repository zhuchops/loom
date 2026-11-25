package editor

import (
	"strings"
	"testing"
)

func TestFormatKey(t *testing.T) {
	cases := map[string]string{
		"ctrl+k e": "Ctrl+K e", "space t 1": "Space t 1", "ctrl+s": "Ctrl+S", "g g": "g g",
		"] d": "] d", "ctrl+space": "Ctrl+Space", "enter": "Enter", "": "",
	}
	for in, want := range cases {
		if got := formatKey(in); got != want {
			t.Errorf("formatKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWhichKeyLabels(t *testing.T) {
	if got := keyLabel("left down"); got != "← ↓" {
		t.Errorf("arrows = %q", got)
	}
	if got := chordTitle("ctrl+w"); got != "Ctrl+W · Window" {
		t.Errorf("chordTitle = %q", got)
	}
	if got := chordTitle("v"); got != "v …" {
		t.Errorf("unnamed chordTitle = %q", got)
	}
	if got := humanize("split-below"); got != "Split below" {
		t.Errorf("humanize = %q", got)
	}
}

func TestPaletteShowsReadableEntries(t *testing.T) {
	e := testEditor(t, "vim", "")
	press(e, "space p")
	if e.menu == nil {
		t.Fatal("palette did not open")
	}
	var saveHint string
	for i, o := range e.menu.options {
		if i > 0 && strings.ToLower(e.menu.options[i-1].label) > strings.ToLower(o.label) {
			t.Fatalf("palette is not sorted at %q", o.label)
		}
		if strings.Contains(o.label, "-") && !strings.HasPrefix(o.label, ":") {
			t.Errorf("palette label looks like a raw id: %q", o.label)
		}
		if strings.HasPrefix(o.hint, "tab-") || strings.HasPrefix(o.hint, "pane-") {
			t.Errorf("palette hint shows a raw id: %q", o.hint)
		}
		if o.label == "Save File" {
			saveHint = o.hint
		}
		if o.label == "Go to Tab 1" || o.label == "Save" {
			t.Errorf("palette lists an internal entry: %q", o.label)
		}
	}
	if saveHint != "Ctrl+S  :w" {
		t.Fatalf("Save File hint = %q", saveHint)
	}
}

func TestPaletteRanksCommandNamesFirst(t *testing.T) {
	for query, want := range map[string]string{"wq": "Save and Quit", "q": "Quit", "q!": "Quit Without Saving", "w": "Save File"} {
		e := testEditor(t, "vim", "")
		press(e, "space p")
		for _, k := range query {
			e.handle(keyInput{key: string(k)})
		}
		if e.menu == nil || len(e.menu.matches) == 0 {
			t.Fatalf("%q: no matches", query)
		}
		if got := e.menu.matches[0].label; got != want {
			t.Errorf("%q: first match = %q, want %q", query, got, want)
		}
	}
}

func TestTabNumbersUseSpaceT(t *testing.T) {
	e := testEditor(t, "vim", "")
	if e.keys["vim.normal"]["space t 3"] != "tab-3" {
		t.Fatal("space t 3 does not go to tab 3")
	}
	if _, ok := e.keys["vim.normal"]["space 3"]; ok {
		t.Fatal("space 3 is still bound")
	}
}

func TestTreeIcons(t *testing.T) {
	cases := []struct {
		path      string
		dir, open bool
		want      string
	}{
		{"/p/src", true, false, "\U000f024b"},
		{"/p/src", true, true, "\U000f0770"},
		{"/p/main.go", false, false, "\U000f07d3"},
		{"/p/go.mod", false, false, "\U000f07d3"},
		{"/p/Cargo.lock", false, false, "\U000f033e"},
		{"/p/app.TSX", false, false, "\U000f0708"},
		{"/p/app.ts", false, false, "\U000f06e6"},
		{"/p/notes.md", false, false, "\U000f0354"},
		{"/p/unknown.xyz", false, false, "\U000f09ee"},
	}
	for _, c := range cases {
		if got := treeIcon(c.path, c.dir, c.open); got != c.want {
			t.Errorf("treeIcon(%q, %v, %v) = %q, want %q", c.path, c.dir, c.open, got, c.want)
		}
	}
}

func TestPaletteCloseTabAndLastPane(t *testing.T) {
	t.Run("close tab from a split", func(t *testing.T) {
		e := testEditor(t, "vim", "")
		e.open(fixture(t, e.root, "one.txt", "one"))
		e.open(fixture(t, e.root, "two.txt", "two"))
		e.splitPane(true, "")
		closed := e.b
		press(e, "space p")
		e.handle(keyInput{paste: "close tab"})
		if len(e.menu.matches) == 0 || e.menu.matches[0].label != "Close Tab" {
			t.Fatal("Close Tab is missing or not first in the palette")
		}
		press(e, "enter")
		if len(e.buffers) != 1 || len(e.layout.leaves()) != 2 || e.b == closed {
			t.Fatal("Close Tab did not remove the tab and keep both panes")
		}
	})
	t.Run("last pane falls back to closing tab", func(t *testing.T) {
		e := testEditor(t, "vim", "")
		e.showDashboard()
		e.open(fixture(t, e.root, "one.txt", "one"))
		press(e, "A ! esc ctrl+w c")
		if len(e.buffers) != 2 || !e.b.dirty() {
			t.Fatal("closing the last pane discarded an unsaved tab")
		}
		press(e, "u")
		press(e, "ctrl+w c")
		if len(e.buffers) != 1 || !e.b.dashboard || e.quit {
			t.Fatal("closing the last pane did not return to Dashboard")
		}
	})
}
