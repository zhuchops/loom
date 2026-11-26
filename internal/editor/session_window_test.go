package editor

import (
	"slices"
	"testing"
)

func TestMoveSessionToNewWindow(t *testing.T) {
	e, dirs := sessionFixture(t, "one", "two")
	s := openAll(t, e, dirs)
	h := e.hub
	var started []string
	h.window = func(dir string) error { started = append(started, dir); return nil }

	makeDirty(s[1])
	h.moveToWindow(s[1])
	if len(started) != 0 || h.find(dirs[1]) == nil {
		t.Fatalf("a session with unsaved changes moved: %v", started)
	}
	s[1].b.saved = string(s[1].b.text)

	h.moveToWindow(s[0])
	if !slices.Equal(started, []string{dirs[0]}) || h.find(dirs[0]) != nil {
		t.Fatalf("started %v; still here: %v", started, h.find(dirs[0]) != nil)
	}
	if slices.Contains(savedSessions(), dirs[0]) {
		t.Fatal("this window still lists the session it gave away")
	}

	// The last session leaves the window on the start page.
	h.moveToWindow(h.current)
	if !slices.Equal(started, []string{dirs[0], dirs[1]}) || !h.current.noProject {
		t.Fatalf("started %v; current is %s", started, h.current.root)
	}
}

func TestSessionsMenuOpensInNewWindow(t *testing.T) {
	e, dirs := sessionFixture(t, "one", "two")
	openAll(t, e, dirs)
	h := e.hub
	h.current.sessionsMenu("", 0)
	if h.current.menu.window != nil {
		t.Fatal("the terminal offers new windows")
	}
	var started []string
	h.window = func(dir string) error { started = append(started, dir); return nil }
	h.current.sessionsMenu("", 0)
	m := h.current.menu
	if m.matches[0].label != "one" {
		t.Fatalf("first row is %q", m.matches[0].label)
	}
	h.current.handle(keyInput{key: "ctrl+o"})
	if !slices.Equal(started, []string{dirs[0]}) || h.current.menu != nil {
		t.Fatalf("Ctrl+O started %v", started)
	}
}
