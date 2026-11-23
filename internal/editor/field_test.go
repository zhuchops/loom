package editor

import "testing"

func TestTextFieldSelects(t *testing.T) {
	e := testEditor(t, "vim", "")
	e.openMenu(&menu{title: "Rename Symbol", input: "fetchUserName", submit: func(string) {}})
	m := e.menu
	in := func(k string, shift bool) { e.handle(keyInput{key: k, shift: shift}) }
	in("alt+left", true) // selects Name? no: words are runs of letters, so the whole name
	if got := m.field.selected(m.input); got != "fetchUserName" {
		t.Fatalf("shift+alt+left selected %q", got)
	}
	in("right", false) // drops the selection at its end
	in("left", true)
	in("left", true)
	in("left", true)
	in("left", true)
	if got := m.field.selected(m.input); got != "Name" {
		t.Fatalf("shift+left selected %q", got)
	}
	in("I", false)
	in("d", false)
	if m.input != "fetchUserId" {
		t.Fatalf("typing over a selection: %q", m.input)
	}
	in("home", true)
	if got := m.field.selected(m.input); got != "fetchUserId" {
		t.Fatalf("shift+home selected %q", got)
	}
	in("backspace", false)
	if m.input != "" {
		t.Fatalf("backspace over a selection: %q", m.input)
	}
	e.handle(keyInput{paste: "one two\nthree"})
	if m.input != "one two three" {
		t.Fatalf("paste: %q", m.input)
	}
	in("alt+left", false)
	in("super+backspace", false)
	if m.input != "three" {
		t.Fatalf("Command+Delete: %q", m.input)
	}
	in("end", false)
	in("left", false)
	in("left", false)
	in("super+delete", false)
	if m.input != "thr" {
		t.Fatalf("Command+Fn+Delete: %q", m.input)
	}
	in("home", false)
	in("alt+delete", false)
	if m.input != "" {
		t.Fatalf("alt+delete: %q", m.input)
	}
}

func TestTextFieldMouseInTheTerminal(t *testing.T) {
	e := testEditor(t, "vim", "")
	e.openMenu(&menu{title: "Rename Symbol", input: "fetch user name", submit: func(string) {}})
	m := e.menu
	v := renderer{screen: newTerminalFrame(80, 20)}
	v.draw(e)
	if len(v.fields) != 1 {
		t.Fatalf("fields drawn: %d", len(v.fields))
	}
	f := v.fields[0]
	mouse := func(x int, down bool) {
		v.mouse(e, pointerEvent{x: f.x + x, y: f.y, leftDown: down})
		v.draw(e)
	}
	mouse(6, true) // on the u of user
	mouse(6, false)
	if m.caret() != 6 {
		t.Fatalf("click put the caret at %d", m.caret())
	}
	m.field.clickAt = m.field.clickAt.AddDate(0, 0, -1) // not a double click
	mouse(6, true)
	mouse(9, true) // drag over "use"
	mouse(9, false)
	if got := m.field.selected(m.input); got != "use" {
		t.Fatalf("drag selected %q", got)
	}
	m.field.clickAt = m.field.clickAt.AddDate(0, 0, -1) // not a double click
	mouse(12, true)
	mouse(12, false)
	mouse(12, true) // a second quick click
	mouse(12, false)
	if got := m.field.selected(m.input); got != "name" {
		t.Fatalf("double click selected %q", got)
	}
	if e.menu != m {
		t.Fatal("clicking the field closed the menu")
	}
}

func TestFindBarFieldHasACaret(t *testing.T) {
	e := testEditor(t, "vim", "hello world\n")
	e.handle(keyInput{key: "/"})
	keys := func(names ...string) {
		for _, k := range names {
			e.handle(keyInput{key: k})
		}
	}
	keys("w", "r", "l", "d", "left", "left", "left", "o")
	if e.bar.find != "world" {
		t.Fatalf("typed in the middle of the find text: %q", e.bar.find)
	}
	e.closeFindBar(false)
	e.mode, e.b.anchor, e.b.cursor = "select", 0, 4
	e.openFindBar(true)
	if e.bar.find != "hello" || e.bar.findField.selected(e.bar.find) != "hello" {
		t.Fatalf("prefilled find text %q is not selected", e.bar.find)
	}
	keys("left", "x")
	if e.bar.find != "xhello" {
		t.Fatalf("an arrow should keep the prefilled text: %q", e.bar.find)
	}
}
