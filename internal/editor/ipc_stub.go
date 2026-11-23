//go:build !desktop && !wails

package editor

// The terminal build never hands files to another window.
var newWindow bool

func forwardToRunning([]string) bool { return false }

var foreground bool

func detachFromTerminal() (bool, error) { return false, nil }
