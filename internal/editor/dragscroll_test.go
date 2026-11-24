package editor

import (
	"strings"
	"testing"
	"time"
)

// dragClock is a renderer clock that a test moves by hand.
type dragClock struct{ t time.Time }

func (c *dragClock) now() time.Time { return c.t }

func TestDragSelectionScrollsAtEdges(t *testing.T) {
	e := testEditor(t, "vim", strings.Repeat(strings.Repeat("x", 200)+"\n", 300))
	clock := &dragClock{t: time.Unix(1000, 0)}
	v := &renderer{screen: newTerminalFrame(80, 24), clock: clock.now}
	v.draw(e)
	pane := v.panes[0]
	x := pane.x + pane.gutter + 2
	drag := func(x, y int, wait time.Duration) {
		clock.t = clock.t.Add(wait)
		v.mouse(e, pointerEvent{x: x, y: y, leftDown: true})
	}
	drag(x, pane.y+2, 0)

	// A terminal ends at its last row, so resting there scrolls, after a
	// short pause, at a few lines a second.
	bottom := pane.y + pane.h - 1
	drag(x, bottom, 0)
	drag(x, bottom, 100*time.Millisecond)
	if e.b.top != 0 {
		t.Fatalf("scrolled before the pause: top %d", e.b.top)
	}
	for range 16 { // a second, in drag ticks
		drag(x, bottom, 60*time.Millisecond)
	}
	if e.b.top < 4 || e.b.top > 12 {
		t.Fatalf("a second at the edge scrolled %d lines, want about 8", e.b.top)
	}

	for range 5 {
		drag(pane.x+pane.w-1, pane.y+3, 60*time.Millisecond)
	}
	if e.b.left == 0 {
		t.Fatalf("view did not scroll right")
	}
}

func TestDragSelectionScrollsFasterFurtherOut(t *testing.T) {
	speed := func(out int) int {
		e := testEditor(t, "vim", strings.Repeat("line\n", 2000))
		clock := &dragClock{t: time.Unix(1000, 0)}
		v := &renderer{screen: newTerminalFrame(80, 24), native: true, clock: clock.now}
		v.draw(e)
		pane := v.panes[0]
		x := pane.x + pane.gutter + 2
		v.mouse(e, pointerEvent{x: x, y: pane.y + 2, leftDown: true})
		y := pane.y + pane.h - 1 + out
		for range 30 { // 1.8 seconds
			clock.t = clock.t.Add(60 * time.Millisecond)
			v.mouse(e, pointerEvent{x: x, y: y, leftDown: true})
		}
		return e.b.top
	}
	// In a window the pointer has to leave the text: its last row is not enough.
	if n := speed(0); n != 0 {
		t.Fatalf("scrolled %d lines with the pointer still on the text", n)
	}
	near, far := speed(1), speed(5)
	if near == 0 || far <= 2*near {
		t.Fatalf("one row out scrolled %d lines, five rows out %d", near, far)
	}
}
