package editor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func fixture(t *testing.T, root, name, content string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func awaitUpdate(t *testing.T, e *editor, done func() bool) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for !done() {
		select {
		case update := <-e.async:
			update()
		case <-timeout:
			t.Fatalf("update timed out: %s", e.message)
		}
	}
}

func TestExplorerFileOperations(t *testing.T) {
	e := testEditor(t, "vim", "")
	t.Setenv("HOME", t.TempDir())
	press(e, "space e")
	if !e.tree.focused {
		t.Fatal("popup did not open")
	}
	press(e, "g g A s r c enter a a . t x t enter")
	a := filepath.Join(e.root, "src", "a.txt")
	if _, err := os.Stat(a); err != nil {
		t.Fatal(err)
	}
	press(e, "r ctrl+u b . t x t enter")
	b := filepath.Join(e.root, "src", "b.txt")
	if e.tree.rows[e.tree.selected].path != b {
		t.Fatal(e.message)
	}
	press(e, "x g g p")
	moved := filepath.Join(e.root, "b.txt")
	if _, err := os.Stat(moved); err != nil {
		t.Fatal(err)
	}
	press(e, "enter")
	if e.b.path != moved || e.tree.popup {
		t.Fatal("file did not open")
	}
	press(e, "space e d y e s enter")
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".Trash", "b.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(moved); !os.IsNotExist(err) {
		t.Fatal("file still in project")
	}
}

func TestRenderAndMouse(t *testing.T) {
	e := testEditor(t, "vim", "")
	e.cfg.Languages = nil
	path := fixture(t, e.root, "main.go", "package main\n\nfunc run(who string) string { return \"hello\" + who }\n")
	e.open(path)
	screen := newTerminalFrame(110, 26)
	v := renderer{screen: screen}
	v.draw(e)
	pane := v.panes[0]
	x, y := pane.x+pane.gutter, pane.y+2
	_, keyword, _ := screen.Get(x, y)
	_, function, _ := screen.Get(x+5, y)
	_, parameter, _ := screen.Get(x+9, y)
	if keyword.GetForeground().Hex() != 0xe67e80 || function.GetForeground().Hex() != 0xa7c080 || parameter.GetForeground().Hex() != 0xe69875 {
		t.Fatalf("wrong syntax colors: %v %v %v", keyword, function, parameter)
	}
	v.mouse(e, pointerEvent{x: x + 5, y: y, leftDown: true})
	v.mouse(e, pointerEvent{x: x + 7, y: y, leftDown: true})
	v.mouse(e, pointerEvent{x: x + 7, y: y})
	s := e.b.selection()
	if e.mode != "select" || string(e.b.text[s.start:s.end]) != "run" {
		t.Fatal("mouse drag did not select run")
	}
	old := e.tree.width
	v.mouse(e, pointerEvent{x: v.divider, y: 5, leftDown: true})
	v.mouse(e, pointerEvent{x: old + 5, y: 5, leftDown: true})
	v.mouse(e, pointerEvent{x: old + 5, y: 5})
	if e.tree.width != old+5 {
		t.Fatal("sidebar did not resize")
	}
	press(e, "esc")
	v.draw(e)
	pane = v.panes[0]
	x, y = pane.x+pane.gutter+5, pane.y+2
	for range 2 {
		v.mouse(e, pointerEvent{x: x, y: y, leftDown: true})
		v.mouse(e, pointerEvent{x: x, y: y})
	}
	s = e.b.selection()
	if e.mode != "select" || string(e.b.text[s.start:s.end]) != "run" {
		t.Fatal("double click did not select the word")
	}
	if !e.open(fixture(t, e.root, "notes.txt", strings.Repeat("some notes\n", 65))) {
		t.Fatal(e.message)
	}
	v.draw(e)
	pane = v.panes[0]
	v.mouse(e, pointerEvent{x: pane.x + pane.gutter, y: pane.y + 2, button: wheelDown})
	if e.b.top == 0 {
		t.Fatal("editor did not scroll")
	}
	tab := v.tabs[0]
	v.mouse(e, pointerEvent{x: tab.x + 2, y: tab.y, leftDown: true})
	v.mouse(e, pointerEvent{x: tab.x + 2, y: tab.y})
	if e.b.path != path {
		t.Fatal("tab click did not switch file")
	}
	press(e, "esc space e")
	v.draw(e)
	tree := v.treeBox
	v.mouse(e, pointerEvent{x: tree.x + 5, y: tree.y + 1, button: mouseRight})
	if e.menu == nil || e.menu.title != "Explorer Actions" {
		t.Fatal("missing explorer context menu")
	}
}

func TestSaveRepeatAndSession(t *testing.T) {
	e := testEditor(t, "vim", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	e.cfg.Languages = nil
	first := fixture(t, e.root, "first.go", "package p\nfunc main(){println(1)}\n")
	second := fixture(t, e.root, "second.txt", "hello\nthere\n")
	e.open(first)
	if !e.save() {
		t.Fatal(e.message)
	}
	raw, _ := os.ReadFile(first)
	if !strings.Contains(string(raw), "func main() { println(1) }") {
		t.Fatalf("missing formatting: %s", raw)
	}
	e.open(second)
	press(e, "A ! ! esc u")
	if got := string(e.b.text); got != "hello\nthere\n" {
		t.Fatalf("typing undo: %q", got)
	}
	press(e, "j .")
	if got := string(e.b.text); got != "hello\nthere!!\n" {
		t.Fatalf("repeat: %q", got)
	}
	press(e, "u")
	e.splitPane(true, first)
	if len(e.layout.leaves()) != 2 || e.b.path != first {
		t.Fatal("split needs two separate views")
	}
	e.saveSession()
	blank, _ := loadBuffer("")
	restored, err := newEditor(e.cfg, blank, e.root)
	if err != nil {
		t.Fatal(err)
	}
	restored.restoreSession(false)
	if len(restored.buffers) != 2 || len(restored.layout.leaves()) != 2 || restored.b.path != e.b.path || restored.layout.leaves()[0].cursor != e.layout.leaves()[0].cursor {
		t.Fatal("tabs, split, or cursor failed to restore")
	}
}

func TestProjectSearchAndGitDiff(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep missing")
	}
	e := testEditor(t, "vim", "")
	path := fixture(t, e.root, "notes.txt", "header\nfind-me\n")
	e.projectSearch("find-me")
	awaitUpdate(t, e, func() bool { return e.menu != nil && len(e.menu.options) == 1 })
	if e.message != "" {
		t.Fatalf("successful search showed %q", e.message)
	}
	press(e, "enter")
	if e.b.path != path || e.b.rowAt(e.b.cursor) != 1 {
		t.Fatal("search did not open matching line")
	}
	if _, err := exec.LookPath("git"); err == nil {
		if marks := changedLines("first\nsecond\n", "first\nchanged\n"); marks[1] != '~' {
			t.Fatalf("incorrect diff marks: %v", marks)
		}
	}
}

func TestGoLanguageIntegration(t *testing.T) {
	if os.Getenv("LOOM_TEST_GOPLS") == "" {
		t.Skip("set LOOM_TEST_GOPLS=1 to run")
	}
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Fatal(err)
	}
	e := testEditor(t, "vim", "")
	fixture(t, e.root, "go.mod", "module test.local/loom\n\ngo 1.26\n")
	path := fixture(t, e.root, "main.go", "package main\nimport \"fmt\"\nfunc greet(who string, count int) string { return who }\nfunc main(){fmt.Println(greet( \"hello\", 2))}\n")
	e.open(path)
	t.Cleanup(e.closeServers)
	e.refreshLanguage()
	awaitUpdate(t, e, func() bool { return !e.languageRunning })
	if len(e.b.semantic) == 0 {
		t.Fatal("gopls returned no semantic colors")
	}
	old := e.session(e.b, false)
	oldClient := old.client
	// Restart through the public command while the active buffer has unsaved text.
	e.b.replace(span{0, 0}, "// unsaved\n")
	e.ex("lsp-restart")
	fresh := e.session(e.b, false)
	if fresh == old || !fresh.restarting {
		t.Fatal("restart did not replace the language server session")
	}
	awaitUpdate(t, e, func() bool { return !fresh.restarting })
	if e.message != "Language server restarted" || !e.b.dirty() {
		t.Fatalf("restart failed or lost the active edit: %s", e.message)
	}
	select {
	case <-oldClient.Done():
	default:
		t.Fatal("old language server is still running")
	}
	awaitUpdate(t, e, func() bool { return e.b.semanticVersion == e.b.version })
	// Restore the original lines for the remaining navigation assertions.
	e.b.history(false)
	e.b.cursor = e.b.lines[3] + strings.Index(string(e.b.text[e.b.lines[3]:e.b.lineEnd(3)]), "greet")
	call := e.b.cursor
	e.b.cursor = call + len("greet(")
	model := &terminalModel{editor: e}
	_, wake := model.Update(tea.KeyPressMsg{Code: 'i'})
	_, request := model.Update(wake())
	first := request()
	model.Update(first)
	if hint := e.signature; hint.label[hint.start:hint.end] != "who string" {
		t.Fatalf("first argument hint = %+v", hint)
	}
	e.b.cursor = call + len("greet( \"he")
	_, wake = model.Update(flashExpired{})
	_, request = model.Update(wake())
	model.Update(request())
	if hint := e.signature; hint.label[hint.start:hint.end] != "who string" {
		t.Fatalf("hint while typing the string = %+v", hint)
	}
	e.b.cursor = call + len("greet( \"hello\",")
	_, wake = model.Update(flashExpired{})
	_, request = model.Update(wake())
	model.Update(request())
	model.Update(first)
	if hint := e.signature; hint.label[hint.start:hint.end] != "count int" {
		t.Fatalf("second argument hint = %+v", hint)
	}
	model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if e.signature.label != "" {
		t.Fatal("signature hint remained after leaving Insert")
	}
	e.b.cursor = call
	e.languageAction("hover")
	awaitUpdate(t, e, func() bool { return e.menu != nil })
	if e.menu.title != "Hover Documentation" {
		t.Fatal(e.menu.title)
	}
	e.menu = nil
	e.languageAction("references")
	awaitUpdate(t, e, func() bool { return e.menu != nil })
	if len(e.menu.options) < 2 {
		t.Fatal("missing reference results")
	}
	e.menu = nil
	e.languageAction("definition")
	awaitUpdate(t, e, func() bool { return e.b.rowAt(e.b.cursor) == 2 })
	b := e.b
	b.replace(span{b.lines[3], len(b.text)}, "func main(){fmt.Printl\n}\n")
	b.cursor = b.lineEnd(3)
	s := e.session(b, true)
	items, err := s.complete(e.document(b), 1, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Label == "Println" {
			if err := applyCompletion(b, item); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b.text), "fmt.Println") {
				t.Fatal("completion did not apply")
			}
			s.mu.Lock()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			var semantic semanticResult
			err := s.sync(e.document(b))
			if err == nil {
				err = s.client.Request(ctx, "textDocument/semanticTokens/full", docParams(e.document(b)), &semantic)
			}
			cancel()
			s.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			awaitUpdate(t, e, func() bool { return len(b.diagnostics) > 0 })
			return
		}
	}
	t.Fatal("gopls did not offer Println")
}

func TestEditableDirectory(t *testing.T) {
	e := testEditor(t, "vim", "")
	t.Setenv("HOME", t.TempDir())
	dir := filepath.Join(e.root, "src")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	old := fixture(t, e.root, "a.txt", "keep my contents")
	doomed := fixture(t, e.root, "b.txt", "send to trash")
	press(e, "space x")
	if e.directory == nil {
		t.Fatal("missing directory buffer")
	}
	press(e, "j j c c s r c / a . t x t esc j d d o c . t x t esc")
	if _, err := os.Stat(old); err != nil {
		t.Fatal("edit reached disk before save")
	}
	press(e, ": w enter")
	if e.menu == nil || len(e.menu.options) != 3 {
		t.Fatalf("wrong plan: %s; %q", e.message, e.directory.doc.b.text)
	}
	press(e, "enter")
	path := filepath.Join(dir, "a.txt")
	if text, err := os.ReadFile(path); err != nil || string(text) != "keep my contents" {
		t.Fatalf("move: %q %v %s", text, err, e.message)
	}
	if _, err := os.Stat(filepath.Join(e.root, "c.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(doomed); !os.IsNotExist(err) {
		t.Fatal("removed file still present")
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".Trash", "b.txt")); err != nil {
		t.Fatal(err)
	}
	press(e, "g g j enter j enter")
	if e.directory != nil || e.b.path != path {
		t.Fatalf("did not open: %v %s", e.b.path, e.message)
	}
}

func TestPaletteExAndDirectory(t *testing.T) {
	e := testEditor(t, "vim", "hello\nworld\nhello\n")
	press(e, ": 2 enter")
	if e.b.rowAt(e.b.cursor) != 1 {
		t.Fatal("line address failed")
	}
	press(e, "/ h e l l o enter")
	if !e.searchVisible {
		t.Fatal("missing search highlight")
	}
	press(e, ": n o h enter")
	if e.searchVisible {
		t.Fatal("noh failed")
	}
	press(e, "n")
	if !e.searchVisible {
		t.Fatal("next match should restore highlight")
	}
	file := fixture(t, e.root, "first.txt", "data")
	press(e, "space x")
	press(e, "j 0 c i w s e c o n d esc ctrl+s")
	if e.menu == nil || len(e.menu.options) != 1 {
		t.Fatalf("expected one rename: %s", e.message)
	}
	press(e, "enter")
	if text, err := os.ReadFile(filepath.Join(e.root, "second.txt")); err != nil || string(text) != "data" {
		t.Fatalf("rename: %q %v %s", text, err, e.message)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("original name still present")
	}
	press(e, "o a d d e d esc esc")
	if e.directory == nil {
		t.Fatal("dirty directory should stay open")
	}
	press(e, ": q ! enter")
	if e.directory != nil {
		t.Fatal("force-close failed")
	}
}

func TestCenteredPaletteAndWhichKey(t *testing.T) {
	e := testEditor(t, "vim", "first\nsecond\n")
	s := newTerminalFrame(100, 30)
	v := renderer{screen: s}
	press(e, ":")
	v.draw(e)
	box := centerBox(100, 30, 78, 15)
	if text, _, _ := s.Get(box.x, box.y); text != "╭" {
		t.Fatalf("no border at centered position: %q", text)
	}
	if v.previewBox.h != 0 {
		t.Fatal("command palette displayed a preview")
	}
	_, dimmed, _ := s.Get(0, 0)
	if dimmed.GetBackground().Hex() == panelStyle.GetBackground().Hex() {
		t.Fatal("workspace was not dimmed")
	}
	press(e, "esc space")
	v.draw(e)
	found := false
	for y := 0; y < 29 && !found; y++ {
		for x := 0; x < 100; x++ {
			if text, _, _ := s.Get(x, y); text == "╭" && x > 0 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("which-key popup did not appear")
	}
	press(e, "x")
	v.draw(e)
	if v.directoryBox.x <= 0 || v.directoryBox.y <= 0 {
		t.Fatal("directory box is not centered")
	}
}

func TestDirectoryRenameSwap(t *testing.T) {
	e := testEditor(t, "vim", "")
	a := fixture(t, e.root, "a.txt", "original a")
	b := fixture(t, e.root, "b.txt", "original b")
	press(e, "space x j c c b . t x t esc j c c a . t x t esc ctrl+s enter")
	for path, want := range map[string]string{a: "original b", b: "original a"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v; %s", path, got, err, e.message)
		}
	}
}
