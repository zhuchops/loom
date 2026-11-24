//go:build !unix

package editor

var foreground bool

func detachFromTerminal() (bool, error) { return false, nil }
