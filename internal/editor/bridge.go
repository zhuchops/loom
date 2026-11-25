package editor

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/png"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/zhuchops/loom/internal/brand"
)

// The bridge between the editor and a page that draws it. The desktop app's
// own window is one host; a Loom on another machine, reached over SSH, is
// another: there the editor runs remotely and sends its frames over a socket
// to the window on this computer.

// bridgeHost is where frames go and what the window around the page can do.
type bridgeHost interface {
	emit(frame any)
	quit()
	// pick shows the system's open or save panel; false means there is none.
	pick(kind pickKind, start string, chosen func(string)) bool
	appearance(dark bool)
}

// Bridge is what the web view calls. Every call is queued and runs on the
// editor's own goroutine, so the editor is never touched from two at once.
type Bridge struct {
	app  context.Context // the Wails runtime, once started
	host bridgeHost
	// peer is set in a window that shows a remote editor: the page's calls go
	// there instead of to an editor here.
	peer       *remotePeer
	remoteHost string // the machine a remote editor runs on, for the title bar
	m          *terminalModel
	inbox      chan tea.Msg
	calls      chan func()
	ticker     *time.Ticker
	// stopped closes when the editor's goroutine ends, as it quits. Nothing
	// runs queued calls after that.
	stopped chan struct{}

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
	// mdSent is the buffer version of the HTML the page has for each Markdown
	// preview; mdReset sends them all again.
	mdSent  map[string]int
	mdReset bool

	// Flow control: one frame is on its way to the page at a time. Input
	// keeps being applied while the page draws, and the next frame carries
	// the state as it is then, so a held key never builds a queue of frames
	// that keeps the cursor moving after the key is let go.
	inFlight bool
	sentAt   time.Time
	dirty    bool
	acks     int
	wake     *time.Timer
}

// frameTimeout is how long a frame may go without the page saying it drew
// it, as when the window is hidden, before the next one is sent anyway.
const frameTimeout = 200 * time.Millisecond

func newBridge(e *editor, ctx context.Context, explicit bool, host bridgeHost) *Bridge {
	b := &Bridge{inbox: make(chan tea.Msg, 256), calls: make(chan func(), 256), stopped: make(chan struct{}), ticker: time.NewTicker(time.Second), explicit: explicit, host: host}
	b.m = &terminalModel{editor: e, view: renderer{screen: newTerminalFrame(0, 0), native: true}, ctx: ctx, ticks: b.ticker.C}
	b.m.attach()
	e.settingsPage = b.toggleSettings
	e.pickFolder = func(start string, chosen func(string)) {
		if b.host == nil || !b.host.pick(pickFolder, start, chosen) {
			e.projectMenu()
		}
	}
	return b
}

// loop is the editor's goroutine: it applies input and results, then draws.
func (b *Bridge) loop() {
	for {
		b.acks = 0
		n := 1
		select {
		case f := <-b.calls:
			f()
		case msg := <-b.inbox:
			b.send(msg)
		}
		n += b.drain()
		if b.m.editor.quit {
			close(b.stopped)
			if b.host != nil {
				b.host.quit()
			}
			return
		}
		if n > b.acks {
			b.dirty = true
		}
		b.flush()
	}
}

// drain applies everything that is waiting and says how much there was.
func (b *Bridge) drain() int {
	n := 0
	for {
		select {
		case f := <-b.calls:
			f()
		case msg := <-b.inbox:
			b.send(msg)
		default:
			return n
		}
		n++
	}
}

// flush sends a frame when something changed and the page is ready for it.
func (b *Bridge) flush() {
	if !b.dirty {
		return
	}
	if b.inFlight {
		if wait := frameTimeout - time.Since(b.sentAt); wait > 0 {
			if b.wake == nil {
				b.wake = time.AfterFunc(wait, func() { b.calls <- func() { b.wake = nil } })
			}
			return
		}
	}
	b.dirty = false
	if b.render() {
		b.inFlight, b.sentAt = true, time.Now()
	}
}

// Drawn is the page saying it has drawn the last frame.
func (b *Bridge) Drawn() {
	if b.peer != nil {
		b.peer.forward("Drawn")
		return
	}
	b.call(func() {
		b.acks++
		b.inFlight = false
	})
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
	if b.peer != nil {
		b.peer.forward("Start", cols, rows)
		return
	}
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
func (b *Bridge) Resize(cols, rows int) {
	if b.peer != nil {
		b.peer.forward("Resize", cols, rows)
		return
	}
	b.call(func() { b.resize(cols, rows) })
}

func (b *Bridge) resize(cols, rows int) {
	cols, rows = max(4, cols), max(5, rows)
	if cols == b.cols && rows == b.rows {
		return
	}
	b.cols, b.rows = cols, rows
	b.send(tea.WindowSizeMsg{Width: cols, Height: rows})
}

// Redraw asks for a frame, as animations do.
func (b *Bridge) Redraw() {
	if b.peer != nil {
		b.peer.forward("Redraw")
		return
	}
	b.call(func() {})
}

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
