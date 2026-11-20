package editor

import (
	"path/filepath"
	"testing"
)

func TestDashboardActionsAndFileTabs(t *testing.T) {
	e := testEditor(t, "vim", "")
	e.showDashboard()
	dashboard := e.b
	if len(e.buffers) != 1 || !dashboard.dashboard {
		t.Fatal("dashboard did not replace the empty startup tab")
	}
	press(e, "i V x")
	e.handle(keyInput{paste: "hello"})
	target := filepath.Join(e.root, "dashboard.txt")
	e.ex("w " + target)
	if dashboard.dirty() || dashboard.path != "" || e.mode != "normal" {
		t.Fatal("dashboard accepted a text edit or save")
	}
	press(e, "j enter")
	if e.menu == nil || e.menu.title != "Search Project · literal text" {
		t.Fatal("dashboard action did not open project search")
	}
	press(e, "esc")
	path := fixture(t, e.root, "main.txt", "first\nsecond")
	if !e.open(path) || len(e.buffers) != 2 || e.buffers[0] != dashboard || e.b.path != path {
		t.Fatal("opening a file did not keep the dashboard as tab 1")
	}
	press(e, "space t 1")
	v := renderer{screen: newTerminalFrame(100, 20)}
	v.draw(e)
	box := e.dashboardBox(v.panes[0].hitBox)
	v.mouse(e, pointerEvent{x: box.x, y: box.y + 2, leftDown: true})
	if e.menu == nil || e.menu.title != "Open Buffers" {
		t.Fatal("clicking the dashboard did not open the buffer picker")
	}
	press(e, "esc")
	e.closeBuffer(dashboard)
	if e.b.path != path {
		t.Fatal("closing the dashboard did not return to the file")
	}
}

func TestDashboardPreservesRestoredFileViews(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	e := testEditor(t, "vim", "")
	first := fixture(t, e.root, "one.txt", "one\ntwo")
	second := fixture(t, e.root, "two.txt", "first\nsecond")
	e.open(first)
	e.splitPane(true, second)
	press(e, "j")
	e.saveSession()

	e.restoreSession(false)
	e.showDashboard()
	if e.buffers[0] != e.b || len(e.buffers) != 3 || len(e.layout.leaves()) != 2 {
		t.Fatal("dashboard launch lost restored tabs or splits")
	}
	e.saveSession()
	e.restoreSession(false)
	if len(e.layout.leaves()) != 2 || e.b.path != second || e.b.rowAt(e.b.cursor) != 1 {
		t.Fatal("quitting from the dashboard lost the underlying file view")
	}
}
