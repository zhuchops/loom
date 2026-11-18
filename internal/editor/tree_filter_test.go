package editor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func treeNames(e *editor) []string {
	var names []string
	for _, r := range e.tree.rows[1:] {
		names = append(names, filepath.Base(r.path))
	}
	return names
}

func TestTreeFilterSearchesCollapsedFolders(t *testing.T) {
	e := testEditor(t, "vim", "")
	os.MkdirAll(filepath.Join(e.root, "a", "deep"), 0755)
	os.MkdirAll(filepath.Join(e.root, "empty"), 0755)
	os.WriteFile(filepath.Join(e.root, "a", "deep", "target.go"), nil, 0644)
	os.WriteFile(filepath.Join(e.root, "other.txt"), nil, 0644)
	e.tree.expanded = map[string]bool{e.root: true}
	e.showTree(false)
	e.tree.focused = true
	for _, k := range []string{"/", "t", "a", "r", "g"} {
		e.handle(keyInput{key: k})
	}
	got := treeNames(e)
	if len(got) != 3 || got[2] != "target.go" || e.tree.rows[e.tree.selected].path != filepath.Join(e.root, "a", "deep", "target.go") {
		t.Fatalf("rows %v selected %d", got, e.tree.selected)
	}
	e.handle(keyInput{key: "enter"})
	e.handle(keyInput{key: "esc"})
	if e.tree.filter != "" || len(treeNames(e)) != 3 {
		t.Fatalf("filter not cleared: %v", treeNames(e))
	}
}

func TestTreeSortModes(t *testing.T) {
	e := testEditor(t, "vim", "")
	for i, name := range []string{"a.go", "b.txt", "c.go"} {
		path := filepath.Join(e.root, name)
		os.WriteFile(path, make([]byte, 10-i*4), 0644)
		os.Chtimes(path, time.Now(), time.Now().Add(time.Duration(i)*-time.Hour))
	}
	e.tree.expanded = map[string]bool{e.root: true}
	e.showTree(false)
	e.tree.focused = true
	check := func(want ...string) {
		t.Helper()
		got := treeNames(e)
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: got %v want %v", treeSorts[e.tree.sortMode], got, want)
			}
		}
	}
	check("a.go", "b.txt", "c.go")
	e.handle(keyInput{key: "s"})
	check("a.go", "b.txt", "c.go") // newest first
	e.handle(keyInput{key: "s"})
	check("a.go", "b.txt", "c.go") // largest first
	e.handle(keyInput{key: "s"})
	check("a.go", "c.go", "b.txt") // by extension
}
