//go:build wails

package editor

import (
	"runtime"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// KeyEvent is a key press as the web view reports it. Key is the browser's
// key name ("ArrowLeft", "Enter"); Char is the Latin character of the
// physical key ("a" for KeyA on any layout), for chords.
type KeyEvent struct {
	Key   string `json:"key"`
	Char  string `json:"char"`
	Ctrl  bool   `json:"ctrl"`
	Alt   bool   `json:"alt"`
	Shift bool   `json:"shift"`
	Meta  bool   `json:"meta"`
}

var webKeys = map[string]rune{
	"Enter": tea.KeyEnter, "Escape": tea.KeyEscape, "Tab": tea.KeyTab,
	"Backspace": tea.KeyBackspace, "Delete": tea.KeyDelete,
	"ArrowLeft": tea.KeyLeft, "ArrowRight": tea.KeyRight, "ArrowUp": tea.KeyUp, "ArrowDown": tea.KeyDown,
	"Home": tea.KeyHome, "End": tea.KeyEnd, "PageUp": tea.KeyPgUp, "PageDown": tea.KeyPgDown,
	"F1": tea.KeyF1, "F2": tea.KeyF2, "F3": tea.KeyF3, "F4": tea.KeyF4, "F5": tea.KeyF5, "F6": tea.KeyF6,
	"F7": tea.KeyF7, "F8": tea.KeyF8, "F9": tea.KeyF9, "F10": tea.KeyF10, "F11": tea.KeyF11, "F12": tea.KeyF12,
}

// webKey turns a key event into what the terminal build would have read,
// or reports that the key is not one Loom takes.
func webKey(ev KeyEvent) (tea.Key, bool) {
	var mod tea.KeyMod
	name := ev.Key
	if ev.Meta && runtime.GOOS == "darwin" {
		// The menu bar owns Command shortcuts; these move the cursor.
		switch name {
		case "ArrowLeft":
			name = "Home"
		case "ArrowRight":
			name = "End"
		case "ArrowUp":
			name = "PageUp"
		case "ArrowDown":
			name = "PageDown"
		case "Backspace", "Delete":
			mod |= tea.ModSuper // clears to the start or end of a text field
		default:
			return tea.Key{}, false
		}
	} else if ev.Meta {
		mod |= tea.ModCtrl
	}
	if ev.Ctrl {
		mod |= tea.ModCtrl
	}
	if ev.Alt {
		mod |= tea.ModAlt
	}
	if ev.Shift {
		mod |= tea.ModShift
	}
	if code, ok := webKeys[name]; ok {
		return tea.Key{Code: code, Mod: mod}, true
	}
	// Plain printable keys arrive as text; only chords are handled here.
	if mod&(tea.ModCtrl|tea.ModAlt) == 0 {
		return tea.Key{}, false
	}
	if name == " " {
		return tea.Key{Code: tea.KeySpace, Mod: mod}, true
	}
	char := ev.Char
	if char == "" {
		char = strings.ToLower(name)
	}
	if utf8.RuneCountInString(char) != 1 {
		return tea.Key{}, false
	}
	r, _ := utf8.DecodeRuneInString(char)
	if latin, ok := latinKeys[r]; ok {
		r = latin
	}
	return tea.Key{Code: r, Mod: mod}, true
}

// latinKeys names the Latin key in the same place on a Russian layout.
var latinKeys = func() map[rune]rune {
	m := map[rune]rune{}
	latin := []rune("qwertyuiop[]asdfghjkl;'zxcvbnm,.`")
	for i, r := range []rune("йцукенгшщзхъфывапролджэячсмитьбюё") {
		m[r] = latin[i]
	}
	return m
}()

// Key is a key press that is not plain typing.
func (b *Bridge) Key(ev KeyEvent) {
	b.call(func() {
		if q := b.dialog; q != nil {
			n := len(q.Choices)
			switch ev.Key {
			case "Escape":
				b.answer(-1)
			case "Enter":
				b.answer(q.Selected)
			case "Tab", "ArrowLeft":
				// Buttons run right to left, the default on the right.
				q.Selected = (q.Selected + 1) % n
			case "ArrowRight":
				q.Selected = (q.Selected + n - 1) % n
			}
			return
		}
		if k, ok := webKey(ev); ok {
			b.send(tea.KeyPressMsg(k))
		}
	})
}

// Text is typed or composed text.
func (b *Bridge) Text(s string) {
	b.call(func() {
		if s == "" {
			return
		}
		if utf8.RuneCountInString(s) > 1 {
			b.send(tea.PasteMsg{Content: s})
			return
		}
		if r, _ := utf8.DecodeRuneInString(s); r >= 0x20 && r != 0x7f {
			b.send(tea.KeyPressMsg(tea.Key{Code: r, Text: s}))
		}
	})
}

// Mouse is a pointer event on the text grid, in cells. Kind is "down",
// "up" or "move"; button is 0 for the main one, 1 middle, 2 secondary.
func (b *Bridge) Mouse(kind string, x, y, button int, held, shift bool) {
	b.call(func() {
		x, y = min(b.cols-1, max(0, x)), min(b.rows-1, max(0, y))
		mouse := tea.Mouse{X: x, Y: y}
		if shift {
			mouse.Mod = tea.ModShift
		}
		switch button {
		case 1:
			mouse.Button = tea.MouseMiddle
		case 2:
			mouse.Button = tea.MouseRight
		default:
			mouse.Button = tea.MouseLeft
		}
		switch kind {
		case "down":
			b.send(tea.MouseClickMsg(mouse))
		case "up":
			b.send(tea.MouseReleaseMsg(mouse))
		default:
			if !held {
				mouse.Button = tea.MouseNone
			}
			b.send(tea.MouseMotionMsg(mouse))
		}
	})
}

// Wheel scrolls the grid by whole lines and columns at cell x, y.
func (b *Bridge) Wheel(x, y, dx, dy int) {
	b.call(func() {
		e, v := b.m.editor, &b.m.view
		v.clearDiagnosticHover()
		x, y = min(b.cols-1, max(0, x)), min(b.rows-1, max(0, y))
		for _, pv := range v.previews {
			if pv.contains(x, y) {
				if dy != 0 {
					v.previewWheel(pv.p, dy)
				}
				return
			}
		}
		doc := e
		if e.directory != nil {
			doc = e.directory.doc
		}
		for _, pane := range v.panes {
			if pane.contains(x, y) && !pane.b.dashboard {
				v.paneWheel(doc, pane, dy, dx*2)
				return
			}
		}
	})
}
