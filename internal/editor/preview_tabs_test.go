package editor

import (
	"path/filepath"
	"testing"
)

func TestPreviewTabs(t *testing.T) {
	e := testEditor(t, "vim", "")
	a := fixture(t, e.root, "a.go", "package a\n")
	b := fixture(t, e.root, "b.go", "package b\n")
	c := fixture(t, e.root, "c.go", "package c\n")
	d := fixture(t, e.root, "d.go", "package d\n")
	names := func() []string {
		var out []string
		for _, buf := range e.buffers {
			name := filepath.Base(buf.path)
			if buf.isPreview() {
				name = "~" + name
			}
			out = append(out, name)
		}
		return out
	}
	want := func(expected ...string) {
		t.Helper()
		got := names()
		if len(got) != len(expected) {
			t.Fatalf("tabs %v, want %v", got, expected)
		}
		for i := range got {
			if got[i] != expected[i] {
				t.Fatalf("tabs %v, want %v", got, expected)
			}
		}
	}

	e.open(a)
	e.openPreview(b)
	want("a.go", "~b.go")
	// The next preview takes the place of the last one.
	e.openPreview(c)
	want("a.go", "~c.go")
	if e.b.path != c {
		t.Fatalf("showing %s", e.b.path)
	}

	// Keeping it opens the next preview beside it.
	e.runCommand("keep-tab")
	e.openPreview(d)
	want("a.go", "c.go", "~d.go")

	// An edit keeps a preview.
	e.handle(keyInput{key: "i"})
	e.handle(keyInput{key: "x"})
	e.handle(keyInput{key: "esc"})
	e.openPreview(b)
	want("a.go", "c.go", "d.go", "~b.go")

	// Opening a preview's file for good keeps it; a file that has a tab
	// keeps its tab when browsed to.
	e.open(b)
	e.openPreview(a)
	want("a.go", "c.go", "d.go", "b.go")

	// Switched off, browsing opens ordinary tabs.
	e.closeBuffer(e.buffers[3])
	e.openPreview(b)
	e.cfg.PreviewTabs = false
	e.openPreview(filepath.Join(e.root, "a.go"))
	e.openPreview(fixture(t, e.root, "e.go", "package e\n"))
	want("a.go", "c.go", "d.go", "~b.go", "e.go")
}

func TestPreviewShownInAnotherPaneStays(t *testing.T) {
	e := testEditor(t, "vim", "")
	a := fixture(t, e.root, "a.go", "package a\n")
	b := fixture(t, e.root, "b.go", "package b\n")
	e.openPreview(a)
	e.splitPane(true, "")
	e.openPreview(b)
	if len(e.buffers) != 2 {
		t.Fatalf("%d tabs, want both files", len(e.buffers))
	}
	if e.buffers[0].isPreview() || !e.buffers[1].isPreview() {
		t.Fatal("the file still on screen should have been kept")
	}
}
