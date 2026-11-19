package editor

import (
	"strings"
	"testing"
)

func TestNativeKeys(t *testing.T) {
	e := testEditor(t, "vim", strings.Repeat("line\n", 100))
	e.viewRows = 11
	press(e, "pagedown")
	if row := e.b.rowAt(e.b.cursor); row != 10 {
		t.Fatalf("pagedown moved to row %d", row)
	}
	press(e, "pageup")
	if e.b.cursor != 0 {
		t.Fatalf("pageup left cursor at %d", e.b.cursor)
	}
	press(e, "ctrl+a")
	if e.mode != "select" || e.b.anchor != 0 || e.b.cursor != len(e.b.text)-1 {
		t.Fatalf("select all: mode %s anchor %d cursor %d", e.mode, e.b.anchor, e.b.cursor)
	}
	press(e, "ctrl+x")
	if len(e.b.text) != 0 {
		t.Fatalf("cut left text %q", string(e.b.text))
	}
	press(e, "ctrl+z")
	if len(e.b.text) == 0 {
		t.Fatalf("ctrl+z did not undo the cut")
	}
}

func TestInsertHomeEndAndTabs(t *testing.T) {
	e := testEditor(t, "vim", "    abc def")
	press(e, "i end")
	if e.b.cursor != len(e.b.text) {
		t.Fatalf("end: %d", e.b.cursor)
	}
	press(e, "home")
	if e.b.cursor != 4 {
		t.Fatalf("home should go to the first character: %d", e.b.cursor)
	}
	press(e, "home")
	if e.b.cursor != 0 {
		t.Fatalf("second home should go to column 0: %d", e.b.cursor)
	}

	e = testEditor(t, "vim", "")
	e.cfg.IndentStyle, e.cfg.TabSize = "spaces", 2
	tabWidth = 2
	defer func() { tabWidth = 4 }()
	press(e, "i x tab")
	if got := string(e.b.text); got != "x " {
		t.Fatalf("tab should reach the next stop: %q", got)
	}
}
