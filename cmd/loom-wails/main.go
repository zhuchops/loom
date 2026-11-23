//go:build wails

// Command loom-wails is the windowed Loom drawn in a web view. Build it with
// -tags wails,production.
package main

import "github.com/zhuchops/loom/internal/editor"

func main() { editor.MainWails() }
