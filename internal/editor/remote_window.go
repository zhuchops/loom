//go:build desktop

package editor

import (
	"encoding/json"
	"net"
	"path/filepath"
)

// A window here for an editor on another machine. Its page talks to the
// remote editor through the connection: the bridge forwards the page's calls
// there, and the frames that come back go to the page.

// openRemoteWindow opens the window for a remote editor.
func openRemoteWindow(conn net.Conn, hello remoteHello) {
	b := &Bridge{peer: newRemotePeer(conn), remoteHost: hello.Host, remoteRoot: filepath.Base(hello.Root)}
	openWindow(b, true)
	h, ok := b.window.(*wailsHost)
	if !ok {
		conn.Close()
		return
	}
	go func() {
		defer conn.Close()
		dec := json.NewDecoder(conn)
		for {
			var m struct {
				Frame      json.RawMessage `json:"frame"`
				Clipboard  string          `json:"clipboard"`
				Appearance *bool           `json:"appearance"`
				Quit       bool            `json:"quit"`
			}
			if err := dec.Decode(&m); err != nil {
				break
			}
			switch {
			case m.Quit:
				h.quit()
				return
			case len(m.Frame) > 0:
				h.emit(m.Frame)
			case m.Appearance != nil:
				h.appearance(*m.Appearance)
			case m.Clipboard != "":
				systemCopy(m.Clipboard)
			}
		}
		// The server or the SSH connection went away.
		h.quit()
	}()
}
