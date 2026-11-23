//go:build (desktop || wails) && !unix

package editor

var foreground bool

func detachFromTerminal() (bool, error) { return false, nil }
