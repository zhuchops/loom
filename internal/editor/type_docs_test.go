package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A variable's hover also carries the documentation of its type, read from a
// file that is not open, and g y jumps to that type.
func TestHoverShowsTheTypesDocumentation(t *testing.T) {
	if os.Getenv("LOOM_TEST_GOPLS") == "" {
		t.Skip("set LOOM_TEST_GOPLS=1 to run")
	}
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Fatal(err)
	}
	e := testEditor(t, "vim", "")
	fixture(t, e.root, "go.mod", "module test.local/loom\n\ngo 1.26\n")
	if err := os.Mkdir(filepath.Join(e.root, "ent"), 0700); err != nil {
		t.Fatal(err)
	}
	userFile := fixture(t, e.root, "ent/user.go", "package ent\n\n// User is a person who signed up.\ntype User struct {\n\tName string\n}\n\n// Find looks a user up.\nfunc Find() *User { return &User{} }\n")
	path := fixture(t, e.root, "main.go", "package main\n\nimport \"test.local/loom/ent\"\n\nfunc main() {\n\tuser := ent.Find()\n\t_ = user\n}\n")
	e.open(path)
	t.Cleanup(e.closeServers)
	e.b.cursor = e.b.lines[6] + 5 // the user in `_ = user`
	press(e, "g h")
	awaitUpdate(t, e, func() bool { return e.menu != nil })
	var docs []string
	for _, o := range e.menu.options {
		docs = append(docs, o.label)
		if o.label == "Type Documentation" {
			if got := o.preview().message; !strings.Contains(got, "User is a person who signed up.") {
				t.Fatalf("type documentation = %q", got)
			}
		}
	}
	if strings.Join(docs, ",") != "Documentation,Type Documentation" {
		t.Fatalf("hover options = %v", docs)
	}
	press(e, "esc")
	e.b.cursor = e.b.lines[6] + 5
	press(e, "g y")
	awaitUpdate(t, e, func() bool { return e.b.path == userFile })
	if row := e.b.rowAt(e.b.cursor); row != 3 {
		t.Fatalf("g y landed on row %d", row)
	}
}
