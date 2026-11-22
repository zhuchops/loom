//go:build desktop

// Command loom-desktop is the windowed Loom. Build it with -tags desktop.
package main

import "github.com/zhuchops/loom/internal/editor"

func main() { editor.MainDesktop() }
