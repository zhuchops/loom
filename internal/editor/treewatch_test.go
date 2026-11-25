package editor

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestTreeShowsFilesMadeOutside(t *testing.T) {
	e := testEditor(t, "vim", "")
	if err := os.MkdirAll(filepath.Join(e.root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := fixture(t, e.root, "old.go", "package a\n")
	fixture(t, e.root, "src/main.go", "package a\n")
	e.tree.expanded[e.root] = true
	e.tree.expanded[filepath.Join(e.root, "src")] = true
	e.refreshTree("")
	names := func() []string {
		var out []string
		for _, r := range e.tree.rows[1:] {
			rel, _ := filepath.Rel(e.root, r.path)
			out = append(out, rel)
		}
		return out
	}
	// Folder times can be coarse; step them so each change shows.
	stamp := 0
	touch := func(dir string) {
		stamp++
		at := time.Unix(1_700_000_000+int64(stamp), 0)
		_ = os.Chtimes(filepath.Join(e.root, dir), at, at)
	}

	e.watchTree()
	before := names()

	fixture(t, e.root, "src/new.go", "package a\n")
	touch("src")
	if err := os.Rename(old, filepath.Join(e.root, "renamed.go")); err != nil {
		t.Fatal(err)
	}
	touch(".")
	e.watchTree()
	got := names()
	for _, want := range []string{"src/new.go", "renamed.go"} {
		if !slices.Contains(got, want) {
			t.Fatalf("tree %v lacks %s (was %v)", got, want, before)
		}
	}
	if slices.Contains(got, "old.go") {
		t.Fatalf("tree %v still lists the renamed file", got)
	}

	if err := os.Remove(filepath.Join(e.root, "src/new.go")); err != nil {
		t.Fatal(err)
	}
	touch("src")
	e.watchTree()
	if slices.Contains(names(), "src/new.go") {
		t.Fatalf("tree %v still lists the deleted file", names())
	}
}
