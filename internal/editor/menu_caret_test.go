package editor

import "testing"

func TestMenuInputHasACaret(t *testing.T) {
	e := testEditor(t, "vim", "")
	var got string
	e.openMenu(&menu{title: "Rename Symbol", input: "userName", submit: func(s string) { got = s }})
	keys := func(names ...string) {
		for _, k := range names {
			e.handle(keyInput{key: k})
		}
	}
	keys("left", "left", "left", "left", "X") // user|Name
	if e.menu.input != "userXName" {
		t.Fatalf("typed in the middle: %q", e.menu.input)
	}
	keys("backspace", "delete") // user|ame
	if e.menu.input != "userame" {
		t.Fatalf("backspace and delete: %q", e.menu.input)
	}
	keys("home", "o", "end", "s")
	if e.menu.input != "ouserames" {
		t.Fatalf("home and end: %q", e.menu.input)
	}
	e.menu.input = "fetch user name"
	keys("end", "alt+left", "alt+left", "alt+backspace")
	if e.menu.input != "user name" || e.menu.caret() != 0 {
		t.Fatalf("word keys: %q caret %d", e.menu.input, e.menu.caret())
	}
	keys("alt+right", "ctrl+k")
	if e.menu.input != "user" {
		t.Fatalf("ctrl+k: %q", e.menu.input)
	}
	e.handle(keyInput{paste: "Id"})
	keys("left", "left", "left", "left", "right", "right", "right", "right", "right", "right", "right", "enter")
	if got != "userId" {
		t.Fatalf("submitted %q", got)
	}
}

func TestMenuCaretIsDrawnWhereItIs(t *testing.T) {
	e := testEditor(t, "vim", "")
	e.openMenu(&menu{title: "Rename", input: "abcdef", submit: func(string) {}})
	v := renderer{screen: newTerminalFrame(80, 20)}
	v.draw(e)
	end := v.screen.cursorX
	e.handle(keyInput{key: "left"})
	e.handle(keyInput{key: "left"})
	v.draw(e)
	if !v.screen.cursorVisible || v.screen.cursorX != end-2 {
		t.Fatalf("caret drawn at %d, want %d", v.screen.cursorX, end-2)
	}
}
