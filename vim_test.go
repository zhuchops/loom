package main

import (
	"testing"
)

func TestVimVisualLine(t *testing.T) {
	e := testEditor(t, "vim", "one\r\ntwo\r\nthree\r\n")
	press(e, "l V j o y G p")
	if got := string(e.b.text); got != "one\r\ntwo\r\nthree\r\none\r\ntwo\r\n" || !e.linewise {
		t.Fatalf("linewise yank/put: %q", got)
	}
	press(e, "g g v V j x")
	if got := string(e.b.text); got != "three\r\none\r\ntwo\r\n" {
		t.Fatalf("linewise delete: %q", got)
	}
	press(e, "V j c Z esc")
	if got := string(e.b.text); got != "Z\r\ntwo\r\n" {
		t.Fatalf("linewise change: %q", got)
	}
	press(e, "V V")
	if e.mode != "normal" || e.b.visualLine {
		t.Fatal("V did not exit linewise mode")
	}
}

func TestVimCommonCommands(t *testing.T) {
	for _, tc := range []struct{ before, keys, want string }{
		{"hello world", "2 r x", "xxllo world"},
		{"one\n  two\nthree", "3 J", "one two three"},
		{"one\r\n  two\r\nthree", "l 2 D", "o\r\nthree"},
		{"  one\n  two", "^ C Z esc", "  Z\n  two"},
		{"one\ntwo", "Y j p", "one\ntwo\none"},
		{"a,b,c,d", "f , ; t , d f d", "a,b,"},
		{"a-b-c-d", "2 f - , D", "a"},
		{"one two three", "y i w w v i w P", "one one three"},
		{"one\ntwo\nthree\nfour\nfive", "2 d 2 d", "five"},
		{"one\ntwo\nthree", "2 G d g g", "three"},
		{"\r\nnext", "d $", "\r\nnext"},
		{"item(a[b]) + more", "f ( d %", "item + more"},
	} {
		t.Run(tc.keys, func(t *testing.T) {
			e := testEditor(t, "vim", tc.before)
			press(e, tc.keys)
			if got := string(e.b.text); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMiniSurround(t *testing.T) {
	e := testEditor(t, "vim", "one two")
	press(e, "s a i w ( w .")
	if got := string(e.b.text); got != "( ( one ) ) two" {
		t.Fatalf("add and repeat: %q", got)
	}
	press(e, "f o 2 s r ( ]")
	if got := string(e.b.text); got != "[( one )] two" {
		t.Fatalf("replace outer pair: %q", got)
	}
	press(e, "s f ] s h ]")
	if e.b.cursor != 8 || e.surroundFlash.outer != (span{0, 9}) {
		t.Fatal("find/highlight did not target the outer pair")
	}
	press(e, "s F ] s d ]")
	if got := string(e.b.text); got != "( one ) two" {
		t.Fatalf("delete: %q", got)
	}
	press(e, "V s a } u")
	if got := string(e.b.text); got != "( one ) two" {
		t.Fatalf("linewise surround did not undo in one step: %q", got)
	}
}

func TestMiniSurroundLinewiseKeepsNewline(t *testing.T) {
	e := testEditor(t, "vim", "one\r\ntwo\r\n")
	press(e, "V j s a }")
	if got := string(e.b.text); got != "{one\r\ntwo}\r\n" {
		t.Fatalf("got %q", got)
	}
}

func TestVimVisualAndSurroundRendering(t *testing.T) {
	e := testEditor(t, "vim", "(one)\n\ntwo")
	e.zen = true
	screen := newTerminalFrame(45, 10)
	v := renderer{screen: screen}
	press(e, "V j")
	v.draw(e)
	for _, x := range []int{0, 1, 2, 3, 44} {
		_, gutter, _ := screen.Get(x, 0)
		if gutter.GetBackground() != selectStyle {
			t.Fatalf("linewise selection missed column %d", x)
		}
	}
	_, emptyRow, _ := screen.Get(8, 1)
	if emptyRow.GetBackground() != selectStyle {
		t.Fatal("linewise selection missed an empty row")
	}
	press(e, "esc g g s h )")
	v.draw(e)
	_, delimiter, _ := screen.Get(4, 0)
	_, word, _ := screen.Get(5, 0)
	if delimiter.GetBackground() == word.GetBackground() {
		t.Fatal("surround did not highlight delimiters")
	}
}
