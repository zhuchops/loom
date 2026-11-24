//go:build desktop

package editor

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// One desktop Loom owns a socket in the state folder. Starting loom-desktop
// again with a file or folder hands it to that window instead of opening a
// second one; -new opts out and starts a window of its own.

// newWindow is set by -new: the process neither forwards nor takes the socket.
var newWindow bool

// forwardToRunning gives paths to a Loom that is already open. It reports
// whether one took them; with no paths it just brings that window forward.
func forwardToRunning(paths []string) bool {
	sock := ipcSocket()
	if sock == "" {
		return false
	}
	conn, err := net.DialTimeout("unix", sock, 500*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()
	wd, _ := os.Getwd()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if json.NewEncoder(conn).Encode(openRequest{Cwd: wd, Paths: paths}) != nil {
		return false
	}
	reply, err := bufio.NewReader(conn).ReadString('\n')
	return err == nil && strings.TrimSpace(reply) == "ok"
}

// listenForRequests makes this process the one that later launches talk to.
// It does nothing if another live Loom already owns the socket.
// A remote editor's request keeps its connection, which remote takes over.
func listenForRequests(handle func(openRequest), remote func(net.Conn, remoteHello)) {
	sock := ipcSocket()
	if sock == "" || newWindow {
		return
	}
	if conn, err := net.DialTimeout("unix", sock, 300*time.Millisecond); err == nil {
		conn.Close()
		return
	}
	if os.MkdirAll(filepath.Dir(sock), 0o700) != nil {
		return
	}
	_ = os.Remove(sock) // left behind by a Loom that did not exit cleanly
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return
	}
	_ = os.Chmod(sock, 0o600)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				var req openRequest
				// The other end waits for "ok" before it sends anything else,
				// so the decoder's buffer holds nothing more.
				if json.NewDecoder(conn).Decode(&req) != nil {
					conn.Close()
					return
				}
				if req.Remote != nil && remote != nil {
					_ = conn.SetDeadline(time.Time{})
					if _, err := conn.Write([]byte("ok\n")); err != nil {
						conn.Close()
						return
					}
					remote(conn, *req.Remote)
					return
				}
				defer conn.Close()
				handle(req)
				_, _ = conn.Write([]byte("ok\n"))
			}()
		}
	}()
}

// resolveOpenPath makes a path from a launch absolute, relative to where it was typed.
func resolveOpenPath(cwd, p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(cwd, p)
	}
	p = filepath.Clean(p)
	if real, err := filepath.EvalSymlinks(p); err == nil {
		p = real
	}
	return p
}

// sessionFor picks the open project a file belongs to: the deepest root
// that holds it, or the shown session when none does.
func (h *sessionHub) sessionFor(path string) *editor {
	var best *editor
	for _, e := range h.live {
		if e.noProject || !strings.HasPrefix(path, e.root+string(filepath.Separator)) {
			continue
		}
		if best == nil || len(e.root) > len(best.root) {
			best = e
		}
	}
	if best == nil {
		return h.current
	}
	return best
}
