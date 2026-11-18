package editor

import (
	"strings"
	"testing"
)

func TestSlashSearchConfirmsFirstMatch(t *testing.T) {
	e := testEditor(t, "vim", "alpha\nbeta\nalpha beta\n")
	press(e, "/ b e t a enter")
	if e.b.cursor != 6 || e.bar != nil || !e.searchVisible {
		t.Fatalf("cursor %d bar %v", e.b.cursor, e.bar)
	}
	press(e, "n")
	if e.b.cursor != 17 {
		t.Fatalf("n went to %d", e.b.cursor)
	}
	press(e, "N")
	if e.b.cursor != 6 {
		t.Fatalf("N went to %d", e.b.cursor)
	}
}

func TestFindBarIsCaseInsensitiveUntilToggled(t *testing.T) {
	e := testEditor(t, "vim", "Foo foo FOO")
	press(e, "/ f o o")
	if n := len(e.matches(e.b).spans); n != 3 {
		t.Fatalf("matches %d", n)
	}
	press(e, "ctrl+t")
	if n := len(e.matches(e.b).spans); n != 1 {
		t.Fatalf("case sensitive matches %d", n)
	}
	press(e, "ctrl+t ctrl+o")
	if n := len(e.matches(e.b).spans); n != 3 {
		t.Fatalf("word matches %d", n)
	}
}

func TestFindBarReplaceCurrentAndAll(t *testing.T) {
	e := testEditor(t, "vim", "cat dog cat\ncat\n")
	press(e, "space h")
	if e.bar == nil || !e.bar.showReplace {
		t.Fatal("replace bar did not open")
	}
	// The word under the cursor ("cat") is prefilled.
	if e.bar.find != "cat" {
		t.Fatalf("prefill %q", e.bar.find)
	}
	press(e, "tab b i r d")
	if e.bar.find != "cat" {
		t.Fatal("typing in the replace field must not touch the find text")
	}
	press(e, "enter")
	if got := string(e.b.text); got != "bird dog cat\ncat\n" {
		t.Fatalf("replace current: %q", got)
	}
	if e.b.cursor != 9 {
		t.Fatalf("cursor should sit on the next match, got %d", e.b.cursor)
	}
	press(e, "ctrl+a")
	if got := string(e.b.text); got != "bird dog bird\nbird\n" {
		t.Fatalf("replace all: %q", got)
	}
	press(e, "esc u")
	if got := string(e.b.text); got != "bird dog cat\ncat\n" {
		t.Fatalf("replace all is not one undo step: %q", got)
	}
}

func TestFindBarRegexReplaceWithGroups(t *testing.T) {
	e := testEditor(t, "vim", "a=1\nb=2\n")
	press(e, "space h ctrl+u")
	typeText(e, `(\w)=(\d)`)
	press(e, "ctrl+e tab")
	typeText(e, "$2:$1")
	press(e, "ctrl+a")
	if got := string(e.b.text); got != "1:a\n2:b\n" {
		t.Fatalf("got %q", got)
	}
}

func TestFindBarShowsCountAndInvalidPattern(t *testing.T) {
	e := testEditor(t, "vim", "one two one")
	press(e, "space h ctrl+u o n e")
	v := renderer{screen: newTerminalFrame(100, 12)}
	v.draw(e)
	dump := screenDump(v.screen)
	if !strings.Contains(dump, "1 of 2") || !strings.Contains(dump, "Find and Replace") {
		t.Fatalf("bar not drawn:\n%s", dump)
	}
	press(e, "ctrl+u ctrl+e ( ")
	v.draw(e)
	if !strings.Contains(screenDump(v.screen), "Invalid") {
		t.Fatal("invalid pattern not reported")
	}
	// Clicking the close button closes the bar.
	for _, h := range v.barHits {
		if h.action == "close" {
			v.mouse(e, pointerEvent{x: h.x + 1, y: h.y, leftDown: true})
			v.mouse(e, pointerEvent{x: h.x + 1, y: h.y})
		}
	}
	if e.bar != nil {
		t.Fatal("close button did nothing")
	}
}

func typeText(e *editor, text string) {
	for _, r := range text {
		e.handle(keyInput{key: string(r)})
	}
}

func TestPrefilledFindTextIsReplacedByTyping(t *testing.T) {
	e := testEditor(t, "vim", "word other")
	press(e, "space h")
	typeText(e, "other")
	if e.bar.find != "other" {
		t.Fatalf("find %q", e.bar.find)
	}
	typeText(e, "s")
	if e.bar.find != "others" {
		t.Fatalf("second keystroke should append, got %q", e.bar.find)
	}
}
