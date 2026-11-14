package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestBubbleTeaEditorInput(t *testing.T) {
	e := testEditor(t, "vim", "界 alpha\nsecond")
	m := &terminalModel{editor: e, view: renderer{screen: newTerminalFrame(0, 0)}}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	initial := m.View()
	if !initial.AltScreen || initial.MouseMode != tea.MouseModeCellMotion || initial.Cursor == nil || initial.Cursor.Shape != tea.CursorBlock {
		t.Fatal("missing fullscreen, mouse, or normal-mode cursor")
	}
	x, y := initial.Cursor.X, initial.Cursor.Y
	m.Update(tea.MouseClickMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
	m.View()
	m.Update(tea.MouseMotionMsg{X: x + 2, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: x + 2, Y: y, Button: tea.MouseLeft})
	s := e.b.selection()
	if e.mode != "select" || string(e.b.text[s.start:s.end]) != "界 " {
		t.Fatalf("mouse drag selected %q in %s mode", string(e.b.text[s.start:s.end]), e.mode)
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(tea.KeyPressMsg{Code: ':'})
	palette := m.View()
	if !strings.Contains(palette.Content, "Command Palette") || palette.Cursor == nil || palette.Cursor.Shape != tea.CursorBar {
		t.Fatal("missing command palette or prompt cursor")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(tea.KeyPressMsg{Code: 'i'})
	m.Update(tea.PasteMsg{Content: "(\n\t"})
	if got := string(e.b.text); got != "界(\n\t alpha\nsecond" {
		t.Fatalf("unexpected paste result: %q", got)
	}

	m.Update(tea.WindowSizeMsg{Width: 3, Height: 3})
	if m.View().Cursor != nil || len(m.view.panes) != 0 {
		t.Fatal("small terminal kept a stale cursor or mouse target")
	}
}
