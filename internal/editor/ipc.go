package editor

import (
	"os"
	"path/filepath"
	"strconv"
)

// One desktop Loom owns a socket in the state folder; later launches, and
// Loom on machines reached with loom ssh, talk to it there.

// openRequest is what a second launch sends to the first.
type openRequest struct {
	Cwd   string   `json:"cwd"`
	Paths []string `json:"paths"`
	// Remote is set by a Loom on another machine that wants a window here
	// for its editor; the connection then carries the session.
	Remote *remoteHello `json:"remote,omitempty"`
}

// remoteHello names a remote editor's machine and project.
type remoteHello struct {
	Host string `json:"host"`
	Root string `json:"root"`
}

// ipcSocket is where the desktop app listens.
func ipcSocket() string {
	root := stateRoot()
	if root == "" {
		return ""
	}
	sock := filepath.Join(root, "desktop.sock")
	if len(sock) > 100 {
		// A socket path is limited to about 104 bytes.
		sock = filepath.Join(os.TempDir(), "loom-"+strconv.Itoa(os.Getuid())+".sock")
	}
	return sock
}
