package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gitProject makes a repository with one commit in a new project.
func gitProject(t *testing.T) *editor {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git missing")
	}
	e := testEditor(t, "vim", "")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	git(t, e.root, "init", "-q", "-b", "main")
	git(t, e.root, "config", "user.name", "Loom Test")
	git(t, e.root, "config", "user.email", "loom@example.com")
	fixture(t, e.root, "a.txt", "one\n")
	git(t, e.root, "add", "-A")
	git(t, e.root, "commit", "-q", "-m", "first")
	return e
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitExec(dir, 10*time.Second, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func TestGitCommandsFromThePalette(t *testing.T) {
	e := gitProject(t)
	// Nothing staged: the commit takes every change.
	fixture(t, e.root, "a.txt", "two\n")
	fixture(t, e.root, "b.txt", "new\n")
	e.commands["git-commit"].run(e)
	if e.menu == nil || !strings.Contains(e.menu.title, "all changes") {
		t.Fatalf("commit asked %v", e.menu)
	}
	e.menu.submit("second")
	e.menu = nil
	if got := git(t, e.root, "log", "-1", "--format=%s"); got != "second" {
		t.Fatalf("last commit %q", got)
	}
	if out := git(t, e.root, "status", "--porcelain"); out != "" {
		t.Fatalf("left changes: %q", out)
	}

	// Amend starts from the last message.
	e.commands["git-amend"].run(e)
	if e.menu.input != "second" {
		t.Fatalf("amend started with %q", e.menu.input)
	}
	e.menu.submit("second, amended")
	e.menu = nil
	if got := git(t, e.root, "log", "-1", "--format=%s"); got != "second, amended" {
		t.Fatalf("amended to %q", got)
	}
	if n := git(t, e.root, "rev-list", "--count", "HEAD"); n != "2" {
		t.Fatalf("%s commits after amending", n)
	}

	// A new branch.
	e.commands["git-new-branch"].run(e)
	e.menu.submit("feature/x")
	e.menu = nil
	if got := git(t, e.root, "branch", "--show-current"); got != "feature/x" {
		t.Fatalf("on %q", got)
	}

	// Stash and pop run in the background.
	fixture(t, e.root, "a.txt", "three\n")
	e.commands["git-stash"].run(e)
	awaitUpdate(t, e, func() bool {
		return strings.Contains(e.message, "Saved working directory") || strings.Contains(e.message, "stash") && !strings.HasSuffix(e.message, "…")
	})
	if out := git(t, e.root, "status", "--porcelain"); out != "" {
		t.Fatalf("stash left %q", out)
	}
	e.commands["git-stash-pop"].run(e)
	awaitUpdate(t, e, func() bool { return !strings.HasSuffix(e.message, "…") })
	if data, _ := os.ReadFile(filepath.Join(e.root, "a.txt")); string(data) != "three\n" {
		t.Fatalf("pop gave %q", data)
	}
}

func TestGitChangesMadeOutsideShowAtOnce(t *testing.T) {
	e := gitProject(t)
	fixture(t, e.root, "a.txt", "changed\n")
	e.refreshGit()
	awaitUpdate(t, e, func() bool { return e.git.files[filepath.Join(e.root, "a.txt")] != 0 })
	if e.git.dir == "" {
		t.Fatal("the git folder is unknown")
	}

	// A commit from a terminal: the marks go without anything typed here.
	git(t, e.root, "commit", "-q", "-am", "from a terminal")
	e.watchGit()
	awaitUpdate(t, e, func() bool { return e.git.files[filepath.Join(e.root, "a.txt")] == 0 })

	// Nothing moved: watching again reads nothing.
	token := e.gitToken
	e.watchGit()
	if e.gitToken != token {
		t.Fatal("refreshed with nothing changed")
	}
}
