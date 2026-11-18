package editor

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDiagnosticHoverWrapsAndScrolls(t *testing.T) {
	e := testEditor(t, "vim", "🙂 bad suffix\nnext")
	message := strings.Repeat("long diagnostic message 界 ", 30) + "FINAL"
	e.b.diagnostics = []diagnostic{{Range: lspRange{position{0, 3}, position{0, 6}}, Severity: 1, Message: message}}
	m := &terminalModel{editor: e, view: renderer{screen: newTerminalFrame(40, 12)}}
	m.View()
	pane := m.view.panes[0]
	x, y := pane.x+pane.gutter+3, pane.y
	cursor := e.b.cursor
	m.Update(tea.MouseMotionMsg{X: x, Y: y})
	m.View()
	box := m.view.diagnosticBox
	if box.h == 0 || !strings.Contains(m.view.screen.String(), "Error: long diagnostic") || e.b.cursor != cursor {
		t.Fatal("hover did not show a wrapped diagnostic without moving the cursor")
	}
	for range 100 {
		m.Update(tea.MouseWheelMsg{X: box.x + 1, Y: box.y + 1, Button: tea.MouseWheelDown})
		m.View()
	}
	if !strings.Contains(m.view.screen.String(), "FINAL") || e.b.top != 0 {
		t.Fatal("scrolling the popup did not reach the full message or scrolled the buffer")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.View()
	if m.view.diagnosticBox.h != 0 {
		t.Fatal("keyboard input did not dismiss the diagnostic")
	}
	// A gutter marker also exposes the message.
	m.Update(tea.MouseMotionMsg{X: pane.x, Y: y})
	m.View()
	if m.view.diagnosticBox.h == 0 {
		t.Fatal("gutter hover did not expose the diagnostic")
	}
	m.Update(tea.MouseMotionMsg{X: x + 4, Y: 0})
	m.View()
	if m.view.diagnosticBox.h != 0 {
		t.Fatal("moving away from the affected range kept the popup")
	}
}

func TestProblemPreviewUsesAvailableMessageRows(t *testing.T) {
	e := testEditor(t, "vim", "bad\nnext")
	var lines []string
	for i := range 7 {
		lines = append(lines, fmt.Sprintf("message row %d", i))
	}
	d := diagnostic{Range: lspRange{position{0, 0}, position{0, 3}}, Severity: 1, Message: strings.Join(lines, "\n")}
	p := bufferPreview(e.b, "test.go", 0, 0, true, &d)
	v := renderer{screen: newTerminalFrame(40, 12)}
	scroll := 0
	v.drawItemPreview(p, hitBox{w: 40, h: 10}, &scroll)
	if frame := v.screen.String(); !strings.Contains(frame, "message row 6") || !strings.Contains(frame, "bad") {
		t.Fatal("preview clipped a message that fits alongside its source")
	}
}

func TestHoverDocumentationPreservesFullWrappedContent(t *testing.T) {
	e := testEditor(t, "vim", "abc")
	content := strings.Repeat("long documentation ", 30) + "FINAL"
	raw, _ := json.Marshal(map[string]string{"contents": content})
	e.showHover(raw)
	v := renderer{screen: newTerminalFrame(52, 18)}
	v.draw(e)
	if e.menu.selectedPreview().message != content {
		t.Fatal("hover preview lost documentation")
	}
	e.menu.previewScroll = 1000
	v.draw(e)
	if !strings.Contains(v.screen.String(), "FINAL") {
		t.Fatal("could not scroll to the end of hover documentation")
	}
}
