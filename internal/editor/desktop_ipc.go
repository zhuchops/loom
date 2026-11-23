//go:build desktop

package editor

import (
	"os"

	"gioui.org/io/system"
)

// openRequested runs on the event loop: folders become sessions, files open
// in the project they belong to, and the window comes forward.
func (d *desktop) openRequested(req openRequest) {
	h := d.m.editor.hub
	for _, raw := range req.Paths {
		p := resolveOpenPath(req.Cwd, raw)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			h.current.openProject(p)
			continue
		}
		h.show(h.sessionFor(p))
		h.current.open(p)
	}
	d.w.Perform(system.ActionRaise)
}

// serveLaunches starts listening for launches from other terminals.
func (d *desktop) serveLaunches() {
	listenForRequests(func(req openRequest) {
		d.post(uiCallback(func() { d.openRequested(req) }))
	})
}
