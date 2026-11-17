package main

import (
	"context"
	"encoding/json"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

type hoverTarget struct {
	buffer      *buffer
	path        string
	version, at int
}

type hoverReady uint64
type hoverResult struct {
	token   uint64
	content string
}

func (m *terminalModel) scheduleHover() tea.Cmd {
	d := m.view.diagnosticHover
	var target hoverTarget
	if d.buffer != nil && d.requestDocumentation && diagnosticHoverAllowed(m.editor) &&
		d.version == d.buffer.version && d.at < len(d.buffer.text) && !unicode.IsSpace(d.buffer.text[d.at]) {
		target = hoverTarget{d.buffer, d.buffer.path, d.version, d.at}
	}
	if target == m.hoverTarget {
		return nil
	}
	m.hoverTarget = target
	m.hoverToken++
	if m.cancelHover != nil {
		m.cancelHover()
		m.cancelHover = nil
	}
	if target.buffer == nil {
		return nil
	}
	token := m.hoverToken
	return tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg { return hoverReady(token) })
}

func (m *terminalModel) requestHover(token uint64) tea.Cmd {
	if token != m.hoverToken || m.hoverTarget.buffer == nil {
		return nil
	}
	target := m.hoverTarget
	s := m.editor.session(target.buffer, true)
	if s == nil {
		return nil
	}
	doc := m.editor.document(target.buffer)
	doc.position = target.buffer.lspPosition(target.at)
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	m.cancelHover = cancel
	return func() tea.Msg {
		defer cancel()
		s.mu.Lock()
		defer s.mu.Unlock()
		var raw json.RawMessage
		if ctx.Err() != nil {
			return hoverResult{token: token}
		}
		if err := s.connect(); err != nil {
			return hoverResult{token: token}
		}
		if err := s.sync(doc); err != nil {
			return hoverResult{token: token}
		}
		params := docParams(doc)
		params["position"] = doc.position
		if err := s.client.request(ctx, "textDocument/hover", params, &raw); err != nil {
			return hoverResult{token: token}
		}
		return hoverResult{token, hoverContent(raw)}
	}
}
