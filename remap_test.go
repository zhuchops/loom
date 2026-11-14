package main

import (
	"strings"
	"testing"
)

func TestSpaceChordsInDirectoryPopup(t *testing.T) {
	e := testEditor(t, "vim", "")
	fixture(t, e.root, "a.txt", "a\n")
	press(e, "space x")
	if e.directory == nil {
		t.Fatal("space x did not open the directory popup")
	}
	press(e, "space /")
	if e.directory != nil || e.menu == nil || !strings.HasPrefix(e.menu.title, "Search Project") {
		t.Fatalf("space / in directory popup: directory=%v menu=%v", e.directory != nil, e.menu)
	}
}

func TestPaletteFromFocusedExplorer(t *testing.T) {
	e := testEditor(t, "vim", "")
	press(e, "space e")
	if !e.tree.focused {
		t.Fatal("space e did not focus the explorer")
	}
	press(e, "space p")
	if e.menu == nil || e.menu.title != "Command Palette" {
		t.Fatal("space p did not open the palette from the explorer")
	}
}

func TestCtrlSpaceCompletesInInsert(t *testing.T) {
	e := testEditor(t, "vim", "hello")
	press(e, "i")
	if e.mode != "insert" {
		t.Fatal("i did not enter insert mode")
	}
	if id := e.keys["vim.insert"]["ctrl+space"]; id != "complete" {
		t.Fatalf("ctrl+space in insert = %q", id)
	}
	if id := e.keys["vim.insert"]["ctrl+k"]; id != "" {
		t.Fatal("ctrl+k is still bound in insert")
	}
}
