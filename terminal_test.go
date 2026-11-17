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
	if !initial.AltScreen || initial.MouseMode != tea.MouseModeAllMotion || initial.Cursor == nil || initial.Cursor.Shape != tea.CursorBlock {
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

func TestCompletionPopupFollowsCursor(t *testing.T) {
	e := testEditor(t, "vim", strings.Repeat("line\n", 29)+"word")
	m := &terminalModel{editor: e, view: renderer{screen: newTerminalFrame(36, 14)}}
	e.mode = "insert"
	items := make([]completionItem, 12)
	for i := range items {
		items[i].Label = "Earlier"
	}
	items[11].Label = "Selected"
	e.completion.items, e.completion.selected = items, 11

	view := m.View()
	box := m.view.completeBox
	if view.Cursor == nil || box.y != view.Cursor.Y+2 || box.x+1 != view.Cursor.X || box.h != 8 {
		t.Fatalf("expected popup below cursor, got %+v at %+v", box, view.Cursor)
	}

	e.b.cursor = len(e.b.text)
	view = m.View()
	box = m.view.completeBox
	if view.Cursor == nil || box.y+box.h != view.Cursor.Y-1 || box.y < 2 || box.x+box.w >= 36 {
		t.Fatalf("expected popup above cursor within the editor, got %+v at %+v", box, view.Cursor)
	}
	m.Update(tea.WindowSizeMsg{Width: 12, Height: 5})
	view = m.View()
	box = m.view.completeBox
	if view.Cursor == nil || box.h == 0 || box.y+box.h != view.Cursor.Y || box.y < 1 || box.x+box.w > 12 {
		t.Fatalf("expected a compact popup above cursor, got %+v at %+v", box, view.Cursor)
	}
	m.Update(tea.MouseClickMsg{X: box.x, Y: box.y + 11 - box.start, Button: tea.MouseLeft})
	if !strings.HasSuffix(string(e.b.text), "Selected") || len(e.completion.items) != 0 {
		t.Fatal("clicking the visible completion did not accept it")
	}
}

func TestCompletionShowsDocumentationPreview(t *testing.T) {
	e := testEditor(t, "vim", "abc")
	e.mode = "insert"
	e.completion.items = []completionItem{{
		Label: "Append", Detail: "func append", Documentation: []byte(`{"kind":"plaintext","value":"Adds elements.\nSecond.\nThird.\nFourth.\nFifth.\nSixth.\nSeventh."}`),
	}}
	v := renderer{screen: newTerminalFrame(100, 20)}
	v.draw(e)
	if !strings.Contains(v.screen.String(), "Seventh.") || !v.screen.cursorVisible {
		t.Fatal("completion preview did not show documentation alongside the cursor")
	}
}

func TestSignatureHintStaysClearOfCompletion(t *testing.T) {
	e := testEditor(t, "vim", "first\ncall(\nlast")
	e.mode = "insert"
	e.signature = signatureHint{label: "call(name string)", start: len("call("), end: len("call(name string")}
	e.completion.items = []completionItem{{Label: "someName"}}
	v := renderer{screen: newTerminalFrame(60, 14)}
	for _, row := range []int{0, 1} {
		e.b.cursor = e.b.lineEnd(row)
		v.draw(e)
		hintRow := v.screen.cursorY - 1
		if row == 0 {
			hintRow = v.screen.cursorY + 1
		}
		if v.completeBox.y <= hintRow || v.completeBox.h == 0 {
			t.Fatalf("completion overlaps the signature row %d: %+v", hintRow, v.completeBox)
		}
		highlighted := false
		for x := 0; x < v.screen.Width(); x++ {
			char, style, _ := v.screen.Get(x, hintRow)
			highlighted = highlighted || char == "n" && style == menuStyle.Bold(true).Foreground(rgb(0xdbbc7f))
		}
		if !highlighted {
			t.Fatalf("active argument was not highlighted on row %d", hintRow)
		}
	}
}
