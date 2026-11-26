//go:build !desktop

package editor

// The terminal build never hands files to another window.
var newWindow bool

func forwardToRunning([]string, bool) bool { return false }
