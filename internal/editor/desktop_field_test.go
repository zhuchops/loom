//go:build desktop

package editor

import (
	"image"
	"runtime"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

func TestDesktopTextFieldMouseAndShortcuts(t *testing.T) {
	s := openScreen(t)
	e, d := s.e, s.d
	e.openMenu(&menu{title: "Rename Symbol", input: "fetch user name", submit: func(string) {}})
	s.frame()
	m := e.menu
	var field image.Rectangle
	for _, z := range d.zones {
		if z.cursor == pointer.CursorText && z.click != nil {
			field = z.r
		}
	}
	if field.Empty() {
		t.Fatal("the picker field has no pointer zone")
	}
	cy := (field.Min.Y + field.Max.Y) / 2
	press := func(x int) {
		s.mouse(pointer.Move, x, cy, 0, f32.Point{})
		s.mouse(pointer.Press, x, cy, pointer.ButtonPrimary, f32.Point{})
	}
	release := func(x int) { s.mouse(pointer.Release, x, cy, 0, f32.Point{}) }

	press(field.Min.X + d.u(2)) // before the text
	release(field.Min.X + d.u(2))
	if m.caret() != 0 {
		t.Fatalf("click before the text put the caret at %d", m.caret())
	}
	m.field.clickAt = time.Time{}
	press(field.Min.X + d.u(2))
	s.mouse(pointer.Move, field.Max.X-d.u(2), cy, pointer.ButtonPrimary, f32.Point{})
	release(field.Max.X - d.u(2))
	if got := m.field.selected(m.input); got != "fetch user name" {
		t.Fatalf("dragging across selected %q", got)
	}
	s.typeText("x")
	if m.input != "x" {
		t.Fatalf("typing over the dragged selection: %q", m.input)
	}

	m.input = "fetch user name"
	m.field = lineField{}
	d.command("select-all")
	s.typeText("y")
	if m.input != "y" {
		t.Fatalf("Select All then typing: %q", m.input)
	}

	if runtime.GOOS == "darwin" {
		m.input = "fetch user name"
		m.field = lineField{}
		s.key(key.NameLeftArrow, key.ModAlt)
		s.key(key.NameDeleteBackward, key.ModCommand)
		if m.input != "name" {
			t.Fatalf("Command+Delete: %q", m.input)
		}
		s.key(key.NameRightArrow, key.ModCommand|key.ModShift)
		if got := m.field.selected(m.input); got != "name" {
			t.Fatalf("Shift+Command+Right selected %q", got)
		}
	}
}
