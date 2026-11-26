package editor

import (
	"os"
	"slices"
)

// A desk runs the editors of every window in the process on one goroutine.
// Each window has an editor of its own, with its sessions, but some of what
// an editor sets is the process's: the theme, the working directory, the tab
// width. The desk swaps those as it turns from one window's work to
// another's, so each editor finds the process as it left it.
type desk struct {
	calls  chan deskCall
	open   []*Bridge // the windows, in the order they opened
	active *Bridge   // whose settings the process has now
	// closed, when set, follows a window whose editor has quit; it runs on
	// the desk's goroutine.
	closed func(*Bridge)
}

type deskCall struct {
	b *Bridge
	f func()
}

func newDesk() *desk { return &desk{calls: make(chan deskCall, 1024)} }

// loop applies the windows' calls and draws the windows they changed, until
// none is left.
func (d *desk) loop() {
	for len(d.open) > 0 {
		touched := map[*Bridge]bool{}
		d.apply(<-d.calls, touched)
	drain:
		for {
			select {
			case c := <-d.calls:
				d.apply(c, touched)
			default:
				break drain
			}
		}
		for _, b := range slices.Clone(d.open) {
			if !touched[b] {
				continue
			}
			d.enter(b)
			if b.m.editor.quit {
				d.finish(b)
				continue
			}
			if b.events > b.acks {
				b.dirty = true
			}
			b.events, b.acks = 0, 0
			b.flush()
		}
	}
}

func (d *desk) apply(c deskCall, touched map[*Bridge]bool) {
	if c.b.gone {
		return
	}
	d.enter(c.b)
	c.b.events++
	c.f()
	touched[c.b] = true
}

// finish takes a window whose editor quit off the desk.
func (d *desk) finish(b *Bridge) {
	b.gone = true
	close(b.stopped)
	b.ticker.Stop()
	d.open = slices.DeleteFunc(d.open, func(o *Bridge) bool { return o == b })
	if d.active == b {
		d.active = nil
	}
	if d.closed != nil {
		d.closed(b)
	}
	if b.host != nil {
		b.host.quit()
	}
}

// enter gives the process to b's editor.
func (d *desk) enter(b *Bridge) {
	if d.active == b {
		return
	}
	if d.active != nil {
		d.active.process = captureProcess()
	}
	d.active = b
	if b.process != nil {
		b.process.restore()
	}
}

// aside runs start, which starts another window's editor and so sets the
// process up for it, and gives the process back to the window it ran in.
// start returns the new window's bridge, or nil.
func (d *desk) aside(start func() *Bridge) *Bridge {
	from := d.active
	if from != nil {
		from.process = captureProcess()
	}
	d.active = nil
	b := start()
	if b != nil {
		b.process = captureProcess()
		d.active = b
	}
	if from != nil {
		d.enter(from)
	}
	return b
}

// processState is what an editor sets for the whole process.
type processState struct {
	th                                                palette
	base, muted, heading, menu, active, panel, border cellStyle
	sel, line                                         terminalColor
	italic                                            bool
	tabWidth                                          int
	dir                                               string
}

func captureProcess() *processState {
	dir, _ := os.Getwd()
	return &processState{
		th: th, base: baseStyle, muted: mutedStyle, heading: headingStyle, menu: menuStyle, active: activeStyle,
		panel: panelStyle, border: borderStyle, sel: selectStyle, line: lineStyle,
		italic: italicComments, tabWidth: tabWidth, dir: dir,
	}
}

func (s *processState) restore() {
	th, baseStyle, mutedStyle, headingStyle, menuStyle, activeStyle = s.th, s.base, s.muted, s.heading, s.menu, s.active
	panelStyle, borderStyle, selectStyle, lineStyle = s.panel, s.border, s.sel, s.line
	italicComments, tabWidth = s.italic, s.tabWidth
	if dir, _ := os.Getwd(); s.dir != "" && dir != s.dir {
		_ = os.Chdir(s.dir)
	}
}
