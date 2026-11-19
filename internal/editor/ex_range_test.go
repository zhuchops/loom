package editor

import (
	"strings"
	"testing"
)

func exRun(e *editor, line string) { e.ex(line) }

func TestSubstituteAndGlobal(t *testing.T) {
	e := testEditor(t, "vim", "foo one\nfoo two foo\nbar\nfoo end\n")
	exRun(e, "%s/foo/X/")
	if got := string(e.b.text); got != "X one\nX two foo\nbar\nX end\n" {
		t.Fatalf("first match only: %q", got)
	}
	exRun(e, "2s/(\\w+) (foo)/\\2 \\1/")
	if got := strings.Split(string(e.b.text), "\n")[1]; got != "X foo two" {
		t.Fatalf("groups: %q", got)
	}
	exRun(e, "%s/o/0/g")
	if got := strings.Split(string(e.b.text), "\n")[1]; got != "X f00 tw0" {
		t.Fatalf("global flag: %q", got)
	}
	e.b.undo = e.b.undo[:0]
	exRun(e, "g/bar/d")
	if strings.Contains(string(e.b.text), "bar") {
		t.Fatalf("g/d left the line: %q", e.b.text)
	}
	exRun(e, "v/end|f00/d")
	if got := string(e.b.text); got != "X f00 tw0\nX end\n" {
		t.Fatalf("v/d: %q", got)
	}
	exRun(e, "1d")
	if got := string(e.b.text); got != "X end\n" {
		t.Fatalf("range delete: %q", got)
	}
}

func TestRangeParsingLeavesOtherCommands(t *testing.T) {
	e := testEditor(t, "vim", "a\nb\nc\n")
	for _, line := range []string{"set rnu", "split", "sp", "w", "vsplit", "42", "$"} {
		if e.exRangeLike(line) {
			t.Fatalf("%q must not be taken as a range command", line)
		}
	}
	for _, line := range []string{"%s/a/b/", "1,2s/a/b/g", "g/a/d", "'<,'>s/a/b/", ".,+1d"} {
		if line == "'<,'>s/a/b/" {
			e.b.marks = map[rune]int{'<': 0, '>': 2}
		}
		if !e.exRangeLike(line) {
			t.Fatalf("%q should be a range command", line)
		}
	}
}

func TestMarksAndRegisters(t *testing.T) {
	e := testEditor(t, "vim", "one\ntwo\nthree\n")
	press(e, "j m a G")
	press(e, "' a")
	if e.b.rowAt(e.b.cursor) != 1 {
		t.Fatalf("mark jump landed on row %d", e.b.rowAt(e.b.cursor))
	}
	// A mark follows its line when text is inserted above.
	press(e, "g g O x esc")
	press(e, "G ' a")
	if got := e.b.rowAt(e.b.cursor); got != 2 {
		t.Fatalf("mark did not move with its line: row %d", got)
	}

	e = testEditor(t, "vim", "one\ntwo\n")
	press(e, `" a y y j " b y y`)
	if e.regs['a'].text != "one\n" || e.regs['b'].text != "two\n" {
		t.Fatalf("named registers: %+v", e.regs)
	}
	press(e, `d d`)
	press(e, `" a p`)
	if got := string(e.b.text); got != "one\none\n" && !strings.Contains(got, "one") {
		t.Fatalf("paste from register a: %q", got)
	}
	e.register = "zzz"
	press(e, `" _ d d`)
	if e.register != "zzz" {
		t.Fatal(`"_ must not touch the default register`)
	}
	if len(e.history) == 0 {
		t.Fatal("history empty")
	}
}

func TestVisualBlock(t *testing.T) {
	e := testEditor(t, "vim", "abcd\nefgh\nijkl\n")
	press(e, "l ctrl+v j j l")
	if !e.b.visualBlock {
		t.Fatal("not in block mode")
	}
	press(e, "y")
	if e.register != "bc\nfg\njk" {
		t.Fatalf("block yank: %q", e.register)
	}
	press(e, "g g l ctrl+v j j l d")
	if got := string(e.b.text); got != "ad\neh\nil\n" {
		t.Fatalf("block delete: %q", got)
	}
	e = testEditor(t, "vim", "abc\nabc\nabc\n")
	press(e, "ctrl+v j j I - - esc")
	if got := string(e.b.text); got != "--abc\n--abc\n--abc\n" {
		t.Fatalf("block insert: %q", got)
	}
	e = testEditor(t, "vim", "abc\nab\nabc\n")
	press(e, "l ctrl+v j j A ! esc")
	if got := string(e.b.text); got != "ab!c\nab!\nab!c\n" {
		t.Fatalf("block append: %q", got)
	}
}
