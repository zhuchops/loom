package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSymbolHoverWrapsScrollsAndDiscardsStaleResults(t *testing.T) {
	e := testEditor(t, "vim", "symbol other")
	m := &terminalModel{editor: e, view: renderer{screen: newTerminalFrame(40, 12)}}
	m.View()
	pane := m.view.panes[0]
	cursor := e.b.cursor
	m.Update(tea.MouseMotionMsg{X: pane.x + pane.gutter, Y: pane.y})
	if m.hoverTarget.buffer != e.b || m.hoverTarget.at != 0 {
		t.Fatal("mouse hover did not target the symbol")
	}
	token := m.hoverToken
	content := strings.Repeat("documentation with long lines 界 ", 30) + "FINAL"
	m.Update(hoverResult{token: token, content: content})
	m.View()
	box := m.view.diagnosticBox
	if box.h == 0 || !strings.Contains(m.view.screen.String(), "Hover Documentation") || e.b.cursor != cursor {
		t.Fatal("symbol hover did not show documentation without moving the cursor")
	}
	for range 100 {
		m.Update(tea.MouseWheelMsg{X: box.x + 1, Y: box.y + 1, Button: tea.MouseWheelDown})
		m.View()
	}
	if !strings.Contains(m.view.screen.String(), "FINAL") {
		t.Fatal("could not read the end of symbol documentation")
	}
	m.Update(tea.MouseMotionMsg{X: pane.x + pane.gutter, Y: 0})
	m.Update(hoverResult{token: token, content: content})
	m.View()
	if m.view.diagnosticBox.h != 0 || m.view.diagnosticHover.message != "" {
		t.Fatal("stale hover response reopened a dismissed popup")
	}
	if m.requestHover(token) != nil {
		t.Fatal("stale hover timer started a request")
	}
}

func TestHoverContentFormats(t *testing.T) {
	for _, raw := range []string{
		`{"contents":" first\nsecond "}`,
		`{"contents":{"kind":"markdown","value":" first\nsecond "}}`,
		`{"contents":["first",{"language":"go","value":"second"}]}`,
	} {
		if got := hoverContent([]byte(raw)); got != "first\nsecond" {
			t.Errorf("hoverContent(%s) = %q", raw, got)
		}
	}
}

func TestDiagnosticMouseHoverIncludesDocumentation(t *testing.T) {
	e := testEditor(t, "vim", "bad symbol")
	e.b.diagnostics = []diagnostic{{Range: lspRange{position{0, 0}, position{0, 3}}, Severity: 1, Message: "type error"}}
	m := &terminalModel{editor: e, view: renderer{screen: newTerminalFrame(50, 12)}}
	m.View()
	pane := m.view.panes[0]
	m.Update(tea.MouseMotionMsg{X: pane.x + pane.gutter, Y: pane.y})
	if m.hoverTarget.buffer != e.b || !strings.Contains(m.view.diagnosticHover.message, "type error") {
		t.Fatal("diagnostic hover did not show the error and request documentation")
	}
	token := m.hoverToken
	m.Update(hoverResult{token: token, content: "symbol documentation"})
	m.View()
	if text := m.view.screen.String(); !strings.Contains(text, "type error") || !strings.Contains(text, "symbol documentation") {
		t.Fatal("hover did not display both diagnostic and documentation")
	}
	m.Update(hoverResult{token: token, content: ""})
	if !strings.Contains(m.view.diagnosticHover.message, "type error") {
		t.Fatal("empty documentation removed the diagnostic")
	}
	m.Update(tea.MouseMotionMsg{X: 0, Y: 0})
	m.Update(hoverResult{token: token, content: "stale documentation"})
	if strings.Contains(m.view.diagnosticHover.message, "stale documentation") {
		t.Fatal("stale hover response was displayed")
	}
}
