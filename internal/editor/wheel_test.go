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

func TestTallHoverNeverCoversThePointer(t *testing.T) {
	e := testEditor(t, "vim", "hello\nworld\n")
	v := &renderer{screen: newTerminalFrame(100, 30)}
	v.draw(e)
	long := strings.Repeat("documentation line\n", 80)
	for _, y := range []int{2, 15, 28} {
		v.diagnosticHover = diagnosticHover{buffer: e.b, version: e.b.version, x: 20, y: y, message: long}
		v.draw(e)
		if v.diagnosticBox.h == 0 {
			t.Fatalf("no hover drawn for pointer row %d", y)
		}
		if v.diagnosticBox.contains(20, y) || v.diagnosticBox.h > 15 {
			t.Fatalf("hover covers the pointer or is too tall at row %d: %+v", y, v.diagnosticBox)
		}
	}
}

func TestNewlineIndentsAfterOpeners(t *testing.T) {
	cases := []struct{ in, want string }{
		{"func a() {", "func a() {\n\t"},
		{"x := f(", "x := f(\n\t"},
		{"\tif y {", "\tif y {\n\t\t"},
		{"x := []int{}", "x := []int{\n\t\n}"},
		{"y := f()", "y := f()\n"},
	}
	for _, c := range cases {
		e := testEditor(t, "vim", c.in)
		e.b.path = "/tmp/x.go"
		e.mode = "insert"
		e.b.cursor = len(c.in)
		if c.in == "x := []int{}" {
			e.b.cursor = len(c.in) - 1
		}
		e.insertNewline()
		if got := string(e.b.text); got != c.want {
			t.Errorf("%q: got %q want %q", c.in, got, c.want)
		}
	}
}
