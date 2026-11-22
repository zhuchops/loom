//go:build desktop && (!darwin || ios)

package editor

import (
	"gioui.org/app"
	"gioui.org/io/event"
)

const (
	titleHeight = 38
	titleCenter = 19
	titleInset  = 12
)

func platformEnvironment() {}

func platformOptions() []app.Option { return nil }

func (d *desktop) platformStart() {}

func (d *desktop) platformEvent(event.Event) {}

func (d *desktop) platformTheme() {}

// pickPath reports that there is no system dialog, so callers ask in the editor.
func (d *desktop) pickPath(pickKind, string, func(string)) bool { return false }
