package editor

import (
	"context"
	"encoding/json"
	"os"
	"strings"
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
		if err := s.client.Request(ctx, "textDocument/hover", params, &raw); err != nil {
			return hoverResult{token: token}
		}
		content := hoverContent(raw)
		if typed := s.typeDocumentation(ctx, doc, content); typed != "" {
			content = joinHover(content, typed)
		}
		return hoverResult{token, content}
	}
}

// joinHover puts the documentation of a symbol's type under its own.
func joinHover(own, typed string) string {
	if own == "" {
		return typed
	}
	return own + "\n\n" + typed
}

// typeDocumentation asks for the type a symbol has (a variable's ent.User,
// say) and returns the hover text of that type's declaration, so the
// documentation of the type shows up beside the symbol's own. It returns ""
// when the server has no type for the symbol, when the symbol is the type
// itself, or when the type's text says nothing new. The caller holds s.mu
// and has synced doc.
func (s *languageSession) typeDocumentation(ctx context.Context, doc document, own string) string {
	params := docParams(doc)
	params["position"] = doc.position
	var raw json.RawMessage
	if s.client.Request(ctx, "textDocument/typeDefinition", params, &raw) != nil {
		return ""
	}
	var locations []location
	if json.Unmarshal(raw, &locations) != nil {
		var one location
		if json.Unmarshal(raw, &one) != nil {
			return ""
		}
		locations = []location{one}
	}
	if len(locations) == 0 {
		return ""
	}
	loc := locations[0]
	if loc.TargetURI != "" {
		loc.URI, loc.Range = loc.TargetURI, loc.TargetSelectionRange
	}
	path := pathFromURI(loc.URI)
	if path == "" {
		return ""
	}
	if path == doc.path && loc.Range.contains(doc.position) {
		return "" // already looking at the type
	}
	uri := fileURI(path)
	if _, open := s.opened[uri]; !open {
		// Servers answer only for files they were given; lend them this one
		// from disk for the one request.
		text, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		if s.client.Notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
			"uri": uri, "version": 0, "languageId": doc.language, "text": string(text),
		}}) != nil {
			return ""
		}
		defer s.client.Notify("textDocument/didClose", map[string]any{"textDocument": map[string]string{"uri": uri}})
	}
	raw = nil
	if s.client.Request(ctx, "textDocument/hover", map[string]any{
		"textDocument": map[string]string{"uri": uri}, "position": loc.Range.Start,
	}, &raw) != nil {
		return ""
	}
	typed := hoverContent(raw)
	if typed == "" || typed == own || strings.Contains(own, typed) {
		return ""
	}
	return typed
}
