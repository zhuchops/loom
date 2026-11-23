//go:build desktop

package editor

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/zhuchops/loom/internal/theme"
)

// screen drives the real desktop frame without opening a window.
type screen struct {
	t     *testing.T
	d     *desktop
	e     *editor
	ops   op.Ops
	in    input.Router
	w, h  int
	scale float32
}

func (s *screen) frame() {
	s.ops.Reset()
	s.d.frame(layout.Context{
		Ops: &s.ops, Now: time.Now(), Source: s.in.Source(),
		Metric:      unit.Metric{PxPerDp: s.scale, PxPerSp: s.scale},
		Constraints: layout.Exact(image.Pt(int(float32(s.w)*s.scale), int(float32(s.h)*s.scale))),
	})
	s.in.Frame(&s.ops)
}

// settle lets git and other background work report back.
func (s *screen) settle(wait time.Duration) {
	for end := time.Now().Add(wait); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		s.d.drain()
	}
	s.frame()
	s.frame()
}

func screenFixture(t *testing.T) string {
	root := filepath.Join(t.TempDir(), "orbit")
	files := map[string]string{
		"README.md":   "# Orbit\n\nA small task service.\n\n- fast\n- simple\n",
		"go.mod":      "module example.com/orbit\n\ngo 1.24\n",
		".gitignore":  "bin/\n",
		"Makefile":    "run:\n\tgo run .\n",
		"config.yaml": "port: 8080\n",
		"main.go": "package main\n\nimport (\n\t\"fmt\"\n\t\"net/http\"\n)\n\n// routes wires every endpoint of the service.\nfunc routes() *http.ServeMux {\n\tmux := http.NewServeMux()\n" +
			"\tmux.HandleFunc(\"/health\", func(w http.ResponseWriter, r *http.Request) {\n\t\tfmt.Fprintln(w, \"ok\")\n\t})\n\treturn mux\n}\n\nfunc main() {\n\tconst port = 8080\n" +
			"\tfmt.Printf(\"listening on %d\\n\", port)\n\tif err := http.ListenAndServe(fmt.Sprintf(\":%d\", port), routes()); err != nil {\n\t\tpanic(err)\n\t}\n}\n",
		"internal/tasks/handlers.go": "package tasks\n\ntype Handler struct{}\n\nfunc (h Handler) List() []string {\n\treturn nil\n}\n",
		"internal/tasks/store.go":    "package tasks\n\ntype Store struct{}\n",
		"web/index.html":             "<!doctype html>\n<title>Orbit</title>\n",
		"web/app.ts":                 "export const name = 'orbit'\n",
	}
	for name, content := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "init"}} {
		c := exec.Command("git", a...)
		c.Dir = root
		if out, err := c.CombinedOutput(); err != nil {
			t.Skipf("git: %v %s", err, out)
		}
	}
	os.WriteFile(filepath.Join(root, "config.yaml"), []byte("port: 9090\ndebug: true\n"), 0o644)
	os.WriteFile(filepath.Join(root, "web/style.css"), []byte("body { margin: 0 }\n"), 0o644)
	root, _ = filepath.EvalSymlinks(root)
	return root
}

// settingsOn opens the settings on a page and types keys into them.
func settingsOn(page string, keys ...string) func(*screen) {
	return func(s *screen) {
		s.e.commands["settings"].run(s.e)
		s.d.settings.show(slices.IndexFunc(settingPages, func(p settingPage) bool { return p.id == page }))
		s.frame()
		for _, k := range keys {
			s.d.settingsKey(keyInput{key: k})
		}
	}
}

// changeOptions stands in for settings the user saved.
func changeOptions(change func(*options)) func(*screen) {
	return func(s *screen) {
		cfg := s.e.cfg
		change(&cfg.options)
		if err := s.e.useConfig(cfg, s.e.keys); err != nil {
			s.t.Fatal(err)
		}
	}
}

// TestDesktopScreens paints every screen. With LOOM_SHOTS=dir it also writes
// each one to a PNG, without opening a window or touching the user's config.
func TestDesktopScreens(t *testing.T) {
	out := os.Getenv("LOOM_SHOTS")
	type shot struct {
		name, theme, file string
		w, h              int
		none              bool
		do                func(*screen)
	}
	keys := func(k string) func(*screen) { return func(s *screen) { press(s.e, k) } }
	run := func(id string) func(*screen) { return func(s *screen) { s.e.commands[id].run(s.e) } }
	const src = "main.go"
	shots := []shot{
		{name: "welcome", none: true},
		{name: "welcome-small", none: true, w: 640, h: 440},
		{name: "project"},
		{name: "project-everforest", theme: "everforest"},
		{name: "editor", file: src},
		{name: "editor-everforest", file: src, theme: "everforest"},
		{name: "editor-light", file: src, theme: "solarized-light"},
		{name: "editor-terminal", file: src, theme: "terminal"},
		{name: "editor-small", file: src, w: 640, h: 440},
		{name: "insert", file: src, do: keys("j j j j i")},
		{name: "visual", file: src, do: keys("8 G V j j")},
		{name: "tabs", file: src, do: func(s *screen) {
			for _, f := range []string{"README.md", "config.yaml", "web/app.ts", "web/index.html", "go.mod"} {
				s.e.open(filepath.Join(s.e.root, f))
			}
		}},
		{name: "split", file: src, do: run("split")},
		{name: "palette", file: src, do: run("palette")},
		{name: "palette-everforest", file: src, theme: "everforest", do: run("palette")},
		{name: "palette-small", file: src, w: 640, h: 440, do: run("palette")},
		{name: "files", file: src, do: run("files")},
		{name: "files-dashboard", do: run("files")},
		{name: "empty", file: src, do: func(s *screen) { run("files")(s); press(s.e, "z q x w") }},
		{name: "themes", file: src, do: run("theme")},
		{name: "search", file: src, do: run("project-search")},
		{name: "buffers", file: src, do: run("buffers")},
		{name: "keys", file: src, do: run("keybindings")},
		{name: "projects", none: true, do: run("recent-projects")},
		{name: "git", file: src, do: run("git-panel")},
		{name: "diff", file: src, do: func(s *screen) { run("git-panel")(s); s.settle(700 * time.Millisecond); press(s.e, "enter") }},
		{name: "branch", file: src, do: run("git-branch")},
		{name: "whichkey", file: src, do: keys("space")},
		{name: "ex", file: src, do: run("ex")},
		{name: "rename-caret", file: src, do: func(s *screen) {
			s.e.openMenu(&menu{title: "Rename Symbol", input: "handleIncomingWebhookRequestForBillingAccount", exact: func(string) bool { return true }, submit: func(string) {}})
			for range 22 {
				s.e.handle(keyInput{key: "left"})
			}
		}},
		{name: "field-selection", file: src, do: func(s *screen) {
			s.e.openMenu(&menu{title: "Rename Symbol", input: "handleIncomingWebhookRequest", exact: func(string) bool { return true }, submit: func(string) {}})
			for range 7 {
				s.e.handle(keyInput{key: "left", shift: true})
			}
		}},
		{name: "find-selection", file: src, do: func(s *screen) {
			s.e.mode, s.e.b.anchor, s.e.b.cursor = "select", 0, 6
			s.e.openFindBar(true)
		}},
		{name: "explorer", file: src, do: run("explorer")},
		{name: "directory", file: src, do: run("explorer-popup")},
		{name: "find", file: src, do: keys("/ f m t")},
		{name: "toast", file: src, do: func(s *screen) { s.e.notify("Saved main.go") }},
		{name: "detail", file: src, do: func(s *screen) {
			s.e.notify("gopls: package example.com/orbit/internal/missing is not in std.\n\nRun go mod tidy to add it.")
			s.e.openNotifications()
			press(s.e, "enter")
		}},
		{name: "completion", file: src, do: func(s *screen) {
			press(s.e, "1 8 G i")
			s.frame()
			c := &s.e.completion
			c.items = c.items[:0]
			for _, it := range []completionItem{{Label: "Printf", Detail: "func(format string, a ...any)"}, {Label: "Println", Detail: "func(a ...any)"}, {Label: "Print", Detail: "func(a ...any)"}} {
				c.items = append(c.items, it)
			}
		}},
		{name: "hover", file: src, do: func(s *screen) {
			s.frame()
			s.d.m.view.diagnosticHover = diagnosticHover{buffer: s.e.b, version: s.e.b.version, x: 48, y: 10, title: "Hover Documentation",
				message: "func NewServeMux() *ServeMux\n\nNewServeMux allocates and returns a new ServeMux."}
		}},
		{name: "confirm", file: src, do: func(s *screen) { press(s.e, "i x esc"); s.d.closeTab(s.e.b) }},
		{name: "quit", file: src, do: func(s *screen) { press(s.e, "i x esc"); s.d.requestQuit() }},
		{name: "settings", file: src, do: run("settings")},
		{name: "settings-fonts", file: src, do: settingsOn("fonts")},
		{name: "settings-fonts-more", file: src, do: settingsOn("fonts", "pagedown", "pagedown")},
		{name: "settings-editor", file: src, do: settingsOn("editor")},
		{name: "settings-cursor", file: src, do: settingsOn("cursor")},
		{name: "settings-interface", file: src, do: settingsOn("interface")},
		{name: "settings-files", file: src, do: settingsOn("files")},
		{name: "settings-languages", file: src, do: settingsOn("languages", "down", "down", "enter")},
		{name: "settings-keyboard", file: src, do: settingsOn("keyboard")},
		{name: "settings-recording", file: src, do: settingsOn("keyboard", "down", "enter", "space", "f")},
		{name: "settings-picker", file: src, do: settingsOn("fonts", "enter")},
		{name: "settings-picker-ui", file: src, do: func(s *screen) {
			run("settings")(s)
			s.d.settings.show(1)
			s.d.openFontPicker(settingByKey["ui_font_family"])
		}},
		{name: "settings-search", file: src, do: settingsOn("appearance", "f", "o", "n", "t")},
		{name: "settings-light", file: src, theme: "solarized-light", do: settingsOn("fonts")},
		{name: "settings-small", file: src, w: 640, h: 440, do: settingsOn("fonts")},
		{name: "settings-none", none: true, do: run("settings")},
		{name: "fonts-changed", file: src, do: changeOptions(func(o *options) {
			o.FontFamily, o.FontSize, o.LineHeight = "Monaco", 16, 1.35
		})},
		{name: "fonts-sf-weight", file: src, do: changeOptions(func(o *options) {
			o.FontFamily, o.FontWeight, o.LetterSpacing = "SF Mono", 700, 1.5
		})},
		{name: "fonts-ui-large", file: src, do: func(s *screen) {
			changeOptions(func(o *options) { o.UIFontFamily, o.UIFontSize = "Helvetica Neue", 18 })(s)
			run("settings")(s)
		}},
		{name: "fonts-ui-small", file: src, do: changeOptions(func(o *options) { o.UIFontFamily, o.UIFontSize = "SF Pro", 11 })},
		{name: "editor-options", file: src, do: changeOptions(func(o *options) {
			o.LineNumbers, o.HighlightLine, o.PathBar, o.StatusBar = false, false, false, false
			o.CursorNormal, o.ItalicComments = "underline", true
		})},
		{name: "zen", file: src, do: run("zen")},
		{name: "sessions", file: src, theme: "everforest", do: withSessions("")},
		{name: "sessions-search", file: src, theme: "solarized-light", do: withSessions("bil")},
		{name: "sessions-small", file: src, w: 640, h: 440, do: withSessions("")},
		{name: "path-everforest", file: "internal/tasks/handlers.go", theme: "everforest", do: keys("G k k")},
		{name: "path-light", file: "internal/tasks/handlers.go", theme: "solarized-light", do: keys("G k k")},
		{name: "path-scopes", file: src, theme: "everforest", do: keys("1 2 G")},
		{name: "path-scopes-narrow", file: src, w: 640, h: 440, do: keys("1 2 G")},
		{name: "path-narrow", file: "internal/tasks/handlers.go", w: 640, h: 440, do: changeOptions(func(o *options) { o.UIFontSize = 18 })},
		{name: "replace", file: src, do: func(s *screen) { run("replace")(s); press(s.e, "f m t") }},
		{name: "results", file: src, do: func(s *screen) { run("project-search")(s); press(s.e, "f m t enter") }},
		{name: "markdown", file: "README.md", do: run("markdown-preview")},
		{name: "signature", file: src, do: func(s *screen) {
			press(s.e, "1 9 G i")
			s.e.signature = signatureHint{label: "Printf(format string, a ...any) (n int, err error)", start: 7, end: 20}
		}},
		{name: "welcome-light", none: true, theme: "solarized-light"},
		{name: "palette-light", file: src, theme: "solarized-light", do: run("palette")},
	}
	for _, c := range shots {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			name := c.theme
			if name == "" {
				name = theme.All[0].Name
			}
			if th, ok := theme.Find(name); ok {
				applyTheme(th)
			}
			t.Cleanup(func() { applyTheme(baselineTheme()) })

			root := screenFixture(t)
			t.Chdir(root)
			rememberProject(root)
			cfg := defaultConfig()
			cfg.AutoComplete = false
			cfg.RestoreSession = false
			for k, l := range cfg.Languages {
				l.Server = nil // no language servers in screenshots
				cfg.Languages[k] = l
			}
			b, err := loadBuffer("")
			if err != nil {
				t.Fatal(err)
			}
			e, err := newEditor(cfg, b, root)
			if err != nil {
				t.Fatal(err)
			}
			if c.none {
				e.enterNoProject()
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			d := newDesktop(new(app.Window), e, ctx, nil)
			e.begin(false)
			defer e.finish()
			d.run(d.m.Init())
			e.refreshGit()
			if c.file != "" {
				e.open(filepath.Join(root, c.file))
			}
			s := &screen{t: t, d: d, e: e, w: c.w, h: c.h, scale: 2}
			if s.w == 0 {
				s.w, s.h = 1200, 760
			}
			s.frame()
			s.settle(150 * time.Millisecond)
			if c.do != nil {
				c.do(s)
			}
			s.settle(500 * time.Millisecond)
			if out == "" {
				return
			}
			size := image.Pt(s.w*2, s.h*2)
			win, err := headless.NewWindow(size.X, size.Y)
			if err != nil {
				t.Skip(err)
			}
			defer win.Release()
			if err := win.Frame(&s.ops); err != nil {
				t.Fatal(err)
			}
			img := image.NewRGBA(image.Rectangle{Max: size})
			if err := win.Screenshot(img); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Join(out, fmt.Sprintf("%s.png", strings.ReplaceAll(c.name, "/", "-"))))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if err := png.Encode(f, img); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func openScreen(t *testing.T) *screen {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := screenFixture(t)
	t.Chdir(root)
	cfg := defaultConfig()
	cfg.AutoComplete, cfg.RestoreSession = false, false
	for k, l := range cfg.Languages {
		l.Server = nil
		cfg.Languages[k] = l
	}
	b, err := loadBuffer("")
	if err != nil {
		t.Fatal(err)
	}
	e, err := newEditor(cfg, b, root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	d := newDesktop(new(app.Window), e, ctx, nil)
	e.begin(false)
	d.run(d.m.Init())
	t.Cleanup(func() { cancel(); e.finish() })
	s := &screen{t: t, d: d, e: e, w: 1000, h: 640, scale: 2}
	s.frame()
	s.frame() // the first frame only asks for focus
	return s
}

func (s *screen) mouse(kind pointer.Kind, x, y int, b pointer.Buttons, scroll f32.Point) {
	s.in.Queue(pointer.Event{Kind: kind, Source: pointer.Mouse, Position: f32.Pt(float32(x), float32(y)), Buttons: b, Scroll: scroll})
	s.frame()
}

func (s *screen) click(r image.Rectangle) {
	x, y := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
	s.mouse(pointer.Move, x, y, 0, f32.Point{})
	s.mouse(pointer.Press, x, y, pointer.ButtonPrimary, f32.Point{})
	s.mouse(pointer.Release, x, y, 0, f32.Point{})
	s.frame()
}

func (s *screen) treeRow(name string) image.Rectangle {
	for i, row := range s.e.tree.rows {
		if filepath.Base(row.path) == name {
			r := s.d.geo.sidebar
			top := r.Min.Y + s.d.u(36) + (i-s.e.tree.scroll)*s.d.u(25)
			return image.Rect(r.Min.X+s.d.u(10), top, r.Max.X-s.d.u(10), top+s.d.u(25))
		}
	}
	s.t.Fatalf("%s is not in the tree", name)
	return image.Rectangle{}
}

func (s *screen) typeText(text string) {
	for _, r := range text {
		s.in.Queue(key.EditEvent{Text: string(r)})
		s.frame()
	}
}

func (s *screen) key(name key.Name, mods key.Modifiers) {
	s.in.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	s.frame()
}

func TestDesktopPointer(t *testing.T) {
	s := openScreen(t)
	e, d := s.e, s.d
	var long strings.Builder
	for i := range 300 {
		fmt.Fprintf(&long, "line %d\n", i)
	}
	if err := os.WriteFile(filepath.Join(e.root, "long.txt"), []byte(long.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	e.refreshTree("")
	s.frame()

	s.click(s.treeRow("long.txt"))
	if filepath.Base(e.b.path) != "long.txt" {
		t.Fatalf("clicking the sidebar opened %q", e.b.path)
	}

	o := d.geo.grid.Min
	x, y := o.X+d.cellX(14), o.Y+d.cellH*4+d.cellH/2
	s.click(image.Rect(x-1, y-1, x+1, y+1))
	if row := e.b.rowAt(e.b.cursor); row != 4 {
		t.Fatalf("clicking the fifth line put the cursor on row %d", row)
	}
	if e.tree.focused {
		t.Fatal("clicking the text left the sidebar focused")
	}

	s.mouse(pointer.Scroll, x, y, 0, f32.Pt(0, float32(12*d.cellH)))
	if e.b.top < 8 {
		t.Fatalf("the wheel scrolled to line %d", e.b.top)
	}

	s.click(s.treeRow("main.go"))
	if filepath.Base(e.b.path) != "main.go" || len(d.tabs) != 3 {
		t.Fatalf("got %q with %d tabs", e.b.path, len(d.tabs))
	}

	// Drag the last tab in front of the first.
	from, to := d.tabs[2].r, d.tabs[0].r
	cy := (from.Min.Y + from.Max.Y) / 2
	s.mouse(pointer.Press, from.Min.X+d.u(40), cy, pointer.ButtonPrimary, f32.Point{})
	s.mouse(pointer.Move, to.Min.X+d.u(5), cy, pointer.ButtonPrimary, f32.Point{})
	s.mouse(pointer.Release, to.Min.X+d.u(5), cy, 0, f32.Point{})
	if filepath.Base(e.buffers[0].path) != "main.go" {
		t.Fatal("dragging did not reorder the tabs")
	}

	s.frame()
	for _, tab := range d.tabs {
		if tab.b == e.b {
			s.click(image.Rect(tab.r.Max.X-d.u(22), tab.r.Min.Y, tab.r.Max.X-d.u(14), tab.r.Max.Y))
		}
	}
	for _, b := range e.buffers {
		if filepath.Base(b.path) == "main.go" {
			t.Fatal("the close button did not close the tab")
		}
	}

	// A click outside a popup dismisses it and does not reach what is beneath.
	e.openPalette()
	s.frame()
	before := e.b
	s.click(s.treeRow("main.go"))
	if e.menu != nil || e.b != before {
		t.Fatal("a click outside the palette went through it")
	}
}

func TestDesktopKeyboard(t *testing.T) {
	s := openScreen(t)
	e := s.e
	e.open(filepath.Join(e.root, "main.go"))
	s.frame()

	s.typeText("i")
	s.typeText("привет, мир")
	if e.mode != "insert" || !strings.HasPrefix(string(e.b.text), "привет, мирpackage") {
		t.Fatalf("typing gave %q", string(e.b.text[:20]))
	}
	// Return and the letter after it can land in the same frame.
	s.in.Queue(key.Event{Name: key.NameReturn, State: key.Press}, key.EditEvent{Text: "x"})
	s.frame()
	if !strings.HasPrefix(string(e.b.text), "привет, мир\nxpackage") {
		t.Fatalf("a letter typed straight after Return was lost: %q", string(e.b.text[:24]))
	}
	s.key(key.NameEscape, 0)
	if e.mode != "normal" {
		t.Fatal("Esc did not leave insert mode")
	}

	s.key("З", key.ModCtrl) // Ctrl+P on a Russian layout
	if e.menu == nil || e.menu.title != "Go to File" {
		t.Fatal("Ctrl+P on a Russian layout did not open Go to File")
	}
	s.key(key.NameEscape, 0)

	s.d.requestQuit()
	s.frame()
	if s.d.dialog == nil || e.quit {
		t.Fatal("quitting with unsaved changes did not ask")
	}
	s.typeText("x")
	s.key(key.NameEscape, 0)
	if s.d.dialog != nil || e.quit {
		t.Fatal("Esc did not cancel quitting")
	}
}

func TestDesktopSaveAsReplaces(t *testing.T) {
	s := openScreen(t)
	e := s.e
	s.d.newFile()
	e.handle(keyInput{paste: "fresh\n"})
	target := filepath.Join(e.root, "config.yaml")
	s.d.writeTo(e.b, target)
	if got, err := os.ReadFile(target); err != nil || string(got) != "fresh\n" || e.b.dirty() {
		t.Fatalf("Save As wrote %q (%v)", got, err)
	}
}

func BenchmarkDesktopFrame(b *testing.B) {
	root := b.TempDir()
	path := filepath.Join(root, "big.go")
	if err := os.WriteFile(path, []byte("package big\n"+strings.Repeat("func f(a, b int) int { return a + b } // adds two numbers\n", 10000)), 0o644); err != nil {
		b.Fatal(err)
	}
	b.Setenv("XDG_STATE_HOME", b.TempDir())
	cfg := defaultConfig()
	for k, l := range cfg.Languages {
		l.Server = nil
		cfg.Languages[k] = l
	}
	buf, err := loadBuffer(path)
	if err != nil {
		b.Fatal(err)
	}
	e, err := newEditor(cfg, buf, root)
	if err != nil {
		b.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &screen{d: newDesktop(new(app.Window), e, ctx, nil), e: e, w: 1440, h: 900, scale: 2}
	for b.Loop() {
		s.frame()
	}
}

func TestDesktopSettings(t *testing.T) {
	s := openScreen(t)
	e, d := s.e, s.d
	t.Cleanup(func() { applyTheme(baselineTheme()) })
	e.open(filepath.Join(e.root, "main.go"))
	s.frame()

	d.command("settings")
	s.frame()
	if d.settings == nil {
		t.Fatal("the settings didn't open")
	}

	// Arrows on the theme row pick the default theme.
	s.key(key.NameRightArrow, 0)
	saved, err := readConfig(e.userConfigPath(), false)
	if err != nil || saved.Theme == "" || saved.Theme != th.name {
		t.Fatalf("saved theme %q, showing %q (%v)", saved.Theme, th.name, err)
	}

	// Searching and changing, all from the keyboard.
	s.typeText("code size")
	cell, text := d.cellW, string(e.b.text)
	s.key(key.NameRightArrow, 0)
	s.frame()
	if e.cfg.FontSize != 13.5 || d.cellW <= cell {
		t.Fatalf("font size %v, cell %v after %v", e.cfg.FontSize, d.cellW, cell)
	}
	if string(e.b.text) != text || e.mode != "normal" {
		t.Fatal("keys typed into the settings reached the editor")
	}

	// Tab leaves the search for the next page, and a click flips a switch.
	s.key(key.NameTab, 0)
	if d.settings.query != "" || settingPages[d.settings.page].id != "fonts" {
		t.Fatalf("Tab went to page %d", d.settings.page)
	}
	d.settings.show(slices.IndexFunc(settingPages, func(p settingPage) bool { return p.id == "editor" }))
	s.frame()
	card := d.centred(d.u(960), d.u(670))
	x := card.Max.X - d.u(40)
	y := card.Min.Y + d.u(56) + d.u(8) + d.u(42) + d.u(23)
	s.click(image.Rect(x-1, y-1, x+1, y+1))
	if e.cfg.LineNumbers {
		t.Fatal("clicking the switch didn't turn line numbers off")
	}

	// Rebinding records keys, Space included.
	d.settings.show(slices.IndexFunc(settingPages, func(p settingPage) bool { return p.id == "keyboard" }))
	d.settings.recording = &keyRecording{id: "theme", title: "Color Theme"}
	s.frame()
	s.typeText(" T")
	s.key(key.NameReturn, 0)
	if d.settings.recording != nil || e.keys["vim.normal"]["space T"] != "theme" {
		t.Fatalf("rebinding failed: %q", d.settings.failure)
	}

	// The font picker saves the family it lands on.
	d.settings.show(1)
	s.key(key.NameReturn, 0)
	if d.settings.picker == nil {
		t.Fatal("Enter didn't open the font picker")
	}
	if choices := d.fontChoices(d.settings.picker); len(choices) > 1 {
		s.key(key.NameDownArrow, 0)
		s.key(key.NameReturn, 0)
		if e.cfg.FontFamily != choices[1].name || d.codeFamily.name != choices[1].name {
			t.Fatalf("picked %q, saved %q", choices[1].name, e.cfg.FontFamily)
		}
	}

	s.key(key.NameEscape, 0)
	if d.settings != nil {
		t.Fatal("Esc didn't close the settings")
	}

	d.zoom(1)
	if got, err := readConfig(e.userConfigPath(), false); err != nil || got.FontSize != 14.5 {
		t.Fatalf("zoom wasn't saved: %v (%v)", got.FontSize, err)
	}
}

func TestPathBarKeepsTheFileName(t *testing.T) {
	s := openScreen(t)
	s.frame()
	s.d.gtx.Ops = &s.d.scratch
	dirs := []string{"internal", "users", "handlers", "v2"}
	if got := s.d.fitFolders(dirs, 1<<20); !slices.Equal(got, dirs) {
		t.Fatalf("with room: %v", got)
	}
	room := s.d.width("…", textPath) + s.d.width("v2", textPath) + 2*s.d.u(22)
	if got := s.d.fitFolders(dirs, room); !slices.Equal(got, []string{"…", "v2"}) {
		t.Fatalf("short of room: %v", got)
	}
	if got := s.d.fitFolders(dirs, 0); len(got) != 0 {
		t.Fatalf("no room: %v", got)
	}
}

func TestPathBarTextReadsInEveryTheme(t *testing.T) {
	t.Cleanup(func() { applyTheme(baselineTheme()) })
	for _, th := range theme.All {
		applyTheme(th)
		d := &desktop{c: deriveColors()}
		band := d.pathFill()
		if band == d.c.bg {
			t.Errorf("%s: the path bar doesn't stand out", th.Name)
		}
		if got := contrast(readable(d.c.dim, band, 4.5), band); got < 4.4 {
			t.Errorf("%s: folders at %.2f", th.Name, got)
		}
	}
}

// withSessions opens two more projects, leaves one unsaved and shows the
// switcher from the first.
func withSessions(query string) func(*screen) {
	return func(s *screen) {
		first, parent := s.d.m.editor, s.t.TempDir()
		// Switching loads the theme that was chosen, so choose this one.
		if err := os.MkdirAll(filepath.Dir(themeStatePath()), 0o755); err != nil {
			s.t.Fatal(err)
		}
		if err := os.WriteFile(themeStatePath(), []byte(th.name), 0o644); err != nil {
			s.t.Fatal(err)
		}
		for _, name := range []string{"billing-api", "design-system"} {
			dir := filepath.Join(parent, name)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				s.t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
				s.t.Fatal(err)
			}
			s.d.m.editor.openProject(dir)
			e := s.d.m.editor
			e.open(filepath.Join(e.root, "main.go"))
			if name == "billing-api" {
				e.handle(keyInput{key: "x"})
			}
		}
		h := first.hub
		h.show(first)
		first.sessionsMenu(query, 0)
		s.frame()
	}
}

func TestDesktopSessions(t *testing.T) {
	s := openScreen(t)
	d, first := s.d, s.e
	withSessions("")(s)
	h := first.hub
	i := slices.IndexFunc(h.live, func(e *editor) bool { return filepath.Base(e.root) == "billing-api" })
	if i < 0 {
		t.Fatal("billing-api has no session")
	}
	billing := h.live[i]

	// Enter goes to the session before this one, and the window follows.
	s.key(key.NameReturn, 0)
	s.frame()
	if d.m.editor == first || d.shown != d.m.editor || filepath.Base(d.m.editor.root) != "design-system" {
		t.Fatalf("Enter showed %s", d.m.editor.root)
	}

	// Typing finds one, and a click picks it.
	d.command("sessions")
	s.frame()
	s.typeText("blg")
	m := d.m.editor.menu
	if m == nil || len(m.matches) == 0 || m.matches[0].label != "billing-api" {
		t.Fatal("the switcher didn't find billing-api")
	}
	s.key(key.NameReturn, 0)
	s.frame()
	if d.m.editor != billing {
		t.Fatalf("picked %s", d.m.editor.root)
	}

	// Typing lands in the session on screen and nowhere else.
	before := first.b.version
	s.typeText("x")
	if first.b.version != before {
		t.Fatal("typing reached a session out of sight")
	}

	// Quitting sees the sessions out of sight, and Save All saves them.
	h.show(first)
	s.frame()
	d.requestQuit()
	if d.dialog == nil || first.quit || !strings.Contains(d.dialog.body, "billing-api") {
		t.Fatal("quitting didn't ask about a session out of sight")
	}
	save := d.dialog.choices[0].run
	d.dialog = nil
	save()
	if !d.m.editor.quit || billing.unsavedCount() != 0 {
		t.Fatal("Save All didn't save and quit")
	}
	if raw, err := os.ReadFile(filepath.Join(billing.root, "main.go")); err != nil || string(raw) == "package main\n" {
		t.Fatalf("billing-api wasn't written: %q (%v)", raw, err)
	}
}

func TestElixirFileMarks(t *testing.T) {
	d := &desktop{}
	for _, path := range []string{"a.ex", "test/a_test.exs", "mix.exs", "page.html.heex", "main.dingo"} {
		if mark, _ := d.fileMark(path); mark == "" {
			t.Errorf("%s has no icon", path)
		}
	}
}

func TestDesktopOpensWhatALaterLaunchSends(t *testing.T) {
	state, err := os.MkdirTemp("/tmp", "ls") // short: a socket path has a length limit
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(state) })
	t.Setenv("XDG_STATE_HOME", state)
	s := openScreen(t)
	d, first := s.d, s.e
	withSessions("")(s)
	h := first.hub
	var billing, design *editor
	for _, e := range h.live {
		switch filepath.Base(e.root) {
		case "billing-api":
			billing = e
		case "design-system":
			design = e
		}
	}
	// A file goes to the project that holds it, even when another is shown.
	h.show(design)
	notes := filepath.Join(billing.root, "NOTES.txt")
	if err := os.WriteFile(notes, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d.openRequested(openRequest{Cwd: design.root, Paths: []string{"../billing-api/NOTES.txt"}})
	if h.current != billing || h.current.b.path != resolveOpenPath("/", notes) {
		t.Fatalf("shown %s, buffer %q", h.current.root, h.current.b.path)
	}
	// A folder opens as a session, or shows the one it already has.
	n := len(h.live)
	d.openRequested(openRequest{Cwd: "/", Paths: []string{design.root}})
	if h.current != design || len(h.live) != n {
		t.Fatalf("an open folder was not shown: %s, %d sessions", h.current.root, len(h.live))
	}
	fresh := filepath.Join(t.TempDir(), "fresh")
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	d.openRequested(openRequest{Cwd: "/", Paths: []string{fresh}})
	if len(h.live) != n+1 || filepath.Base(h.current.root) != "fresh" {
		t.Fatalf("a new folder did not become a session: %d sessions", len(h.live))
	}

	// The socket carries a request from a second launch to the first.
	got := make(chan openRequest, 1)
	listenForRequests(func(r openRequest) { got <- r })
	if !forwardToRunning([]string{"a.txt", "dir"}) {
		t.Fatal("a running Loom did not take the launch")
	}
	if r := <-got; len(r.Paths) != 2 || r.Cwd == "" {
		t.Fatalf("request = %+v", r)
	}
	// With -new a process neither sends nor listens.
	newWindow = true
	defer func() { newWindow = false }()
	if info, err := os.Stat(ipcSocket()); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("socket: %v %v", info, err)
	}
}

func TestDesktopWheelLeavesTheCursorOffScreen(t *testing.T) {
	s := openScreen(t)
	e := s.e
	var sb strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "line %d\n", i)
	}
	e.open(fixture(t, e.root, "long.txt", sb.String()))
	s.frame()
	b := e.b
	b.cursor = 0
	s.frame()
	if b.top != 0 {
		t.Fatalf("top = %d", b.top)
	}
	mid := image.Pt(s.w*3/4, s.h/2)
	for i := 0; i < 40; i++ {
		s.mouse(pointer.Scroll, mid.X, mid.Y, 0, f32.Pt(0, 60))
		s.frame()
	}
	top := b.top
	if top < 20 {
		t.Fatalf("the wheel scrolled to %d only", top)
	}
	if b.cursor != 0 {
		t.Fatalf("scrolling moved the cursor to %d", b.cursor)
	}
	for i := 0; i < 5; i++ {
		s.frame()
		time.Sleep(20 * time.Millisecond)
	}
	if b.top != top {
		t.Fatalf("the view snapped back from %d to %d", top, b.top)
	}
	// Whatever happens in the background, and the pointer resting on the text,
	// leaves the view where it was scrolled to.
	e.checkDisk()
	e.refreshGit()
	e.refreshLanguage()
	e.notify("something happened")
	s.mouse(pointer.Move, mid.X, mid.Y, 0, f32.Point{})
	s.mouse(pointer.Move, mid.X+10, mid.Y+10, 0, f32.Point{})
	s.d.send(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeftShift}))
	s.frame()
	if b.top != top || b.cursor != 0 {
		t.Fatalf("the view moved from %d to %d, cursor %d", top, b.top, b.cursor)
	}
	if s.d.m.view.screen.cursorVisible {
		t.Fatalf("the cursor is drawn at %d,%d (top %d, row %d, mode %s, tree focused %v)", s.d.m.view.screen.cursorX, s.d.m.view.screen.cursorY, b.top, b.rowAt(b.cursor), e.mode, e.tree.focused)
	}
}
