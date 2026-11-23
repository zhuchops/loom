//go:build wails

package editor

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"image/png"
	"io/fs"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/wailsapp/wails/v2"
	wopt "github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wrt "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/zhuchops/loom/internal/brand"
)

// The Wails front end keeps the editor, its commands and its renderer in Go,
// as the terminal build does. The renderer lays out only the text panes on a
// character grid; each frame sends the rows that changed and the state of the
// chrome (tabs, sidebar, pickers, popovers) to the web view, which draws them
// with HTML and CSS.

//go:embed all:wailsui
var wailsAssets embed.FS

// MainWails runs Loom in a web view window and exits the process on failure.
func MainWails() {
	// Started from Finder, the working directory is "/"; begin in the home folder.
	if wd, err := os.Getwd(); err == nil && wd == "/" {
		if home, err := os.UserHomeDir(); err == nil {
			_ = os.Chdir(home)
		}
	}
	platformEnvironment()
	e, explicit, done, err := prepare(true)
	if err == nil && !done {
		err = runWails(e, explicit)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "loom:", err)
		os.Exit(1)
	}
}

// Bridge is what the web view calls. Every call is queued and runs on the
// editor's own goroutine, so the editor is never touched from two at once.
type Bridge struct {
	app    context.Context // the Wails runtime, once started
	m      *terminalModel
	inbox  chan tea.Msg
	calls  chan func()
	ticker *time.Ticker

	explicit   bool
	started    bool
	cols, rows int
	frame      frameState
	dialog     *dialog
	list       listState
	rows2      panelRows
	settings   *settingsView
	fonts      []webFont // installed families, as the page found them
	preview    previewCache
}

func runWails(e *editor, explicit bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := newBridge(e, ctx, explicit)
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
		BackgroundColour: &wopt.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        b.startup,
		OnBeforeClose:    b.beforeClose,
		OnShutdown:       func(context.Context) { e.hub.finish() },
		Bind:             []any{b},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
	})
}

func newBridge(e *editor, ctx context.Context, explicit bool) *Bridge {
	b := &Bridge{inbox: make(chan tea.Msg, 256), calls: make(chan func(), 256), ticker: time.NewTicker(time.Second), explicit: explicit}
	b.m = &terminalModel{editor: e, view: renderer{screen: newTerminalFrame(0, 0), native: true}, ctx: ctx, ticks: b.ticker.C}
	b.m.attach()
	e.settingsPage = b.toggleSettings
	e.pickFolder = func(start string, chosen func(string)) { b.pick(pickFolder, start, chosen) }
	return b
}

func (b *Bridge) startup(ctx context.Context) {
	b.app = ctx
	go b.loop()
}

// beforeClose keeps the window open while files are unsaved.
func (b *Bridge) beforeClose(context.Context) bool {
	reply := make(chan bool)
	b.calls <- func() {
		e := b.m.editor
		if e.quit || len(e.hub.unsaved()) == 0 {
			reply <- false
			return
		}
		e.notify("Save or discard your changes before quitting")
		reply <- true
	}
	return <-reply
}

// loop is the editor's goroutine: it applies input and results, then draws.
func (b *Bridge) loop() {
	for {
		select {
		case f := <-b.calls:
			f()
		case msg := <-b.inbox:
			b.send(msg)
		}
		b.drain()
		if b.m.editor.quit {
			wrt.Quit(b.app)
			return
		}
		b.render()
	}
}

func (b *Bridge) drain() {
	for {
		select {
		case f := <-b.calls:
			f()
		case msg := <-b.inbox:
			b.send(msg)
		default:
			return
		}
	}
}

// post hands msg to the editor's goroutine from any other.
func (b *Bridge) post(msg tea.Msg) { b.inbox <- msg }

// run executes a Bubble Tea command in the background and queues its result.
func (b *Bridge) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		if msg := cmd(); msg != nil {
			b.post(msg)
		}
	}()
}

func (b *Bridge) send(msg tea.Msg) {
	switch msg := msg.(type) {
	case uiCallback:
		msg()
		return
	case tea.BatchMsg:
		for _, cmd := range msg {
			b.run(cmd)
		}
		return
	case tea.QuitMsg:
		b.m.editor.quit = true
		return
	case tea.KeyPressMsg:
		if b.settingsHaveKeys() {
			b.settingsKey(terminalKey(tea.Key(msg)))
			return
		}
	case tea.PasteMsg:
		if b.settingsHaveKeys() {
			b.settingsKey(keyInput{paste: msg.Content})
			return
		}
	}
	_, cmd := b.m.Update(msg)
	b.run(cmd)
	b.run(b.m.settle()) // a click can switch sessions without going through Update
}

// call queues f on the editor's goroutine.
func (b *Bridge) call(f func()) { b.calls <- f }

// Start is the web view saying it is ready, with the size of its text grid.
func (b *Bridge) Start(cols, rows int) {
	b.call(func() {
		if !b.started {
			b.started = true
			b.m.editor.begin(b.explicit)
			b.run(b.m.Init())
			b.playDemo()
		}
		b.resize(cols, rows)
		b.frame = frameState{} // the page is new: send everything
	})
}

// Resize gives the text grid a new size in cells.
func (b *Bridge) Resize(cols, rows int) { b.call(func() { b.resize(cols, rows) }) }

func (b *Bridge) resize(cols, rows int) {
	cols, rows = max(4, cols), max(5, rows)
	if cols == b.cols && rows == b.rows {
		return
	}
	b.cols, b.rows = cols, rows
	b.send(tea.WindowSizeMsg{Width: cols, Height: rows})
}

// Redraw asks for a frame, as animations do.
func (b *Bridge) Redraw() { b.call(func() {}) }

// uiCallback runs on the editor's goroutine.
type uiCallback func()

// Logo is the Loom mark as a PNG data URL, for the Welcome tab.
func (b *Bridge) Logo(size int) string {
	var out bytes.Buffer
	if err := png.Encode(&out, brand.Mark(max(16, min(512, size)))); err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
}

// Log writes a message from the page to standard error, which the detached
// app sends to its log file.
func (b *Bridge) Log(msg string) { fmt.Fprintln(os.Stderr, "loom: page:", msg) }
