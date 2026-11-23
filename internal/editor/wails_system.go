//go:build wails

package editor

import (
	"os"
	"runtime"

	wmenu "github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

// The window's place in the system: the macOS menu bar, launches from other
// terminals, and closing with unsaved work.

// appMenu is the macOS menu bar. Its items run the same commands as the Gio
// window's; elsewhere the keymap has the Ctrl shortcuts and there is none.
func (b *Bridge) appMenu() *wmenu.Menu {
	if runtime.GOOS != "darwin" {
		return nil
	}
	cmd := func(key string) *keys.Accelerator { return keys.CmdOrCtrl(key) }
	shift := func(key string) *keys.Accelerator { return keys.Combo(key, keys.CmdOrCtrlKey, keys.ShiftKey) }
	option := func(key string) *keys.Accelerator { return keys.Combo(key, keys.CmdOrCtrlKey, keys.OptionOrAltKey) }
	item := func(m *wmenu.Menu, label string, id string, acc *keys.Accelerator) {
		m.AddText(label, acc, func(*wmenu.CallbackData) { b.Command(id) })
	}
	bar := wmenu.NewMenu()

	m := bar.AddSubmenu("Loom")
	item(m, "Settings…", "settings", cmd(","))
	item(m, "Color Theme…", "theme", nil)
	m.AddSeparator()
	m.AddText("Hide Loom", cmd("h"), func(*wmenu.CallbackData) { wrt.Hide(b.app) })
	m.AddSeparator()
	item(m, "Quit Loom", "quit", cmd("q"))

	m = bar.AddSubmenu("File")
	item(m, "New File", "new-file", cmd("n"))
	item(m, "Open File…", "open-file", cmd("o"))
	item(m, "Open Project…", "open-project", shift("o"))
	item(m, "Recent Projects…", "projects", nil)
	item(m, "Switch Session…", "sessions", cmd("k"))
	m.AddSeparator()
	item(m, "Save", "save", cmd("s"))
	item(m, "Save As…", "save-as", shift("s"))
	m.AddSeparator()
	item(m, "Close Tab", "close-tab", cmd("w"))
	item(m, "Close Session", "close-project", nil)

	m = bar.AddSubmenu("Edit")
	item(m, "Undo", "undo", cmd("z"))
	item(m, "Redo", "redo", shift("z"))
	m.AddSeparator()
	item(m, "Cut", "cut", cmd("x"))
	item(m, "Copy", "copy", cmd("c"))
	item(m, "Paste", "paste", cmd("v"))
	item(m, "Select All", "select-all", cmd("a"))
	m.AddSeparator()
	item(m, "Find…", "search", cmd("f"))
	item(m, "Replace…", "replace", option("f"))
	item(m, "Find in Project…", "project-search", shift("f"))

	m = bar.AddSubmenu("View")
	item(m, "Command Palette…", "palette", shift("p"))
	item(m, "Toggle Sidebar", "sidebar", cmd("b"))
	item(m, "Source Control", "git", shift("g"))
	item(m, "Problems", "problems", shift("m"))
	m.AddSeparator()
	item(m, "Increase Font Size", "zoom-in", cmd("="))
	item(m, "Decrease Font Size", "zoom-out", cmd("-"))
	item(m, "Reset Font Size", "zoom-reset", cmd("0"))
	m.AddSeparator()
	m.AddText("Toggle Full Screen", keys.Combo("f", keys.CmdOrCtrlKey, keys.ControlKey), func(*wmenu.CallbackData) {
		if wrt.WindowIsFullscreen(b.app) {
			wrt.WindowUnfullscreen(b.app)
		} else {
			wrt.WindowFullscreen(b.app)
		}
	})

	m = bar.AddSubmenu("Go")
	item(m, "Go to File…", "files", cmd("p"))
	item(m, "Go to Line…", "goto-line", cmd("l"))
	item(m, "Welcome", "welcome", nil)

	m = bar.AddSubmenu("Window")
	m.AddText("Minimize", cmd("m"), func(*wmenu.CallbackData) { wrt.WindowMinimise(b.app) })
	m.AddText("Zoom", nil, func(*wmenu.CallbackData) { wrt.WindowToggleMaximise(b.app) })

	m = bar.AddSubmenu("Help")
	item(m, "Keyboard Shortcuts", "shortcuts", nil)
	return bar
}

// openRequested runs on the editor's goroutine: folders become sessions,
// files open in the project they belong to, and the window comes forward.
func (b *Bridge) openRequested(req openRequest) {
	h := b.m.editor.hub
	for _, raw := range req.Paths {
		p := resolveOpenPath(req.Cwd, raw)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			h.current.openProject(p)
			continue
		}
		h.show(h.sessionFor(p))
		h.current.open(p)
	}
	if b.app != nil {
		wrt.WindowUnminimise(b.app)
		wrt.WindowShow(b.app)
		raiseWindow()
	}
}

// serveLaunches starts listening for launches from other terminals.
func (b *Bridge) serveLaunches() {
	if newWindow {
		return
	}
	listenForRequests(func(req openRequest) { b.call(func() { b.openRequested(req) }) })
}
