//go:build desktop && darwin && !ios

package editor

/*
#cgo CFLAGS: -fobjc-arc -Wno-deprecated-declarations
#cgo LDFLAGS: -framework AppKit

#include <stdlib.h>
#include "desktop_darwin.h"
*/
import "C"

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"gioui.org/app"
	"gioui.org/io/event"
)

const (
	titleHeight = 36
	titleCenter = 16
	titleInset  = 82 // room for the traffic lights
)

// AppKit calls back on the main thread; the desktop owns everything else.
var native struct {
	d atomic.Pointer[desktop]
	sync.Mutex
	next    int
	waiting map[int]func(string)
}

func platformOptions() []app.Option { return []app.Option{app.Decorated(false)} }

func (d *desktop) platformStart() {
	native.d.Store(d)
	d.m.editor.pickFolder = func(start string, chosen func(string)) { d.pickPath(pickFolder, start, chosen) }
}

func (d *desktop) platformEvent(ev event.Event) {
	if v, ok := ev.(app.AppKitViewEvent); ok && v.View != 0 {
		C.loomInstallWindow(C.uintptr_t(v.View))
		d.platformTheme()
	}
}

func (d *desktop) platformTheme() {
	if native.d.Load() != d {
		return
	}
	dark := 0
	if d.c.dark {
		dark = 1
	}
	C.loomSetAppearance(C.int(dark), C.int(d.c.side.R), C.int(d.c.side.G), C.int(d.c.side.B))
}

//export loomMenuAction
func loomMenuAction(id *C.char) {
	command := C.GoString(id)
	if d := native.d.Load(); d != nil {
		// Never make AppKit wait for the event loop.
		go d.post(uiCallback(func() { d.command(command) }))
	}
}

// pickPath shows a Finder panel as a sheet on the window.
func (d *desktop) pickPath(kind pickKind, start string, chosen func(string)) bool {
	if native.d.Load() != d {
		return false
	}
	native.Lock()
	native.next++
	id := native.next
	if native.waiting == nil {
		native.waiting = map[int]func(string){}
	}
	native.waiting[id] = chosen
	native.Unlock()

	dir := C.CString(start)
	defer C.free(unsafe.Pointer(dir))
	C.loomPickPath(C.int(id), C.int(kind), dir)
	return true
}

//export loomPickedPath
func loomPickedPath(id C.int, path *C.char) {
	chosen := C.GoString(path)
	native.Lock()
	done := native.waiting[int(id)]
	delete(native.waiting, int(id))
	native.Unlock()
	if d := native.d.Load(); d != nil && done != nil && chosen != "" {
		go d.post(uiCallback(func() { done(chosen) }))
	}
}

// platformEnvironment repairs what an app started from Finder lacks: the
// shell's PATH, without which no language server is found, and a UTF-8
// locale, without which pbcopy garbles anything but ASCII.
func platformEnvironment() {
	if os.Getenv("LANG") == "" && os.Getenv("LC_ALL") == "" {
		os.Setenv("LANG", "en_US.UTF-8")
	}
	if os.Getenv("TERM") != "" {
		return // started from a shell
	}
	var dirs []string
	add := func(list ...string) {
		for _, p := range list {
			if filepath.IsAbs(p) && !slices.Contains(dirs, p) {
				dirs = append(dirs, p)
			}
		}
	}
	if shell := os.Getenv("SHELL"); filepath.IsAbs(shell) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		const mark = "__LOOM_PATH__"
		cmd := exec.CommandContext(ctx, shell, "-ilc", `printf '`+mark+`%s`+mark+`' "$PATH"`)
		cmd.WaitDelay = 200 * time.Millisecond
		out, _ := cmd.Output()
		if parts := strings.Split(string(out), mark); len(parts) == 3 {
			add(strings.Split(parts[1], ":")...)
		}
	}
	add(strings.Split(os.Getenv("PATH"), ":")...)
	add("/opt/homebrew/bin", "/usr/local/bin", "/usr/local/go/bin")
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, "go/bin"), filepath.Join(home, ".local/bin"), filepath.Join(home, ".cargo/bin"))
	}
	os.Setenv("PATH", strings.Join(dirs, ":"))
}
