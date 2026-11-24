//go:build desktop

package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	wopt "github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

// A window here for an editor on another machine. The desktop app that owns
// the socket starts one process per remote session (Wails has one window
// per process) and hands it the connection as file descriptor 3.

const remoteSessionFlag = "-remote-session"

// openRemoteWindow starts the window process for a remote editor.
func openRemoteWindow(conn net.Conn, hello remoteHello) {
	defer conn.Close()
	unix, ok := conn.(*net.UnixConn)
	if !ok {
		return
	}
	file, err := unix.File()
	if err != nil {
		return
	}
	defer file.Close()
	exe, err := os.Executable()
	if err != nil {
		return
	}
	title := hello.Host + ": " + filepath.Base(hello.Root)
	cmd := exec.Command(exe, remoteSessionFlag)
	cmd.ExtraFiles = []*os.File{file}
	cmd.Env = append(os.Environ(), detachedMark+"=1", "LOOM_REMOTE_TITLE="+title)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "loom: remote window:", err)
		return
	}
	go func() { _ = cmd.Wait() }()
}

// runRemoteWindow is the window process: its page talks to the remote editor
// through the connection on descriptor 3.
func runRemoteWindow() error {
	conn, err := net.FileConn(os.NewFile(3, "remote"))
	if err != nil {
		return fmt.Errorf("remote window: %w", err)
	}
	defer conn.Close()
	newWindow = true // never the one later launches talk to
	b := &Bridge{peer: newRemotePeer(conn)}
	quitting := false
	assets, err := fs.Sub(wailsAssets, "wailsui")
	if err != nil {
		return err
	}
	title := os.Getenv("LOOM_REMOTE_TITLE")
	if title == "" {
		title = "Loom"
	}
	return wails.Run(&wopt.App{
		Title:            title,
		Width:            1280,
		Height:           820,
		MinWidth:         640,
		MinHeight:        420,
		BackgroundColour: &wopt.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup: func(ctx context.Context) {
			b.app = ctx
			go func() {
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
						quitting = true
						wrt.Quit(ctx)
						return
					case len(m.Frame) > 0:
						wrt.EventsEmit(ctx, "frame", m.Frame)
					case m.Appearance != nil:
						setAppearance(*m.Appearance)
					case m.Clipboard != "":
						systemCopy(m.Clipboard)
					}
				}
				// The server or the SSH connection went away.
				quitting = true
				wrt.Quit(ctx)
			}()
		},
		OnDomReady: func(context.Context) { installWindow() },
		OnBeforeClose: func(context.Context) bool {
			if quitting {
				return false
			}
			// The editor decides: it may ask about unsaved files first.
			b.peer.forward("Close")
			return true
		},
		Bind: []any{b},
		Menu: b.appMenu(),
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
	})
}
