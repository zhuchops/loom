package editor

import "testing"

func TestVimShiftNormal(t *testing.T) {
	e := testEditor(t, "vim", "one\ntwo\nthree")
	press(e, "> >")
	if got := string(e.b.text); got != "    one\ntwo\nthree" {
		t.Fatalf(">>: %q", got)
	}
	press(e, "2 < <")
	if got := string(e.b.text); got != "one\ntwo\nthree" {
		t.Fatalf("2<<: %q", got)
	}
	press(e, "2 > >")
	if got := string(e.b.text); got != "    one\n    two\nthree" {
		t.Fatalf("2>>: %q", got)
	}
	press(e, "u")
	if got := string(e.b.text); got != "one\ntwo\nthree" {
		t.Fatalf("undo: %q", got)
	}
}

func TestVimShiftSelectionKeepsSelection(t *testing.T) {
	e := testEditor(t, "vim", "one\ntwo\nthree")
	press(e, "V j >")
	if e.mode != "select" || !e.b.visualLine || e.b.anchor < 0 {
		t.Fatalf("selection was reset: mode=%s linewise=%v anchor=%d", e.mode, e.b.visualLine, e.b.anchor)
	}
	if got := string(e.b.text); got != "    one\n    two\nthree" {
		t.Fatalf("indent: %q", got)
	}
	press(e, "j")
	if e.mode != "select" || e.b.anchor < 0 {
		t.Fatal("move cleared the selection")
	}
	press(e, "> 2 >")
	if got := string(e.b.text); got != "                one\n                two\n            three" {
		t.Fatalf("repeat indent with count: %q", got)
	}
	press(e, "esc")
	if e.mode != "normal" || e.b.anchor >= 0 {
		t.Fatal("esc did not remove the selection")
	}
}

func TestVimOutdentSelection(t *testing.T) {
	e := testEditor(t, "vim", "    one\n\ttwo\nthree")
	press(e, "V j <")
	if got := string(e.b.text); got != "one\ntwo\nthree" {
		t.Fatalf("outdent: %q", got)
	}
	if e.mode != "select" || !e.b.visualLine {
		t.Fatal("outdent cleared the selection")
	}
}
