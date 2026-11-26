//go:build desktop

package editor

import (
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The window's place in the system: the macOS menu bar, launches from other
// terminals, and closing with unsaved work.

// appMenu is the macOS menu bar. Its items run the commands of the window
// in front; elsewhere the keymap has the Ctrl shortcuts and there is none.
func appMenu() *application.Menu {
	if runtime.GOOS != "darwin" {
		return nil
	}
	front := func(f func(b *Bridge)) func(*application.Context) {
		return func(*application.Context) {
			if b := frontBridge(); b != nil {
				f(b)
			}
		}
	}
	item := func(m *application.Menu, label, id, keys string) {
		it := m.Add(label).OnClick(front(func(b *Bridge) { b.Command(id) }))
		if keys != "" {
			it.SetAccelerator(keys)
		}
	}
	window := func(f func(w *application.WebviewWindow)) func(*application.Context) {
		return func(*application.Context) {
			if w, ok := desktop.app.Window.Current().(*application.WebviewWindow); ok && w != nil {
				f(w)
			}
		}
	}
	bar := desktop.app.Menu.New()

	m := bar.AddSubmenu("Loom")
	m.Add("About Loom").OnClick(func(*application.Context) { desktop.app.Menu.ShowAbout() })
	m.AddSeparator()
	item(m, "Settings…", "settings", "CmdOrCtrl+,")
	item(m, "Color Theme…", "theme", "")
	m.AddSeparator()
	m.Add("Hide Loom").SetAccelerator("CmdOrCtrl+H").OnClick(func(*application.Context) { desktop.app.Hide() })
	m.AddSeparator()
	m.Add("Quit Loom").SetAccelerator("CmdOrCtrl+Q").OnClick(func(*application.Context) { quitApp() })

	m = bar.AddSubmenu("File")
	item(m, "New File", "new-file", "CmdOrCtrl+N")
	item(m, "New Window", "new-window", "CmdOrCtrl+Shift+N")
	item(m, "Open File…", "open-file", "CmdOrCtrl+O")
	item(m, "Open Project…", "open-project", "CmdOrCtrl+Shift+O")
	item(m, "Recent Projects…", "projects", "")
	item(m, "Switch Session…", "sessions", "CmdOrCtrl+K")
	item(m, "Move Session to New Window", "session-window", "")
	m.AddSeparator()
	item(m, "Save", "save", "CmdOrCtrl+S")
	item(m, "Save As…", "save-as", "CmdOrCtrl+Shift+S")
	m.AddSeparator()
	item(m, "Close Tab", "close-tab", "CmdOrCtrl+W")
	item(m, "Close Session", "close-project", "")
	m.Add("Close Window").SetAccelerator("CmdOrCtrl+Shift+W").OnClick(window(func(w *application.WebviewWindow) { w.Close() }))

	m = bar.AddSubmenu("Edit")
	item(m, "Undo", "undo", "CmdOrCtrl+Z")
	item(m, "Redo", "redo", "CmdOrCtrl+Shift+Z")
	m.AddSeparator()
	item(m, "Cut", "cut", "CmdOrCtrl+X")
	item(m, "Copy", "copy", "CmdOrCtrl+C")
	item(m, "Paste", "paste", "CmdOrCtrl+V")
	item(m, "Select All", "select-all", "CmdOrCtrl+A")
	m.AddSeparator()
	item(m, "Find…", "search", "CmdOrCtrl+F")
	item(m, "Replace…", "replace", "CmdOrCtrl+OptionOrAlt+F")
	item(m, "Find in Project…", "project-search", "CmdOrCtrl+Shift+F")

	m = bar.AddSubmenu("View")
	item(m, "Command Palette…", "palette", "CmdOrCtrl+Shift+P")
	item(m, "Toggle Sidebar", "sidebar", "CmdOrCtrl+B")
	item(m, "Source Control", "git", "CmdOrCtrl+Shift+G")
	item(m, "Problems", "problems", "CmdOrCtrl+Shift+M")
	m.AddSeparator()
	item(m, "Increase Font Size", "zoom-in", "CmdOrCtrl+=")
	item(m, "Decrease Font Size", "zoom-out", "CmdOrCtrl+-")
	item(m, "Reset Font Size", "zoom-reset", "CmdOrCtrl+0")
	m.AddSeparator()
	m.Add("Toggle Full Screen").SetAccelerator("CmdOrCtrl+Ctrl+F").OnClick(window(func(w *application.WebviewWindow) { w.ToggleFullscreen() }))

	m = bar.AddSubmenu("Go")
	item(m, "Go to File…", "files", "CmdOrCtrl+P")
	item(m, "Go to Line…", "goto-line", "CmdOrCtrl+L")
	item(m, "Welcome", "welcome", "")

	bar.AddRole(application.WindowMenu)

	m = bar.AddSubmenu("Help")
	item(m, "Keyboard Shortcuts", "shortcuts", "")
	return bar
}

// openRequested runs on the editor's goroutine: folders become sessions,
// files open in the project they belong to, and the window comes forward.
// A request for a new window opens one, on the folder given or the start
// page.
func (b *Bridge) openRequested(req openRequest) {
	h := b.m.editor.hub
	if req.NewWindow {
		dir := ""
		for _, raw := range req.Paths {
			if p := resolveOpenPath(req.Cwd, raw); isDir(p) {
				dir = p
				break
			}
		}
		if dir != "" || len(req.Paths) == 0 {
			if err := b.newWindow(dir); err != nil {
				h.current.notify("Can't open a new window: " + err.Error())
			}
			return
		}
	}
	for _, raw := range req.Paths {
		p := resolveOpenPath(req.Cwd, raw)
		if isDir(p) {
			h.current.openProject(p)
			continue
		}
		h.show(h.sessionFor(p))
		h.current.open(p)
	}
	if b.window != nil {
		b.window.focus()
	}
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// serveLaunches starts listening for launches from other terminals. They go
// to the window in front.
func serveLaunches() {
	listenForRequests(func(req openRequest) {
		if b := frontBridge(); b != nil && b.m != nil {
			b.call(func() { b.openRequested(req) })
		} else if b != nil {
			// The window in front shows a remote editor: use a local one.
			for _, o := range windowsNow() {
				if o.m != nil {
					o.call(func() { o.openRequested(req) })
					return
				}
			}
		}
	}, openRemoteWindow)
}

// windowEditor starts the editor of a new window on dir, or on the start
// page. It sets the process up for that editor, as the first window's does.
func windowEditor(dir string) (*editor, error) {
	root, noProject := dir, dir == ""
	if noProject {
		root, _ = os.UserHomeDir()
		if root == "" {
			root = "/"
		}
	}
	if err := os.Chdir(root); err != nil {
		return nil, err
	}
	configRoot := root
	if noProject {
		configRoot = ""
	}
	cfg, problem := loadConfig(desktop.cfgPath, desktop.explicit, configRoot)
	if problem == nil {
		problem = checkTheme(cfg)
	}
	if problem != nil {
		if user, ok := usableConfig(desktop.cfgPath, desktop.explicit); ok {
			cfg = user
		} else {
			cfg = defaultConfig()
			cfg.path, cfg.explicit = desktop.cfgPath, desktop.explicit
		}
	}
	applyTheme(defaultTheme())
	_ = loadTheme(cfg)
	b, err := loadBuffer("")
	if err != nil {
		return nil, err
	}
	e, err := newEditor(cfg, b, root)
	if err != nil {
		return nil, err
	}
	if problem != nil {
		e.configProblem = problem
		e.notify("Config: " + problem.Error())
	}
	if noProject {
		e.enterNoProject()
	}
	return e, nil
}

// dockOpen runs a pick from the Dock menu: an empty path is a new window, a
// project open in a window brings that window forward with it, any other
// opens in a new window.
func dockOpen(path string) {
	list := windowsNow()
	for _, b := range list {
		if b.m == nil || path == "" {
			continue
		}
		b.call(func() {
			if open := b.m.editor.hub.find(path); open != nil {
				b.m.editor.hub.show(open)
				b.window.focus()
			}
		})
	}
	if b := frontBridge(); b != nil && b.m != nil {
		b.call(func() {
			for _, o := range windowsNow() {
				if path != "" && o.m != nil && o.m.editor.hub.find(path) != nil {
					return // shown above
				}
			}
			if err := b.newWindow(path); err != nil {
				b.m.editor.notify("Can't open a new window: " + err.Error())
			}
		})
	}
}

// dockProjects are the recent projects the Dock menu offers.
func dockProjects() []string {
	list := recentProjects()
	return list[:min(len(list), 10)]
}
