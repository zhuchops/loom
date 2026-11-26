//go:build desktop

package editor

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"image/png"
	"io/fs"
	"log/slog"
	"os"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/zhuchops/loom/internal/brand"
)

// The desktop front end, built on Wails, keeps the editor, its commands and its renderer in Go,
// as the terminal build does. The renderer lays out only the text panes on a
// character grid; each frame sends the rows that changed and the state of the
// chrome (tabs, sidebar, pickers, popovers) to the web view, which draws them
// with HTML and CSS.
//
// All of Loom's windows live in one process. Each has an editor of its own,
// with its sessions; one desk runs them all on one goroutine (desk.go).

//go:embed all:wailsui
var wailsAssets embed.FS

// desktop is the running app: its windows and the desk their editors run on.
var desktop struct {
	app  *application.App
	desk *desk
	// cfgPath and explicit are the config the first window was started with;
	// new windows use it too.
	cfgPath  string
	explicit bool

	mu      sync.Mutex
	windows map[uint]*Bridge // by Wails window ID
	order   []uint           // in the order they opened
	// quitting is set while Quit asks the windows about unsaved work, and
	// quitOK once they all answered.
	quitting, quitOK bool
}

// MainDesktop runs Loom in a web view window and exits the process on failure.
func MainDesktop() {
	// Started from Finder, the working directory is "/"; begin in the home folder.
	if wd, err := os.Getwd(); err == nil && wd == "/" {
		if home, err := os.UserHomeDir(); err == nil {
			_ = os.Chdir(home)
		}
	}
	platformEnvironment()
	quietTyping()
	if handled, err := remoteEntry(true); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, "loom:", err)
			os.Exit(1)
		}
		return
	}
	e, explicit, done, err := prepare(true)
	if err == nil && !done {
		err = runDesktop(e, explicit)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "loom:", err)
		os.Exit(1)
	}
}

func runDesktop(e *editor, explicit bool) error {
	assets, err := fs.Sub(wailsAssets, "wailsui")
	if err != nil {
		return err
	}
	desktop.windows = map[uint]*Bridge{}
	desktop.desk = newDesk()
	desktop.desk.closed = windowClosed
	desktop.cfgPath, desktop.explicit = e.cfg.path, explicit
	app := application.New(application.Options{
		Name:        "Loom",
		Description: "A fast, modal code editor",
		Services:    []application.Service{application.NewService(&Page{})},
		Assets:      application.AssetOptions{Handler: application.BundledAssetFileServer(assets), DisableLogging: true},
		LogLevel:    slog.LevelError,
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		Linux:       application.LinuxOptions{ProgramName: "loom"},
		ShouldQuit:  shouldQuit,
	})
	desktop.app = app
	if menu := appMenu(); menu != nil {
		app.Menu.Set(menu)
	}
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		installDock()
		serveLaunches()
	})
	first := desktop.desk.bridge(e, context.Background(), explicit, nil)
	openWindow(first, newWindow)
	go desktop.desk.loop()
	return app.Run()
}

// Page is what the web view calls. Each call names the window it came from,
// and goes to that window's bridge.
type Page struct{}

func (Page) of(ctx context.Context) *Bridge {
	w, _ := ctx.Value(application.WindowKey).(application.Window)
	if w == nil {
		return nil
	}
	desktop.mu.Lock()
	defer desktop.mu.Unlock()
	return desktop.windows[w.ID()]
}

func (p *Page) Start(ctx context.Context, cols, rows int) {
	if b := p.of(ctx); b != nil {
		b.Start(cols, rows)
	}
}

func (p *Page) Resize(ctx context.Context, cols, rows int) {
	if b := p.of(ctx); b != nil {
		b.Resize(cols, rows)
	}
}

func (p *Page) Drawn(ctx context.Context) {
	if b := p.of(ctx); b != nil {
		b.Drawn()
	}
}

func (p *Page) Redraw(ctx context.Context) {
	if b := p.of(ctx); b != nil {
		b.Redraw()
	}
}

func (p *Page) Key(ctx context.Context, ev KeyEvent) {
	if b := p.of(ctx); b != nil {
		b.Key(ev)
	}
}

func (p *Page) Text(ctx context.Context, s string) {
	if b := p.of(ctx); b != nil {
		b.Text(s)
	}
}

func (p *Page) Mouse(ctx context.Context, kind string, x, y, button int, held, shift bool) {
	if b := p.of(ctx); b != nil {
		b.Mouse(kind, x, y, button, held, shift)
	}
}

func (p *Page) Wheel(ctx context.Context, x, y, dx, dy int) {
	if b := p.of(ctx); b != nil {
		b.Wheel(x, y, dx, dy)
	}
}

func (p *Page) Act(ctx context.Context, name string, i int, s string) {
	if b := p.of(ctx); b != nil {
		b.Act(name, i, s)
	}
}

func (p *Page) Command(ctx context.Context, id string) {
	if b := p.of(ctx); b != nil {
		b.Command(id)
	}
}

func (p *Page) Answer(ctx context.Context, i int) {
	if b := p.of(ctx); b != nil {
		b.Answer(i)
	}
}

func (p *Page) Logo(size int) string { return (*Bridge)(nil).Logo(size) }

func (p *Page) Log(msg string) { (*Bridge)(nil).Log(msg) }

// openWindow gives b a window. A window opened beside others starts a little
// below and right of the one in front.
func openWindow(b *Bridge, cascade bool) {
	title := "Loom"
	if b.remoteHost != "" && b.remoteRoot != "" {
		title = b.remoteHost + ": " + b.remoteRoot
	}
	w := desktop.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            title,
		Width:            1280,
		Height:           820,
		MinWidth:         640,
		MinHeight:        420,
		URL:              "/",
		BackgroundType:   windowBackgroundType(),
		BackgroundColour: windowBackground(),
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			Backdrop:                application.MacBackdropTranslucent,
			InvisibleTitleBarHeight: 0,
			CollectionBehavior:      application.MacWindowCollectionBehaviorFullScreenPrimary,
		},
		Linux:                      application.LinuxWindow{Icon: linuxIcon()},
		DefaultContextMenuDisabled: true,
	})
	h := &wailsHost{b: b, w: w}
	if b.host == nil {
		b.host = h
	}
	b.window = h
	desktop.mu.Lock()
	desktop.windows[w.ID()] = b
	desktop.order = append(desktop.order, w.ID())
	desktop.mu.Unlock()
	if b.m != nil {
		b.m.editor.hub.window = func(dir string) error { return b.newWindow(dir) }
		b.quitApp = quitApp
	}
	w.RegisterHook(events.Common.WindowClosing, func(ev *application.WindowEvent) {
		if h.closing.Load() {
			return
		}
		ev.Cancel()
		if b.peer != nil {
			// The editor on the other machine decides: it may ask first.
			b.peer.forward("Close")
			return
		}
		b.call(b.requestQuit)
	})
	w.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		installWindow(w.NativeWindow(), cascade)
	})
	w.OnWindowEvent(events.Common.WindowFocus, func(*application.WindowEvent) {
		if b.m != nil {
			b.call(b.focused)
		}
	})
}

// newWindow opens another window on dir, or on the start page. It runs on
// the desk's goroutine, in the window it was asked from.
func (b *Bridge) newWindow(dir string) error {
	var problem error
	next := b.desk.aside(func() *Bridge {
		e, err := windowEditor(dir)
		if err != nil {
			problem = err
			return nil
		}
		return b.desk.bridge(e, context.Background(), false, nil)
	})
	if next == nil {
		return problem
	}
	openWindow(next, true)
	return nil
}

// focused catches a window up on what other windows changed meanwhile, as
// a session does when it is shown.
func (b *Bridge) focused() {
	e := b.m.editor
	if e.syncConfig() {
		if err := loadTheme(e.cfg); err == nil {
			italicComments = e.cfg.ItalicComments
			e.refreshHighlights()
		}
	}
	e.checkDisk()
}

// windowClosed follows a window whose editor quit, on the desk's goroutine.
func windowClosed(b *Bridge) {
	if b.m != nil {
		b.m.editor.hub.finish()
	}
	if desktop.quitting {
		nextQuit()
	}
}

// forget drops a closed window; with none left, Loom quits.
func forget(id uint) {
	desktop.mu.Lock()
	delete(desktop.windows, id)
	desktop.order = slices.DeleteFunc(desktop.order, func(o uint) bool { return o == id })
	empty := len(desktop.windows) == 0
	desktop.mu.Unlock()
	if empty {
		desktop.quitOK = true
		go desktop.app.Quit()
	}
}

// windowsNow lists the open windows' bridges, in the order they opened.
func windowsNow() []*Bridge {
	desktop.mu.Lock()
	defer desktop.mu.Unlock()
	var out []*Bridge
	for _, id := range desktop.order {
		out = append(out, desktop.windows[id])
	}
	return out
}

// frontBridge is the window in front, or the first one.
func frontBridge() *Bridge {
	if w := desktop.app.Window.Current(); w != nil {
		desktop.mu.Lock()
		b := desktop.windows[w.ID()]
		desktop.mu.Unlock()
		if b != nil {
			return b
		}
	}
	if list := windowsNow(); len(list) > 0 {
		return list[0]
	}
	return nil
}

// shouldQuit answers the system's request to quit (⌘Q, the Dock, logging
// out): not yet, while the windows are asked about unsaved work.
func shouldQuit() bool {
	if desktop.quitOK {
		return true
	}
	quitApp()
	return false
}

// quitApp closes every window, asking in each one with unsaved work first.
func quitApp() {
	if b := frontBridge(); b != nil && b.desk != nil {
		b.call(func() {
			desktop.quitting = true
			nextQuit()
		})
	}
}

// nextQuit, on the desk's goroutine, closes the windows one after another:
// those with nothing unsaved at once, the others after asking. Cancel stops.
func nextQuit() {
	for _, b := range windowsNow() {
		if b.peer != nil {
			b.peer.forward("Close")
			continue
		}
		if b.gone || b.m.editor.quit {
			continue
		}
		if !b.unsaved() {
			b.m.editor.hub.quitAll(false)
			b.call(func() {}) // the desk finishes it
			continue
		}
		b.desk.enter(b)
		if b.window != nil {
			b.window.focus()
		}
		b.askQuit(func() { desktop.quitting = false })
		return
	}
}

// wailsHost is a window of the desktop app.
type wailsHost struct {
	b       *Bridge
	w       *application.WebviewWindow
	closing atomic.Bool
}

func (h *wailsHost) emit(frame any) {
	h.w.DispatchWailsEvent(&application.CustomEvent{Name: "frame", Data: frame})
}

// quit closes the window: its editor has quit.
func (h *wailsHost) quit() {
	if h.closing.Swap(true) {
		return
	}
	id := h.w.ID()
	go func() {
		h.w.Close()
		forget(id)
	}()
}

func (h *wailsHost) execJS(code string) { h.w.ExecJS(code) }

// focus brings the window to the front.
func (h *wailsHost) focus() {
	h.w.UnMinimise()
	h.w.Show()
	h.w.Focus()
	raiseApp()
}

func (h *wailsHost) openURL(url string) { _ = desktop.app.Browser.OpenURL(url) }

func (h *wailsHost) appearance(dark bool) { setAppearance(h.w.NativeWindow(), dark) }

// pick shows the system's open or save panel off the editor's goroutine and
// hands the chosen path back to it.
func (h *wailsHost) pick(kind pickKind, start string, chosen func(string)) bool {
	b := h.b
	go func() {
		var path string
		var err error
		dialogs := desktop.app.Dialog
		switch kind {
		case pickFolder:
			path, err = dialogs.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(true).
				SetTitle("Open Project").SetDirectory(start).AttachToWindow(h.w).PromptForSingleSelection()
		case pickFile:
			path, err = dialogs.OpenFile().CanChooseFiles(true).ShowHiddenFiles(true).
				SetTitle("Open File").SetDirectory(start).AttachToWindow(h.w).PromptForSingleSelection()
		case pickSave:
			path, err = dialogs.SaveFile().CanCreateDirectories(true).ShowHiddenFiles(true).
				SetDirectory(start).AttachToWindow(h.w).PromptForSingleSelection()
		}
		if err != nil || path == "" {
			return
		}
		b.call(func() { chosen(path) })
	}()
	return true
}

// linuxIcon is Loom's icon for the window on Linux.
func linuxIcon() []byte {
	if runtime.GOOS != "linux" {
		return nil
	}
	var icon bytes.Buffer
	_ = png.Encode(&icon, brand.Icon(256))
	return icon.Bytes()
}

// windowBackground is clear on macOS, where the window blurs the desktop
// behind the chrome. Elsewhere the window is opaque: a see-through GTK window
// repaints its translucent parts over what was there, so colors drift and
// flash wherever the pointer changes something.
func windowBackground() application.RGBA {
	if runtime.GOOS == "darwin" {
		return application.RGBA{}
	}
	r, g, b := th.bg.RGB()
	return application.RGBA{Red: uint8(r), Green: uint8(g), Blue: uint8(b), Alpha: 255}
}

func windowBackgroundType() application.BackgroundType {
	if runtime.GOOS == "darwin" {
		return application.BackgroundTypeTranslucent
	}
	return application.BackgroundTypeSolid
}
