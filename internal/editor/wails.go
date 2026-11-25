//go:build desktop

package editor

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"image/png"
	"io/fs"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v2"
	wopt "github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wrt "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zhuchops/loom/internal/brand"
)

// The desktop front end, built on Wails, keeps the editor, its commands and its renderer in Go,
// as the terminal build does. The renderer lays out only the text panes on a
// character grid; each frame sends the rows that changed and the state of the
// chrome (tabs, sidebar, pickers, popovers) to the web view, which draws them
// with HTML and CSS.

//go:embed all:wailsui
var wailsAssets embed.FS

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
	if len(os.Args) > 1 && os.Args[1] == remoteSessionFlag {
		if err := runRemoteWindow(); err != nil {
			fmt.Fprintln(os.Stderr, "loom:", err)
			os.Exit(1)
		}
		return
	}
	if handled, err := remoteEntry(true); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, "loom:", err)
			os.Exit(1)
		}
		return
	}
	e, explicit, done, err := prepare(true)
	if err == nil && !done {
		err = runWails(e, explicit)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "loom:", err)
		os.Exit(1)
	}
}

func runWails(e *editor, explicit bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := newBridge(e, ctx, explicit, nil)
	b.host = wailsHost{b}
	defer b.ticker.Stop()
	assets, err := fs.Sub(wailsAssets, "wailsui")
	if err != nil {
		return err
	}
	return wails.Run(&wopt.App{
		Title:            "Loom",
		Width:            1280,
		Height:           820,
		MinWidth:         640,
		MinHeight:        420,
		BackgroundColour: windowBackground(),
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        b.startup,
		OnDomReady:       func(context.Context) { installWindow() },
		OnBeforeClose:    b.beforeClose,
		OnShutdown:       func(context.Context) { e.hub.finish() },
		Bind:             []any{b},
		Menu:             b.appMenu(),
		Linux:            linuxOptions(),
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
	})
}

// linuxOptions gives the window Loom's icon and the program name the desktop
// entry that install.sh writes looks for (StartupWMClass=loom).
func linuxOptions() *linux.Options {
	if runtime.GOOS != "linux" {
		return nil
	}
	var icon bytes.Buffer
	_ = png.Encode(&icon, brand.Icon(256))
	return &linux.Options{Icon: icon.Bytes(), ProgramName: "loom"}
}

// windowBackground is clear on macOS, where the window blurs the desktop
// behind the chrome. Elsewhere the window is opaque: a see-through GTK window
// repaints its translucent parts over what was there, so colors drift and
// flash wherever the pointer changes something.
func windowBackground() *wopt.RGBA {
	if runtime.GOOS == "darwin" {
		return &wopt.RGBA{}
	}
	r, g, b := th.bg.RGB()
	return &wopt.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

func (b *Bridge) startup(ctx context.Context) {
	b.app = ctx
	b.serveLaunches()
	go b.loop()
}

// beforeClose keeps the window open while files are unsaved, and asks what
// to do with them.
//
// Quitting from the editor (Save All or Don't Save in that question, ⌘Q)
// ends the editor's goroutine and then asks Wails to quit, which calls this
// again on another goroutine. By then nothing runs queued calls, and the
// editor has already settled what to do, so the window just closes.
func (b *Bridge) beforeClose(context.Context) bool {
	reply := make(chan bool, 1)
	ask := func() {
		if b.m.editor.quit || !b.unsaved() {
			reply <- false
			return
		}
		b.requestQuit()
		reply <- true
	}
	select {
	case b.calls <- ask:
	case <-b.stopped:
		return false
	}
	select {
	case keep := <-reply:
		return keep
	case <-b.stopped:
		return false
	}
}

// wailsHost is the desktop app's own window.
type wailsHost struct{ b *Bridge }

func (h wailsHost) emit(frame any) { wrt.EventsEmit(h.b.app, "frame", frame) }

func (h wailsHost) quit() { wrt.Quit(h.b.app) }

func (h wailsHost) execJS(code string) { wrt.WindowExecJS(h.b.app, code) }

func (h wailsHost) appearance(dark bool) { setAppearance(dark) }

// pick shows the system's open or save panel off the editor's goroutine and
// hands the chosen path back to it.
func (h wailsHost) pick(kind pickKind, start string, chosen func(string)) bool {
	b := h.b
	go func() {
		var path string
		var err error
		switch kind {
		case pickFolder:
			path, err = wrt.OpenDirectoryDialog(b.app, wrt.OpenDialogOptions{DefaultDirectory: start, Title: "Open Project", CanCreateDirectories: true})
		case pickFile:
			path, err = wrt.OpenFileDialog(b.app, wrt.OpenDialogOptions{DefaultDirectory: start, Title: "Open File", ShowHiddenFiles: true})
		case pickSave:
			path, err = wrt.SaveFileDialog(b.app, wrt.SaveDialogOptions{DefaultDirectory: start, Title: "Save As", CanCreateDirectories: true})
		}
		if err != nil || path == "" {
			return
		}
		b.call(func() { chosen(path) })
	}()
	return true
}
