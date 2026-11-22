package editor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectsAreRemembered(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	old, _ := os.Getwd()
	defer os.Chdir(old)

	if lastProject() != "" {
		t.Fatal("a fresh config should have no last project")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, _ := loadBuffer("")
	e, err := newEditor(defaultConfig(), b, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e.enterNoProject()
	e.needsProject("files")
	e.runCommand("files")
	if e.menu != nil {
		t.Fatal("Go to File must not open without a project")
	}
	e.openProject(dir)
	canonical, _ := filepath.EvalSymlinks(dir)
	if e.noProject || e.root != canonical {
		t.Fatalf("project not opened: noProject=%v root=%q", e.noProject, e.root)
	}
	if got := lastProject(); got != canonical {
		t.Fatalf("last project = %q, want %q", got, canonical)
	}
	e.closeProject()
	if !e.noProject || lastProject() != "" {
		t.Fatal("closing a project must forget it as the default")
	}
	if len(recentProjects()) != 1 {
		t.Fatalf("closed project should stay in the recent list: %v", recentProjects())
	}
}
