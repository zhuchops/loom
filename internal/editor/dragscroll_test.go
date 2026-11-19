package editor

import (
	"strings"
	"testing"
)

func TestDragSelectionScrollsAtEdges(t *testing.T) {
	e := testEditor(t, "vim", strings.Repeat(strings.Repeat("x", 200)+"\n", 100))
	v := &renderer{screen: newTerminalFrame(80, 24)}
	v.draw(e)
	pane := v.panes[0]
	v.mouse(e, pointerEvent{x: pane.x + pane.gutter + 2, y: pane.y + 2, leftDown: true})
	before := e.b.top
	// Resting on the bottom row keeps scrolling down.
	for range 5 {
		v.mouse(e, pointerEvent{x: pane.x + pane.gutter + 2, y: pane.y + pane.h - 1, leftDown: true})
	}
	if e.b.top <= before {
		t.Fatalf("view did not scroll down: top %d", e.b.top)
	}
	for range 5 {
		v.mouse(e, pointerEvent{x: pane.x + pane.w - 1, y: pane.y + 3, leftDown: true})
	}
	if e.b.left == 0 {
		t.Fatalf("view did not scroll right")
	}
}
