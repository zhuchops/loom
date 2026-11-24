package editor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDraggingInTheSidebarMovesFiles(t *testing.T) {
	e := testEditor(t, "vim", "")
	for _, dir := range []string{"src", "docs", "docs/old"} {
		if err := os.MkdirAll(filepath.Join(e.root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	notes := fixture(t, e.root, "notes.md", "hi\n")
	fixture(t, e.root, "docs/guide.md", "guide\n")
	e.open(notes)
	e.tree.expanded[filepath.Join(e.root, "docs")] = true
	e.refreshTree("")
	path := func(rel string) string { return filepath.Join(e.root, rel) }
	row := func(rel string) int {
		t.Helper()
		for i, r := range e.tree.rows {
			if r.path == path(rel) {
				return i
			}
		}
		t.Fatalf("%s is not in the tree", rel)
		return -1
	}

	// Onto a folder: into it. The open tab follows the file.
	if err := e.dropEntry(path("notes.md"), path("src")); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(e.root, "src", "notes.md")
	if _, err := os.Stat(moved); err != nil || e.b.path != moved {
		t.Fatalf("not moved into src: %v, tab at %s", err, e.b.path)
	}
	// Onto a file: beside it.
	if err := e.dropEntry(path("src/notes.md"), path("docs/guide.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.root, "docs", "notes.md")); err != nil {
		t.Fatal("not moved beside guide.md")
	}
	// Into the folder it is already in: nothing happens.
	if err := e.dropEntry(path("docs/notes.md"), path("docs")); err != nil {
		t.Fatal(err)
	}
	// A folder into itself is refused.
	if err := e.dropEntry(path("docs"), path("docs/old")); err == nil {
		t.Fatal("a folder moved into itself")
	}
	// The project folder stays where it is.
	if err := e.dropEntry(e.root, path("src")); err != nil || e.tree.rows[0].path != e.root {
		t.Fatal("the project folder moved")
	}
	// Resting on a folder opens it.
	e.expandEntry(row("src"))
	if !e.tree.expanded[filepath.Join(e.root, "src")] {
		t.Fatal("src did not open")
	}
}
