package editor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTabsAndPaneLayout(t *testing.T) {
	e := testEditor(t, "vim", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	a := fixture(t, e.root, "one.txt", "one\ntwo\nthree\n")
	b := fixture(t, e.root, "two.txt", "different\n")
	c := fixture(t, e.root, "three.txt", "another\n")
	for _, path := range []string{a, b, c} {
		e.open(path)
	}
	press(e, "space t 1 g t g T")
	if e.b.path != a {
		t.Fatal("tab cycling")
	}
	press(e, "3 g t space t h")
	if e.b.path != c || e.buffers[1].path != c {
		t.Fatal("tab number or reorder")
	}
	press(e, "space t $ space t 0 ctrl+w v")
	left, right := e.layout.leaves()[0], e.activePane
	if e.b.path != a || left == right || left.b != right.b {
		t.Fatal("split must share the active buffer")
	}
	press(e, "j ctrl+w h")
	if e.activePane != left || e.b.cursor != 0 {
		t.Fatal("left view cursor changed")
	}
	press(e, "ctrl+w l")
	if e.activePane != right || e.b.cursor != e.b.lines[1] {
		t.Fatal("right view cursor lost")
	}
	e.ex("split " + b)
	if len(e.layout.leaves()) != 3 || e.b.path != b {
		t.Fatal("nested split failed")
	}
	press(e, "ctrl+w k")
	if e.b.path != a || e.b.cursor != e.b.lines[1] {
		t.Fatal("directional focus failed")
	}
	press(e, "space z space n")
	if !e.zen || !e.relativeNumbers || len(e.layout.leaves()) != 3 {
		t.Fatal("Zen/relative numbers altered layout")
	}
	press(e, "space z")
	e.saveSession()
	blank, _ := loadBuffer("")
	restored, _ := newEditor(e.cfg, blank, e.root)
	restored.restoreSession(false)
	if len(restored.layout.leaves()) != 3 || !restored.relativeNumbers || restored.b.cursor != e.b.cursor || restored.b.path != a {
		t.Fatal("layout did not restore")
	}
	press(e, "ctrl+w c ctrl+w o")
	if len(e.layout.leaves()) != 1 || len(e.buffers) != 3 {
		t.Fatal("closing panes closed buffers")
	}
}

func TestParentAndDirectoryJump(t *testing.T) {
	e := testEditor(t, "vim", "")
	child := filepath.Join(e.root, "child")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	press(e, "space x enter")
	if e.directory.dir != filepath.Dir(e.root) {
		t.Fatal("parent row did not navigate")
	}
	e.ex("cd " + child)
	if e.directory.dir != child {
		t.Fatal("absolute jump failed")
	}
	press(e, "-")
	if e.directory.dir != e.root {
		t.Fatal("parent motion failed")
	}
	press(e, "g o")
	if e.menu == nil || e.menu.title != "Jump to Directory" {
		t.Fatal("directory picker missing")
	}
	e.menuInput(keyInput{paste: "child"})
	press(e, "enter")
	if e.directory.dir != child {
		t.Fatal("relative directory jump failed")
	}
	press(e, "space S space e q")
	if e.directory != nil || e.tree.sidebar || e.tree.focused {
		t.Fatal("close explorer failed")
	}
}

func TestZenAndRelativeGutter(t *testing.T) {
	e := testEditor(t, "vim", "one\ntwo\nthree\nfour\n")
	screen := newTerminalFrame(130, 30)
	v := renderer{screen: screen}
	press(e, "j j space n ctrl+w v space z")
	v.draw(e)
	if len(v.panes) != 1 || len(v.tabs) != 0 || v.divider != -1 {
		t.Fatal("Zen did not hide chrome")
	}
	p := v.panes[0]
	x := p.x + p.gutter - 2
	if got, _, _ := screen.Get(x, p.y+2); got != "3" {
		t.Fatalf("current line number: %q", got)
	}
	if got, _, _ := screen.Get(x, p.y+3); got != "1" {
		t.Fatalf("relative line number: %q", got)
	}
	press(e, "space z")
	v.draw(e)
	if len(v.panes) != 2 || len(v.tabs) != 1 || v.divider < 0 {
		t.Fatal("Zen exit lost workspace")
	}
}
