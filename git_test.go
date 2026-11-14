package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gitRepo(t *testing.T) *editor {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("GIT_AUTHOR_NAME", "loom")
	t.Setenv("GIT_AUTHOR_EMAIL", "loom@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "loom")
	t.Setenv("GIT_COMMITTER_EMAIL", "loom@example.com")
	e := testEditor(t, "vim", "")
	for _, args := range [][]string{{"init", "-q"}, {"config", "commit.gpgsign", "false"}, {"config", "user.name", "loom"}, {"config", "user.email", "loom@example.com"}} {
		if _, err := gitExec(e.root, 10*time.Second, args...); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func gitMust(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitExec(dir, 10*time.Second, args...)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestSourceControlStageAndCommit(t *testing.T) {
	e := gitRepo(t)
	fixture(t, e.root, "a.txt", "one\ntwo\n")
	gitMust(t, e.root, "add", ".")
	gitMust(t, e.root, "commit", "-q", "-m", "init")
	fixture(t, e.root, "a.txt", "one\nthree\n")
	fixture(t, e.root, "b.txt", "new\n")

	press(e, "space g g")
	if e.panel == nil {
		t.Fatal("source control did not open")
	}
	awaitUpdate(t, e, func() bool { return len(e.panel.changes) == 2 })
	row, ok := e.panel.current()
	if !ok || row.change.rel != "a.txt" {
		t.Fatalf("first selectable row = %+v", row)
	}

	press(e, "s")
	awaitUpdate(t, e, func() bool { return e.panel.hasStaged() })

	press(e, "c")
	if e.menu == nil || e.menu.title != "Commit message" {
		t.Fatal("commit prompt did not open")
	}
	e.menuInput(keyInput{paste: "fix"})
	press(e, "enter")
	if got := strings.TrimSpace(gitMust(t, e.root, "log", "-1", "--format=%s")); got != "fix" {
		t.Fatalf("last commit = %q, message: %s", got, e.message)
	}
}

func TestDiffHunkNavigation(t *testing.T) {
	e := gitRepo(t)
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, "line "+strings.Repeat("x", i%3))
	}
	path := fixture(t, e.root, "long.txt", strings.Join(lines, "\n")+"\n")
	gitMust(t, e.root, "add", ".")
	gitMust(t, e.root, "commit", "-q", "-m", "init")
	lines[1] = "changed top"
	lines[27] = "changed bottom"
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e.open(path)

	press(e, "space g d")
	if e.diff == nil || len(e.diff.hunks) != 2 {
		t.Fatalf("diff hunks = %+v", e.diff)
	}
	if e.diff.cursor != e.diff.hunks[0].line {
		t.Fatal("diff did not open on the first hunk")
	}
	press(e, "] c")
	if e.diff.cursor != e.diff.hunks[1].line {
		t.Fatal("] c did not reach the second hunk")
	}
	press(e, "[ c")
	if e.diff.cursor != e.diff.hunks[0].line {
		t.Fatal("[ c did not go back")
	}
	want := e.diff.hunks[0].start - 1
	press(e, "enter")
	if e.diff != nil || e.b.path != path {
		t.Fatal("enter did not open the file from the hunk")
	}
	if row := e.b.rowAt(e.b.cursor); row != want {
		t.Fatalf("cursor row = %d, want %d", row, want)
	}
}

func TestBranchPickerSwitches(t *testing.T) {
	e := gitRepo(t)
	fixture(t, e.root, "a.txt", "one\n")
	gitMust(t, e.root, "add", ".")
	gitMust(t, e.root, "commit", "-q", "-m", "init")
	gitMust(t, e.root, "branch", "feature")

	e.openBranchPicker()
	if e.menu == nil || e.menu.title != "Switch Branch" {
		t.Fatal("branch picker did not open")
	}
	found := false
	for _, o := range e.menu.options {
		if o.label == "feature" {
			o.run()
			found = true
		}
	}
	if !found {
		t.Fatal("feature branch missing from picker")
	}
	if got := gitBranch(e.root); got != "feature" {
		t.Fatalf("branch = %q, message: %s", got, e.message)
	}
	e.refreshGit()
	awaitUpdate(t, e, func() bool { return e.git.branch == "feature" })
}

func TestSourceControlRendersWithDiff(t *testing.T) {
	e := gitRepo(t)
	fixture(t, e.root, "a.txt", "one\n")
	gitMust(t, e.root, "add", ".")
	gitMust(t, e.root, "commit", "-q", "-m", "init")
	fixture(t, e.root, "a.txt", "two\n")
	press(e, "space g g")
	awaitUpdate(t, e, func() bool { return len(e.panel.changes) == 1 })
	press(e, "enter")
	if e.diff == nil {
		t.Fatal("enter did not open the diff")
	}
	screen := newTerminalFrame(100, 30)
	v := renderer{screen: screen}
	v.draw(e)
	if text, _, _ := screen.Get(0, 0); text == "" {
		t.Fatal("nothing drawn")
	}
	if !filepath.IsAbs(e.panel.top) {
		t.Fatal("repository top is not absolute")
	}
}
