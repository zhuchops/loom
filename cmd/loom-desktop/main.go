//go:build desktop

// Command loom-desktop is the windowed Loom, drawn in the system web view.
// Build it with -tags desktop,production (and webkit2_41 on Linux distributions
// that ship WebKitGTK 4.1).
package main

import "github.com/zhuchops/loom/internal/editor"

func main() { editor.MainDesktop() }
