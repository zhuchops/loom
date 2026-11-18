package editor

import "testing"

func TestDragTabIntoSplit(t *testing.T) {
	e := testEditor(t, "vim", "")
	a := fixture(t, e.root, "a.txt", "alpha")
	b := fixture(t, e.root, "b.txt", "beta")
	e.open(a)
	e.open(b)
	v := renderer{screen: newTerminalFrame(100, 20)}
	v.draw(e)
	if len(v.tabs) < 2 {
		t.Fatal("expected tabs")
	}
	tab := v.tabs[0]
	pane := v.panes[0]
	// Press the first tab, drag to the right edge of the pane, release.
	v.mouse(e, pointerEvent{x: tab.x + 2, y: 0, leftDown: true})
	v.mouse(e, pointerEvent{x: pane.x + pane.w - 3, y: pane.y + pane.h/2, leftDown: true})
	if v.drop.zone != "right" {
		t.Fatalf("zone %q", v.drop.zone)
	}
	v.draw(e)
	v.mouse(e, pointerEvent{x: pane.x + pane.w - 3, y: pane.y + pane.h/2})
	leaves := e.layout.leaves()
	if len(leaves) != 2 || leaves[0].b.path != b || leaves[1].b.path != a || e.activePane != leaves[1] {
		t.Fatalf("right drop wrong: %d leaves", len(leaves))
	}
	// Drag the tab for b onto the left edge of the right pane.
	v.draw(e)
	tab = v.tabs[1]
	right := v.panes[1]
	v.mouse(e, pointerEvent{x: tab.x + 2, y: 0, leftDown: true})
	v.mouse(e, pointerEvent{x: right.x + 1, y: right.y + right.h/2, leftDown: true})
	v.mouse(e, pointerEvent{x: right.x + 1, y: right.y + right.h/2})
	leaves = e.layout.leaves()
	if len(leaves) != 3 || leaves[1].b.path != b {
		t.Fatalf("left drop wrong: %d leaves", len(leaves))
	}
}
