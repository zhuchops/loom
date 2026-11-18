package editor

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilePreviewFillsTallAndNarrowPickers(t *testing.T) {
	e := testEditor(t, "vim", "")
	var source strings.Builder
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&source, "source row %02d\n", i)
	}
	path := fixture(t, e.root, "tall.txt", source.String())
	e.openMenu(&menu{title: "Results", options: []option{{label: "tall.txt", preview: e.filePreview(path, 39, 0, false, nil)}}})
	for _, width := range []int{100, 52} {
		v := renderer{screen: newTerminalFrame(width, 27)}
		e.menu.previewScroll = 0
		v.draw(e)
		if frame := v.screen.String(); !strings.Contains(frame, "source row 30") || !strings.Contains(frame, "source row 40") {
			t.Fatalf("width %d: near-EOF preview did not show earlier rows and the final source line", width)
		}
		e.menu.previewScroll -= 30
		v.draw(e)
		if !strings.Contains(v.screen.String(), "source row 01") {
			t.Fatalf("width %d: could not scroll up to the beginning", width)
		}
	}
}

func TestProjectProblemsListsEveryFile(t *testing.T) {
	e := testEditor(t, "vim", "")
	a := fixture(t, e.root, "a.go", "package p\n")
	b := fixture(t, e.root, "b.go", "package p\n")
	publish(t, e, a, 1, 2)
	publish(t, e, b, 1)
	awaitUpdate(t, e, func() bool { return len(e.problems) == 2 })

	e.showProjectProblems()
	if e.menu == nil || !strings.HasPrefix(e.menu.title, "Project Problems · 2 errors, 1 warnings") {
		t.Fatalf("title = %v", e.menu)
	}
	if len(e.menu.options) != 3 {
		t.Fatalf("options = %d", len(e.menu.options))
	}
	if !strings.HasPrefix(e.menu.options[0].label, "a.go:1") || e.menu.options[0].hint != "error" {
		t.Fatalf("first option = %+v", e.menu.options[0])
	}
	if e.menu.options[1].hint != "warning" {
		t.Fatalf("second option hint = %q", e.menu.options[1].hint)
	}
	e.menu.options[2].run()
	if e.b.path != b {
		t.Fatalf("enter did not open %s, got %s", filepath.Base(b), e.b.path)
	}
}

func TestFileProblemsUsesCurrentBuffer(t *testing.T) {
	e := testEditor(t, "vim", "")
	path := fixture(t, e.root, "open.go", "package p\n")
	other := fixture(t, e.root, "other.go", "package p\n")
	e.open(path)
	publish(t, e, path, 1, 2)
	publish(t, e, other, 1)
	awaitUpdate(t, e, func() bool { return len(e.b.diagnostics) == 2 })

	e.showFileProblems()
	if e.menu == nil || !strings.HasPrefix(e.menu.title, "File Problems · 1 errors, 1 warnings") {
		t.Fatalf("title = %v", e.menu)
	}
	if len(e.menu.options) != 2 {
		t.Fatalf("file problems listed %d items", len(e.menu.options))
	}
}

func TestProblemKeysOpenTheRightList(t *testing.T) {
	e := testEditor(t, "vim", "")
	path := fixture(t, e.root, "keys.go", "package p\n")
	e.open(path)
	publish(t, e, path, 1)
	awaitUpdate(t, e, func() bool { return len(e.b.diagnostics) == 1 })
	press(e, "space d")
	if e.menu == nil || !strings.HasPrefix(e.menu.title, "File Problems") {
		t.Fatalf("space d opened %v", e.menu)
	}
	e.menu = nil
	press(e, "space D")
	if e.menu == nil || !strings.HasPrefix(e.menu.title, "Project Problems") {
		t.Fatalf("space D opened %v", e.menu)
	}
}

func TestDiagnosticRangeAndProblemPreview(t *testing.T) {
	e := testEditor(t, "vim", "")
	path := fixture(t, e.root, "sample.go", "🙂 bad suffix\nnext\n")
	e.open(path)
	d := diagnostic{
		Range:    lspRange{Start: position{0, 3}, End: position{0, 6}},
		Severity: 1,
		Message:  "cannot use bad here\nexpected a value",
	}
	e.b.diagnostics = []diagnostic{d}
	v := renderer{screen: newTerminalFrame(100, 22)}
	v.draw(e)
	pane := v.panes[0]
	x, y := pane.x+pane.gutter+3, pane.y
	for delta := -1; delta <= 3; delta++ {
		_, style, _ := v.screen.Get(x+delta, y)
		want := terminalColor(0)
		if delta >= 0 && delta < 3 {
			want = diagnosticColor(1)
		}
		if style.underline != want {
			t.Fatalf("cell %d underline = %v, want %v", delta, style.underline, want)
		}
	}

	e.showFileProblems()
	v.draw(e)
	preview := v.previewBox
	if !strings.Contains(v.screen.String(), "cannot use bad here") || !strings.Contains(v.screen.String(), "sample.go:1:4") {
		t.Fatal("diagnostic preview lacks its message or location")
	}
	found := false
	for row := preview.y; row < preview.y+preview.h; row++ {
		for col := preview.x; col < preview.x+preview.w; col++ {
			text, style, _ := v.screen.Get(col, row)
			found = found || text == "b" && style.underline == diagnosticColor(1)
		}
	}
	if !found {
		t.Fatal("preview did not underline the affected source")
	}
	v.screen.Resize(52, 18)
	v.draw(e)
	if v.previewBox.y < v.menuBox.y+v.menuBox.h || !strings.Contains(v.screen.String(), "Preview") {
		t.Fatal("preview did not move below the picker on a narrow terminal")
	}
}

func TestProjectProblemPreviewDoesNotOpenFile(t *testing.T) {
	e := testEditor(t, "vim", "")
	path := fixture(t, e.root, "unopened.go", "package example\n\nfunc broken() {}\n")
	d := diagnostic{Range: lspRange{position{2, 5}, position{2, 11}}, Severity: 2, Message: "Example warning"}
	e.problems[path] = []diagnostic{d}
	e.showProjectProblems()
	p := e.menu.selectedPreview()
	if p.source == nil || !strings.Contains(string(p.source.text), "func broken") || p.row != 2 || len(e.buffers) != 1 || e.b.path != "" {
		t.Fatal("preview failed to read the unopened file without opening a tab")
	}
}

func TestDiagnosticPointAndExclusiveEnd(t *testing.T) {
	e := testEditor(t, "vim", "abc\ndef")
	e.b.diagnostics = []diagnostic{
		{Range: lspRange{position{0, 1}, position{1, 0}}, Severity: 2},
		{Range: lspRange{position{0, 2}, position{0, 3}}, Severity: 1},
		{Range: lspRange{position{1, 3}, position{1, 3}}, Severity: 1},
	}
	marks := diagnosticRows(e.b, 0, 2)
	for _, tc := range []struct{ row, at, severity int }{{0, 0, 0}, {0, 1, 2}, {0, 2, 1}, {1, 4, 0}, {1, 7, 1}} {
		if got := diagnosticAt(marks[tc.row], tc.at, tc.at+1); got != tc.severity {
			t.Errorf("row %d, offset %d: got severity %d, want %d", tc.row, tc.at, got, tc.severity)
		}
	}
}
