package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalLeadersAndJumpHistory(t *testing.T) {
	for _, profile := range []string{"vim"} {
		t.Run(profile, func(t *testing.T) {
			e := testEditor(t, profile, "")
			e.cfg.Languages = nil
			first := fixture(t, e.root, "first.txt", "first\nsecond\nthird\n")
			second := fixture(t, e.root, "second.txt", "some other file\n")
			e.open(first)
			press(e, "space f")
			if e.menu == nil || e.menu.title != "Go to File" {
				t.Fatal("Space f did not open file picker")
			}
			press(e, "esc space x")
			if e.directory == nil {
				t.Fatal("space x did not open directory")
			}
			press(e, "esc space e")
			if !e.tree.focused {
				t.Fatal("Space e did not focus sidebar")
			}
			press(e, "esc space /")
			if e.menu == nil || !strings.HasPrefix(e.menu.title, "Search Project") {
				t.Fatal("Space / did not open search")
			}
			press(e, "esc : 3 enter")
			e.jumpTo(second, 0, 5, false)
			press(e, "ctrl+o")
			if e.b.path != first || e.b.rowAt(e.b.cursor) != 2 {
				t.Fatal("missing cross-file back jump")
			}
			press(e, "ctrl+o")
			if e.b.rowAt(e.b.cursor) != 0 {
				t.Fatal("missing line-number back jump")
			}
			press(e, "g ] g ]")
			if e.b.path != second || e.b.cursor != 5 {
				t.Fatal("missing forward jump")
			}
			press(e, "ctrl+o : 2 enter g ]")
			if e.b.path != first || e.b.rowAt(e.b.cursor) != 1 {
				t.Fatal("new jump did not truncate forward history")
			}
		})
	}
}

func TestExternalReloadAndGuardedSave(t *testing.T) {
	e := testEditor(t, "vim", "")
	path := fixture(t, e.root, "file.txt", "first")
	e.open(path)
	fixture(t, e.root, "file.txt", "changed on disk")
	e.checkDisk()
	if string(e.b.text) != "changed on disk" || e.b.dirty() {
		t.Fatal("clean buffer did not reload")
	}
	e.b.replace(span{0, len(e.b.text)}, "editor version")
	fixture(t, e.root, "file.txt", "second disk version")
	e.checkDisk()
	if !e.b.external || e.save() || e.menu == nil {
		t.Fatal("conflicting save did not show review")
	}
	fixture(t, e.root, "file.txt", "third disk version")
	press(e, "down enter")
	raw, _ := os.ReadFile(path)
	if string(raw) != "third disk version" || e.menu == nil {
		t.Fatal("stale confirmation overwrote disk")
	}
	press(e, "down enter")
	raw, _ = os.ReadFile(path)
	if string(raw) != "editor version" || e.b.dirty() || e.b.external {
		t.Fatal("confirmed save did not update disk")
	}
}

func TestRecoverAfterExternalChange(t *testing.T) {
	e := testEditor(t, "vim", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := fixture(t, e.root, "file.txt", "baseline")
	e.open(path)
	e.startRecovery()
	e.b.replace(span{0, len(e.b.text)}, "unsaved text")
	if err := e.saveRecovery(); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(e.recovery.root, "2147483647-1.json")
	if err := os.Rename(e.recovery.path, candidate); err != nil {
		t.Fatal(err)
	}
	fixture(t, e.root, "file.txt", "external update")
	blank, _ := loadBuffer("")
	reopened, err := newEditor(e.cfg, blank, e.root)
	if err != nil {
		t.Fatal(err)
	}
	reopened.startRecovery()
	if reopened.menu == nil {
		t.Fatal("missing recovery offer")
	}
	press(reopened, "enter")
	if reopened.b.path != path || string(reopened.b.text) != "unsaved text" || !reopened.b.external {
		t.Fatal("recovery lost edits or disk conflict")
	}
	if reopened.save() || reopened.menu == nil {
		t.Fatal("recovered buffer could silently overwrite external changes")
	}
}

func TestWorkspaceEditPreviewIsAtomic(t *testing.T) {
	e := testEditor(t, "vim", "")
	a := fixture(t, e.root, "a.txt", "one")
	b := fixture(t, e.root, "b.txt", "two")
	e.open(a)
	replacement := textEdit{Range: &lspRange{position{0, 0}, position{0, 3}}, NewText: "new"}
	change := workspaceEdit{Changes: map[string][]textEdit{fileURI(a): {replacement}, fileURI(b): {replacement}}}
	target := e.actionTarget()
	e.reviewWorkspaceEdit(target, "Rename", change)
	if e.menu == nil {
		t.Fatal(e.message)
	}
	fixture(t, e.root, "b.txt", "changed")
	press(e, "enter")
	if string(e.b.text) != "one" || len(e.buffers) != 1 {
		t.Fatal("part of stale workspace edit was applied")
	}
	fixture(t, e.root, "b.txt", "two")
	e.reviewWorkspaceEdit(target, "Rename", change)
	press(e, "enter")
	if len(e.buffers) != 2 {
		t.Fatal(e.message)
	}
	for _, buf := range e.buffers {
		if string(buf.text) != "new" || !buf.dirty() || len(buf.undo) != 1 {
			t.Fatal("workspace edit was not one unsaved edit")
		}
	}
}

func TestGoplsActionsAndRename(t *testing.T) {
	if os.Getenv("LOOM_TEST_GOPLS") == "" {
		t.Skip("set LOOM_TEST_GOPLS=1 to run")
	}
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Fatal(err)
	}
	e := testEditor(t, "vim", "")
	fixture(t, e.root, "go.mod", "module test.local/loom\n\ngo 1.26\n")
	fixture(t, e.root, "definition.go", "package demo\n\nfunc greet() int { return 1 }\n")
	call := fixture(t, e.root, "call.go", "package demo\n\nimport \"fmt\"\n\nfunc caller() int { return greet() }\n")
	e.open(call)
	t.Cleanup(e.closeServers)
	e.refreshLanguage()
	awaitUpdate(t, e, func() bool { return len(e.b.diagnostics) > 0 })
	e.b.cursor = e.b.lines[2] + 8
	press(e, "space a")
	awaitUpdate(t, e, func() bool { return e.menu != nil })
	if e.menu.title != "Code Actions" {
		t.Fatal(e.menu.title)
	}
	v := renderer{screen: newTerminalFrame(100, 24)}
	v.draw(e)
	if v.previewBox.h != 0 {
		t.Fatal("code action list displayed a preview")
	}
	e.menuInput(keyInput{paste: "Organize Imports"})
	if len(e.menu.matches) == 0 {
		t.Fatalf("organize imports absent: %v", e.menu.options)
	}
	press(e, "enter")
	awaitUpdate(t, e, func() bool { return e.menu != nil })
	if !e.menu.confirmation {
		t.Fatal("no action preview")
	}
	v.draw(e)
	if v.previewBox.h == 0 {
		t.Fatal("code action confirmation did not display its edit preview")
	}
	press(e, "enter")
	if strings.Contains(string(e.b.text), "import") {
		t.Fatal(e.message)
	}
	e.b.cursor = strings.Index(string(e.b.text), "greet")
	press(e, "space r ctrl+u")
	e.menuInput(keyInput{paste: "salute"})
	press(e, "enter")
	awaitUpdate(t, e, func() bool { return e.menu != nil })
	if !e.menu.confirmation {
		t.Fatal("no rename preview")
	}
	press(e, "enter")
	if len(e.buffers) != 2 {
		t.Fatal(e.message)
	}
	for _, b := range e.buffers {
		if !strings.Contains(string(b.text), "salute") || !b.dirty() {
			t.Fatalf("rename missing from %s: %s", b.path, e.message)
		}
		raw, _ := os.ReadFile(b.path)
		if !strings.Contains(string(raw), "greet") {
			t.Fatal("rename saved before approval")
		}
	}
}
