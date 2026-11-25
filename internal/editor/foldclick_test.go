package editor

import "testing"

func TestClickingTheFoldButtonFolds(t *testing.T) {
	e := testEditor(t, "vim", "func a() {\n\tx := 1\n\ty := 2\n}\nfunc b() {}\n")
	click := func(v *renderer, x, y int) {
		v.mouse(e, pointerEvent{x: x, y: y, leftDown: true})
		v.mouse(e, pointerEvent{x: x, y: y})
	}
	for _, native := range []bool{false, true} {
		e.b.folds.closed = nil
		v := &renderer{screen: newTerminalFrame(80, 24), native: native}
		v.draw(e)
		pane := v.panes[0]
		if len(pane.folds) != 1 || pane.folds[0].row != 0 || pane.folds[0].closed {
			t.Fatalf("fold buttons %+v, want one open fold on the first line", pane.folds)
		}
		x := pane.foldX
		click(v, x, pane.y)
		if !e.b.folds.closed[0] {
			t.Fatal("the click did not close the fold")
		}
		v.draw(e)
		if f := v.panes[0].folds; len(f) != 1 || !f[0].closed {
			t.Fatalf("fold buttons %+v after folding", f)
		}
		if mark, _, _ := v.screen.Get(x, pane.y); !native && mark != "▸" {
			t.Fatal("the terminal shows no closed-fold mark")
		}
		click(v, x, pane.y)
		if e.b.folds.closed[0] {
			t.Fatal("the second click did not open the fold")
		}
		// A click on a line without a fold places the cursor as usual.
		v.draw(e)
		click(v, x, pane.y+2)
		if e.b.folds.closed[2] || e.b.rowAt(e.b.cursor) != 2 {
			t.Fatalf("a click beside a line moved the cursor to row %d", e.b.rowAt(e.b.cursor))
		}
	}
}
