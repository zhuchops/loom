//go:build desktop && darwin

package editor

// #include <stdlib.h>
import "C"

import "strings"

// What the Dock menu in wails_darwin.go asks of Go. It runs on the main
// thread, so it reads the list from disk instead of asking the editor.

//export loomDockProjects
func loomDockProjects() *C.char {
	return C.CString(strings.Join(dockProjects(), "\n"))
}

//export loomDockOpen
func loomDockOpen(path *C.char) {
	go dockOpen(C.GoString(path))
}
