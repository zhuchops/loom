package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVimExtras(t *testing.T) {
	e := testEditor(t, "vim", "foo bar\nfoo\n\nbaz foo\n")
	press(e, "*")
	if e.b.cursor != 8 {
		t.Fatalf("* went to %d", e.b.cursor)
	}
	press(e, "#")
	if e.b.cursor != 0 {
		t.Fatalf("# went to %d", e.b.cursor)
	}
	press(e, "}")
	if e.b.rowAt(e.b.cursor) != 2 {
		t.Fatalf("} went to row %d", e.b.rowAt(e.b.cursor))
	}
	press(e, "{")
	if e.b.rowAt(e.b.cursor) != 0 {
		t.Fatalf("{ went to row %d", e.b.rowAt(e.b.cursor))
	}
	press(e, "~ ~ ~")
	if got := strings.Split(string(e.b.text), "\n")[0]; got != "FOO bar" {
		t.Fatalf("~: %q", got)
	}
	press(e, "v l esc g v")
	if e.mode != "select" {
		t.Fatal("gv did not reselect")
	}
	press(e, "U")
	if e.mode != "normal" {
		t.Fatal("U should end the selection")
	}
	e.viewRows = 5
	press(e, "G H")
	if e.b.rowAt(e.b.cursor) != e.b.top {
		t.Fatalf("H: row %d top %d", e.b.rowAt(e.b.cursor), e.b.top)
	}
}

func TestScopesAndBreadcrumbs(t *testing.T) {
	e := testEditor(t, "vim", "package x\n\nfunc (s *S) Handle() {\n\tif ok {\n\t\twork()\n\t}\n}\n")
	e.b.cursor = e.b.lines[4]
	if got := e.breadcrumbs(e.b); got != "Handle › if ok" {
		t.Fatalf("breadcrumbs: %q", got)
	}
}

func TestDeepScopesKeepTheFunction(t *testing.T) {
	src := "func createUser() {\n"
	for i := range 6 {
		src += strings.Repeat("\t", i+1) + "if ok {\n"
	}
	src += strings.Repeat("\t", 7) + "work()\n"
	e := testEditor(t, "vim", src)
	e.b.cursor = e.b.lines[7]
	if got := e.scopeTrail(e.b); len(got) != 4 || got[0] != "createUser" {
		t.Fatalf("trail: %v", got)
	}
}

func TestRecentAndJumps(t *testing.T) {
	e := testEditor(t, "vim", "x")
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	e.root = dir
	a, b := filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go")
	os.WriteFile(a, []byte("package a\n"), 0o644)
	os.WriteFile(b, []byte("package b\n"), 0o644)
	e.open(a)
	e.open(b)
	e.touchRecent(a)
	if len(e.recent) < 2 || e.recent[0] != a || e.recent[1] != b {
		t.Fatalf("recent order: %v", e.recent)
	}
	if opts := e.recentOptions(true); len(opts) == 0 || opts[0].label != "a.go" {
		t.Fatalf("options: %+v", opts)
	}
}

func TestProjectReplace(t *testing.T) {
	e := testEditor(t, "vim", "x")
	dir := t.TempDir()
	e.root = dir
	file := filepath.Join(dir, "a.txt")
	os.WriteFile(file, []byte("Hello world\nhello again\nnothing\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "bin.dat"), []byte("hello\x00world"), 0o644)
	re, regex, err := projectPattern("hello")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	e.walkProject(e.root, func(path string, text []byte) {
		if re.Match(text) {
			files[path] = true
		}
	})
	if len(files) != 1 || !files[file] {
		t.Fatalf("binary files must be skipped: %v", files)
	}
	e.applyProjectReplace(re, regex, "hi $1", files)
	raw, _ := os.ReadFile(file)
	if string(raw) != "hi $1 world\nhi $1 again\nnothing\n" {
		t.Fatalf("replaced: %q", raw)
	}
}

// Nothing may cover the code: every row of a pane shows its own line.
func TestNothingIsPinnedOverTheCode(t *testing.T) {
	var src strings.Builder
	src.WriteString("func Big() {\n\tif ok {\n\t\treturn run(func() {\n")
	for i := range 80 {
		fmt.Fprintf(&src, "\t\t\tstep%02d()\n", i)
	}
	src.WriteString("\t\t})\n\t}\n}\n")
	for _, scrolloff := range []int{0, 1, 5} {
		e := testEditor(t, "vim", src.String())
		e.cfg.Scrolloff = scrolloff
		v := &renderer{screen: newTerminalFrame(80, 24)}
		check := func(row int) {
			t.Helper()
			e.b.cursor = e.b.lines[row]
			v.draw(e)
			for _, pane := range v.panes {
				for i := range 3 {
					line := pane.b.top + i
					want := strings.TrimSpace(string(pane.b.text[pane.b.lines[line]:pane.b.lineEnd(line)]))
					var got strings.Builder
					for x := pane.x + pane.gutter; x < pane.x+pane.w; x++ {
						c, _, _ := v.screen.Get(x, pane.y+i)
						got.WriteString(c)
					}
					if !strings.Contains(got.String(), want) {
						t.Fatalf("scrolloff %d, cursor %d: row %d shows %q, want %q", scrolloff, row, i, got.String(), want)
					}
				}
			}
		}
		for row := range 83 { // down the file, then back up
			check(row)
		}
		for row := 82; row >= 0; row-- {
			check(row)
		}
	}
}
