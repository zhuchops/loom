package editor

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestWheelKeepsOneAxisPerSwipe(t *testing.T) {
	e := testEditor(t, "vim", strings.Repeat(strings.Repeat("x", 300)+"\n", 100))
	v := &renderer{screen: newTerminalFrame(80, 24)}
	v.draw(e)
	p := v.panes[0]
	at := func(b pointerButton) { v.mouse(e, pointerEvent{x: p.x + 5, y: p.y + 2, button: b}) }
	at(wheelDown)
	at(wheelRight)
	at(wheelDown)
	at(wheelRight)
	if e.b.left != 0 || e.b.top == 0 {
		t.Fatalf("a vertical swipe leaked sideways: top %d left %d", e.b.top, e.b.left)
	}
	time.Sleep(wheelGesture + 20*time.Millisecond)
	top := e.b.top
	at(wheelRight)
	at(wheelDown)
	at(wheelRight)
	if e.b.left == 0 || e.b.top != top {
		t.Fatalf("a horizontal swipe leaked vertically: top %d left %d", e.b.top, e.b.left)
	}
}

func TestWheelScrollsTabs(t *testing.T) {
	e := testEditor(t, "vim", "x\n")
	for i := 0; i < 12; i++ {
		e.buffers = append(e.buffers, &buffer{path: fmt.Sprintf("/tmp/file%02d.go", i)})
	}
	v := &renderer{screen: newTerminalFrame(80, 24)}
	v.draw(e)
	first := v.tabs[0].b
	v.mouse(e, pointerEvent{x: 30, y: 0, button: wheelDown})
	v.draw(e)
	if v.tabs[0].b == first {
		t.Fatal("wheel over the tab bar did not scroll the tabs")
	}
	if e.b != e.buffers[0] {
		t.Fatal("scrolling tabs changed the active buffer")
	}
}
