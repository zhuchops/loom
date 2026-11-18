package editor

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

	tea "charm.land/bubbletea/v2"
	"github.com/rivo/uniseg"
)

type signatureHelp struct {
	Signatures      []signatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

type signatureInformation struct {
	Label           string `json:"label"`
	ActiveParameter *int   `json:"activeParameter"`
	Parameters      []struct {
		Label json.RawMessage `json:"label"`
	} `json:"parameters"`
}

type signatureHint struct {
	label      string
	start, end int // Active parameter byte offsets within the label.
}

func signatureLabelOffset(label string, units int) int {
	for i, r := range label {
		if units == 0 {
			return i
		}
		units -= utf16.RuneLen(r)
		if units < 0 {
			return -1
		}
	}
	if units == 0 {
		return len(label)
	}
	return -1
}

func (help signatureHelp) hint() signatureHint {
	if len(help.Signatures) == 0 {
		return signatureHint{}
	}
	index := help.ActiveSignature
	if index < 0 || index >= len(help.Signatures) {
		index = 0
	}
	sig := help.Signatures[index]
	hint := signatureHint{label: sig.Label}
	active := help.ActiveParameter
	if sig.ActiveParameter != nil {
		active = *sig.ActiveParameter
	}
	if active < 0 || active >= len(sig.Parameters) {
		return hint
	}
	from := strings.IndexByte(sig.Label, '(') + 1
	for i, parameter := range sig.Parameters[:active+1] {
		var offsets []int
		var start, end int
		if json.Unmarshal(parameter.Label, &offsets) == nil && len(offsets) == 2 {
			start = signatureLabelOffset(sig.Label, offsets[0])
			end = signatureLabelOffset(sig.Label, offsets[1])
		} else {
			var label string
			if json.Unmarshal(parameter.Label, &label) != nil || label == "" {
				continue
			}
			at := strings.Index(sig.Label[from:], label)
			if at < 0 {
				continue
			}
			start, end = from+at, from+at+len(label)
		}
		if start >= 0 && end > start {
			from = end
			if i == active {
				hint.start, hint.end = start, end
			}
		}
	}
	return hint
}

func (s *languageSession) requestSignature(ctx context.Context, doc document, trigger string, retrigger bool) (signatureHint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return signatureHint{}, err
	}
	if err := s.connect(); err != nil {
		return signatureHint{}, err
	}
	if err := ctx.Err(); err != nil || !s.supportsSignatureHelp {
		return signatureHint{}, err
	}
	if err := s.sync(doc); err != nil {
		return signatureHint{}, err
	}
	params := docParams(doc)
	params["position"] = doc.position
	triggerContext := map[string]any{"triggerKind": 3, "isRetrigger": retrigger}
	if slices.Contains(s.signatureTriggers, trigger) {
		triggerContext["triggerKind"], triggerContext["triggerCharacter"] = 2, trigger
	}
	params["context"] = triggerContext
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var result signatureHelp
	err := s.client.Request(ctx, "textDocument/signatureHelp", params, &result)
	return result.hint(), err
}

type signatureTarget struct {
	buffer          *buffer
	path            string
	version, cursor int
}

type signatureReady uint64
type signatureResult struct {
	token   uint64
	session *languageSession
	hint    signatureHint
}

func (m *terminalModel) scheduleSignature() tea.Cmd {
	e := m.editor
	var target signatureTarget
	if e.mode == "insert" && e.menu == nil && e.notificationDetail == nil &&
		e.directory == nil && !e.tree.focused && !e.tree.popup && e.panel == nil && e.diff == nil &&
		e.b.path != "" && len(e.cfg.Languages[e.cfg.languageFor(e.b.path)].Server) > 0 {
		target = signatureTarget{e.b, e.b.path, e.b.version, e.b.cursor}
	}
	if target == m.signatureTarget {
		return nil
	}
	previous := m.signatureTarget
	m.signatureTarget = target
	// Keep the hint visible while the server checks the new position.
	if target.buffer != previous.buffer || target.path != previous.path {
		e.signature = signatureHint{}
	}
	m.signatureToken++
	if m.cancelSignature != nil {
		m.cancelSignature()
		m.cancelSignature = nil
	}
	if target.buffer == nil {
		return nil
	}
	token := m.signatureToken
	delay := 100 * time.Millisecond
	if target.cursor > 0 && strings.ContainsRune("(,)", target.buffer.text[target.cursor-1]) {
		delay = 0
	}
	return tea.Tick(delay, func(time.Time) tea.Msg { return signatureReady(token) })
}

func (m *terminalModel) requestSignature(token uint64) tea.Cmd {
	if token != m.signatureToken || m.signatureTarget.buffer == nil {
		return nil
	}
	e := m.editor
	s := e.session(e.b, true)
	if s == nil {
		return nil
	}
	doc := e.document(e.b)
	var trigger string
	if e.b.cursor > 0 {
		trigger = string(e.b.text[e.b.cursor-1])
	}
	retrigger := e.signature.label != ""
	ctx, cancel := context.WithCancel(s.ctx)
	m.cancelSignature = cancel
	return func() tea.Msg {
		defer cancel()
		hint, err := s.requestSignature(ctx, doc, trigger, retrigger)
		if err != nil {
			hint = signatureHint{}
		}
		return signatureResult{token, s, hint}
	}
}

func (v *renderer) drawSignature(hint signatureHint, top, bottom int) {
	w := v.screen.Width()
	x, y := v.screen.cursorX, v.screen.cursorY
	row := y - 1
	if row < top || v.completeBox.h > 0 && v.completeBox.y < y && y+1 < bottom {
		row = y + 1
	}
	if row < top || row >= bottom {
		return
	}
	const padding = 2
	bold := menuStyle.Bold(true).Foreground(th.yellow)
	// Scroll long overloads horizontally so the active parameter remains visible.
	before := strings.Map(signatureDisplayRune, hint.label[:hint.start])
	active := strings.Map(signatureDisplayRune, hint.label[hint.start:hint.end])
	label := strings.Map(signatureDisplayRune, hint.label)
	limit := w - padding
	skip := max(0, cellWidth(before)+min(cellWidth(active), limit)-limit)
	width := min(w, cellWidth(label)-skip+padding)
	x = max(0, min(x-1, w-width))
	v.screen.FillArea(x, row, width, 1, ' ', menuStyle)
	g := uniseg.NewGraphemes(label)
	column := 0
	for g.Next() && limit > 0 {
		size := g.Width()
		column += size
		if column-size < skip {
			continue
		}
		if size > limit {
			break
		}
		start, end := g.Positions()
		// Controls are ASCII in most labels, but use the normalized offsets
		// so Unicode control characters cannot shift the highlight.
		style := menuStyle
		if hint.end > hint.start && end > len(before) && start < len(before)+len(active) {
			style = bold
		}
		if size > 0 {
			v.screen.Put(x+1, row, g.Str(), size, style)
		}
		x += size
		limit -= size
	}
}

func signatureDisplayRune(r rune) rune {
	if unicode.IsControl(r) {
		return ' '
	}
	return r
}
