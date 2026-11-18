package editor

import "testing"

func TestResizePanesWithKeys(t *testing.T) {
	e := testEditor(t, "vim", "hello")
	press(e, "ctrl+w v")
	v := renderer{screen: newTerminalFrame(100, 20)}
	v.draw(e)
	if len(v.paneDividers) != 1 {
		t.Fatalf("dividers %d", len(v.paneDividers))
	}
	before := v.paneDividers[0].box.x
	// The active pane is the right one, so widening it moves the divider left.
	press(e, "ctrl+w >")
	v.draw(e)
	if after := v.paneDividers[0].box.x; after >= before {
		t.Fatalf("divider %d -> %d, want it to move left", before, after)
	}
	press(e, "ctrl+w < ctrl+w <")
	v.draw(e)
	if v.paneDividers[0].box.x <= before {
		t.Fatal("narrowing did not move the divider right")
	}
	press(e, "ctrl+w =")
	v.draw(e)
	if v.paneDividers[0].box.x != before {
		t.Fatal("equalize did not restore the even split")
	}
	press(e, "ctrl+w s ctrl+w +")
	v.draw(e)
	if len(v.paneDividers) != 2 {
		t.Fatalf("dividers %d", len(v.paneDividers))
	}
}

func TestResizePanesWithMouse(t *testing.T) {
	e := testEditor(t, "vim", "hello")
	press(e, "ctrl+w v")
	v := renderer{screen: newTerminalFrame(100, 20)}
	v.draw(e)
	d := v.paneDividers[0].box
	v.mouse(e, pointerEvent{x: d.x, y: d.y + 2, leftDown: true})
	v.mouse(e, pointerEvent{x: d.x + 15, y: d.y + 2, leftDown: true})
	v.mouse(e, pointerEvent{x: d.x + 15, y: d.y + 2})
	v.draw(e)
	if got := v.paneDividers[0].box.x; got < d.x+14 || got > d.x+16 {
		t.Fatalf("divider at %d, want about %d", got, d.x+15)
	}
	// Dragging far past the edge keeps a minimum width.
	d = v.paneDividers[0].box
	v.mouse(e, pointerEvent{x: d.x, y: d.y + 2, leftDown: true})
	v.mouse(e, pointerEvent{x: 400, y: d.y + 2, leftDown: true})
	v.mouse(e, pointerEvent{x: 400, y: d.y + 2})
	v.draw(e)
	right := v.panes[len(v.panes)-1]
	if right.w < minPaneWidth {
		t.Fatalf("right pane shrank to %d", right.w)
	}
	// A double-click evens the split out again.
	d = v.paneDividers[0].box
	v.mouse(e, pointerEvent{x: d.x, y: d.y + 2, leftDown: true})
	v.mouse(e, pointerEvent{x: d.x, y: d.y + 2})
	v.mouse(e, pointerEvent{x: d.x, y: d.y + 2, leftDown: true})
	v.mouse(e, pointerEvent{x: d.x, y: d.y + 2})
	if e.layout.ratio != 0 {
		t.Fatalf("ratio %d after double-click", e.layout.ratio)
	}
}

func TestPaneRatioSurvivesSession(t *testing.T) {
	root := &pane{first: &pane{b: &buffer{path: "/a"}}, second: &pane{b: &buffer{path: "/b"}}, vertical: true, ratio: 300}
	saved := savePaneLayout(root, root.first)
	if saved.Ratio != 300 {
		t.Fatalf("saved ratio %d", saved.Ratio)
	}
	buffers := map[string]*buffer{"/a": {text: nil, lines: []int{0}}, "/b": {text: nil, lines: []int{0}}}
	restored, _ := restorePaneLayout(saved, buffers, 0)
	if restored.ratio != 300 {
		t.Fatalf("restored ratio %d", restored.ratio)
	}
}
