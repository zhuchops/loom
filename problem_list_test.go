package main

import (
	"path/filepath"
	"strings"
	"testing"
)

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
