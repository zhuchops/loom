package editor

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// sessionFixture is a blank editor and some project folders to open in it.
func sessionFixture(t *testing.T, names ...string) (*editor, []string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	parent, _ := filepath.EvalSymlinks(t.TempDir())
	var dirs []string
	for _, name := range names {
		dir := filepath.Join(parent, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(name+" one\n"+name+" two\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, dir)
	}
	e := blankEditor(t)
	t.Cleanup(func() {
		e.hub.finish()
		_ = os.Chdir(wd)
		applyTheme(baselineTheme())
	})
	return e, dirs
}

func blankEditor(t *testing.T) *editor {
	t.Helper()
	cfg := defaultConfig()
	cfg.path = configPath()
	cfg.AutoComplete = false
	home, _ := filepath.EvalSymlinks(t.TempDir())
	b, err := loadBuffer("")
	if err != nil {
		t.Fatal(err)
	}
	e, err := newEditor(cfg, b, home)
	if err != nil {
		t.Fatal(err)
	}
	e.enterNoProject()
	return e
}

// openAll opens each folder and its file, and returns the sessions.
func openAll(t *testing.T, e *editor, dirs []string) []*editor {
	t.Helper()
	var out []*editor
	for _, dir := range dirs {
		e.hub.current.openProject(dir)
		s := e.hub.current
		if s.root != dir || !s.open(filepath.Join(dir, "a.txt")) {
			t.Fatalf("couldn't open %s, at %s: %s", dir, s.root, s.message)
		}
		out = append(out, s)
	}
	return out
}

func makeDirty(e *editor) { e.handle(keyInput{key: "x"}) }

func cwd(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir, _ = filepath.EvalSymlinks(dir)
	return dir
}

func TestSessionsKeepTheirState(t *testing.T) {
	e, dirs := sessionFixture(t, "alpha", "beta")
	s := openAll(t, e, dirs[:1])
	a := s[0]
	if a != e {
		t.Fatal("the blank start page should become the first project")
	}
	makeDirty(a)
	text, undo := string(a.b.text), len(a.b.undo)

	b := openAll(t, a, dirs[1:])[0]
	h := e.hub
	if b == a || len(h.live) != 2 || cwd(t) != dirs[1] {
		t.Fatalf("no second session: %d live, in %s", len(h.live), cwd(t))
	}
	press(b, "j")
	if !h.show(a) || h.current != a || cwd(t) != dirs[0] {
		t.Fatal("couldn't switch back")
	}
	if string(a.b.text) != text || len(a.b.undo) != undo || a.unsavedCount() != 1 {
		t.Fatal("switching lost the changes")
	}
	press(a, "u")
	if a.unsavedCount() != 0 {
		t.Fatal("switching lost undo")
	}
	h.show(b)
	if b.b.rowAt(b.b.cursor) != 1 {
		t.Fatal("switching lost the cursor")
	}

	a.openProject(dirs[0]) // as a folder dialog answering late would
	if h.current != a || len(h.live) != 2 {
		t.Fatal("an open project must be shown, not opened twice")
	}
}

func TestSessionsShareWhatWasCopied(t *testing.T) {
	e, dirs := sessionFixture(t, "alpha", "beta")
	s := openAll(t, e, dirs)
	s[1].hub.show(s[0])
	press(s[0], "y y")
	s[0].hub.show(s[1])
	press(s[1], "p")
	if !strings.Contains(string(s[1].b.text), "alpha one") {
		t.Fatalf("pasted %q", string(s[1].b.text))
	}
}

func TestSessionSwitcher(t *testing.T) {
	e, dirs := sessionFixture(t, "billing-api", "loom", "website")
	s := openAll(t, e, dirs)
	h := e.hub

	press(s[2], "space tab")
	m := s[2].menu
	if m == nil || m.title != "Sessions" || m.matches[0].label != "loom" || m.matches[2].label != "website" {
		t.Fatalf("switcher: %+v", m)
	}
	press(s[2], "enter")
	if h.current != s[1] {
		t.Fatal("Enter should go to the previous session")
	}
	press(h.current, "space tab")
	press(h.current, "enter")
	if h.current != s[2] {
		t.Fatal("Space Tab Enter should flip between two sessions")
	}

	press(h.current, "space tab b l g")
	if got := h.current.menu.matches[0].label; got != "billing-api" {
		t.Fatalf("fuzzy search found %q first", got)
	}
	press(h.current, "enter")
	if h.current != s[0] || cwd(t) != dirs[0] {
		t.Fatal("the fuzzy pick didn't switch")
	}
}

func TestFuzzyNamesBeatPaths(t *testing.T) {
	m := &menu{input: "lm", options: []option{
		{"website", "open · ~/library/mine/website", nil, nil},
		{"loom", "open · ~/loom", nil, nil},
	}}
	m.filter()
	if m.matches[0].label != "loom" {
		t.Fatalf("got %q first", m.matches[0].label)
	}
}

func TestClosingSessions(t *testing.T) {
	e, dirs := sessionFixture(t, "alpha", "beta", "gamma")
	s := openAll(t, e, dirs)
	h := e.hub
	makeDirty(s[0])

	press(s[2], "space tab a l p h a ctrl+x")
	if !slices.Contains(h.live, s[0]) {
		t.Fatal("closed a session with unsaved changes")
	}
	if h.current.menu == nil || h.current.menu.input != "alpha" {
		t.Fatal("the switcher should stay up")
	}
	press(h.current, "esc space tab b e t a ctrl+x")
	if slices.Contains(h.live, s[1]) || slices.Contains(savedSessions(), dirs[1]) {
		t.Fatal("Ctrl+X didn't close beta")
	}
	select {
	case <-s[1].closed:
	default:
		t.Fatal("a closed session is still listened to")
	}

	press(h.current, "esc")
	h.current.closeProject()
	if h.current != s[0] || slices.Contains(savedSessions(), dirs[2]) || lastProject() != dirs[0] {
		t.Fatal("Close Project should go back to the session before it")
	}
	h.current.closeProject()
	if h.current.noProject {
		t.Fatal("closed the last project over unsaved changes")
	}
	press(h.current, "u")
	h.current.closeProject()
	if !h.current.noProject || len(savedSessions()) != 0 || lastProject() != "" {
		t.Fatal("the last session should leave the start page")
	}
}

func TestQuitLooksAtEverySession(t *testing.T) {
	e, dirs := sessionFixture(t, "alpha", "beta")
	s := openAll(t, e, dirs)
	h := e.hub
	makeDirty(s[0])

	s[1].ex("q!")
	if s[1].quit || s[0].unsavedCount() == 0 {
		t.Fatal(":q! discarded a session out of sight")
	}
	s[1].ex("qa")
	if s[1].quit || s[0].quit || h.current != s[0] {
		t.Fatal(":qa should show the session with unsaved changes")
	}
	h.show(s[1])
	s[1].ex("qa!")
	if !s[1].quit || !s[0].discardRecovery {
		t.Fatal(":qa! should discard every session")
	}
}

func TestCleanSessionsJustQuit(t *testing.T) {
	e, dirs := sessionFixture(t, "alpha", "beta")
	s := openAll(t, e, dirs)
	s[1].ex("qa")
	if !s[1].quit {
		t.Fatal("didn't quit")
	}
}

func TestSessionsSurviveARestart(t *testing.T) {
	e, dirs := sessionFixture(t, "alpha", "beta")
	openAll(t, e, dirs)
	e.hub.finish()

	again := blankEditor(t)
	defer again.hub.finish()
	if got := savedSessions(); !slices.Equal(got, []string{dirs[1], dirs[0]}) {
		t.Fatalf("saved %v", got)
	}
	again.sessionsMenu("alpha", 0)
	if hint := again.menu.matches[0].hint; !strings.HasPrefix(hint, "open") {
		t.Fatalf("not listed as open: %q", hint)
	}
	press(again, "enter")
	now := again.hub.current
	if now.root != dirs[0] || now.b.path != filepath.Join(dirs[0], "a.txt") {
		t.Fatalf("came back at %s with %q", now.root, now.b.path)
	}
}

func TestSessionRecoveryCoversThoseOutOfSight(t *testing.T) {
	e, dirs := sessionFixture(t, "alpha", "beta")
	e.startRecovery()
	s := openAll(t, e, dirs)
	e.hub.show(s[0])
	makeDirty(s[0])
	e.hub.show(s[1])
	if err := os.Remove(s[0].recovery.path); err != nil {
		t.Fatalf("leaving didn't save recovery: %v", err)
	}
	e.hub.saveRecovery()
	if _, err := os.Stat(s[0].recovery.path); err != nil {
		t.Fatalf("no recovery for the session out of sight: %v", err)
	}
	if s[0].recovery.root == s[1].recovery.root {
		t.Fatal("sessions share a recovery folder")
	}
}

func TestSessionsFollowTheirOwnConfig(t *testing.T) {
	e, dirs := sessionFixture(t, "plain", "themed", "broken")
	if err := os.WriteFile(filepath.Join(dirs[1], ".loom.json"), []byte(`{"theme":"solarized-light","tab_size":7}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirs[2], ".loom.json"), []byte(`{"tab_size":`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := openAll(t, e, dirs)
	h := e.hub
	if s[2].cfg.TabSize == 7 || th.name == "solarized-light" {
		t.Fatal("a broken config took over the last project's settings")
	}
	h.show(s[1])
	if th.name != "solarized-light" || tabWidth != 7 {
		t.Fatalf("theme %s, tabs %d", th.name, tabWidth)
	}
	h.show(s[0])
	if th.name == "solarized-light" || tabWidth == 7 {
		t.Fatal("a project's settings followed into another session")
	}
}

func TestMacroStopsWhenTheSessionChanges(t *testing.T) {
	e, dirs := sessionFixture(t, "alpha", "beta")
	s := openAll(t, e, dirs)
	s[1].macros["q"] = []keyInput{{key: "space"}, {key: "tab"}, {key: "enter"}, {key: "x"}}
	s[1].playMacro("q")
	if e.hub.current != s[0] || s[1].unsavedCount() != 0 {
		t.Fatal("a macro went on typing out of sight")
	}
}

func TestTerminalFollowsSessions(t *testing.T) {
	e, dirs := sessionFixture(t, "alpha", "beta")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &terminalModel{editor: e, ctx: ctx, view: renderer{screen: newTerminalFrame(80, 24)}}
	m.attach()
	m.Init()
	s := openAll(t, e, dirs)
	m.Update(flashExpired{})
	if m.editor != s[1] || !m.waiting[s[0]] || !m.waiting[s[1]] {
		t.Fatal("the model didn't follow")
	}
	m.View()

	// Work finishing out of sight still lands, in its own session.
	wait := m.waitForSession(s[0])
	s[0].async <- func() { s[0].notify("done") }
	result := make(chan tea.Msg, 1)
	go func() { result <- wait() }()
	for range 50 {
		select {
		case msg := <-result:
			m.Update(msg)
			if s[0].message == "done" {
				if s[1].message == "done" {
					t.Fatal("it landed in the wrong session")
				}
				return
			}
			go func() { result <- wait() }() // something else reported first
		case <-time.After(5 * time.Second):
			t.Fatal("nothing listens to the session out of sight")
		}
	}
	t.Fatal("the callback never ran")
}

func TestSessionWithoutItsFolderCanStillBeReached(t *testing.T) {
	e, dirs := sessionFixture(t, "alpha", "beta")
	s := openAll(t, e, dirs)
	makeDirty(s[0])
	if err := os.RemoveAll(dirs[0]); err != nil {
		t.Fatal(err)
	}
	if !e.hub.show(s[0]) || e.hub.current != s[0] || s[0].unsavedCount() != 1 {
		t.Fatal("the changes can't be reached")
	}
	if !strings.Contains(s[0].message, "can't be opened") {
		t.Fatalf("no warning: %q", s[0].message)
	}
}

func TestSwitcherOpensFromEverySurface(t *testing.T) {
	for _, surface := range []string{"blank start page", "project start page", "file tree", "directory editor"} {
		t.Run(surface, func(t *testing.T) {
			e, dirs := sessionFixture(t, "alpha")
			switch surface {
			case "blank start page":
				e.showDashboard()
			case "project start page":
				e.openProject(dirs[0])
				e.showDashboard()
			case "file tree":
				openAll(t, e, dirs)
				e.tree.focused = true
			case "directory editor":
				openAll(t, e, dirs)
				e.openDirectory("")
			}
			press(e, "space tab")
			if e.menu == nil || e.menu.title != "Sessions" {
				t.Fatal("no switcher")
			}
		})
	}
}
