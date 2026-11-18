package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type workspaceEdit struct {
	Changes         map[string][]textEdit
	DocumentChanges []struct {
		Kind         string
		TextDocument struct {
			URI     string
			Version *int
		}
		Edits []textEdit
	}
}
type codeAction struct {
	Title    string
	Kind     string
	Edit     *workspaceEdit
	Data     json.RawMessage
	Command  json.RawMessage
	Disabled *struct{ Reason string }
}

type actionTarget struct {
	buffer   *buffer
	doc      document
	cursor   int
	versions map[string]int
}

func (e *editor) actionTarget() actionTarget {
	t := actionTarget{e.b, e.document(e.b), e.b.cursor, map[string]int{}}
	for _, b := range e.buffers {
		if b.path != "" {
			t.versions[b.path] = b.version
		}
	}
	return t
}
func (t actionTarget) current(e *editor) bool {
	return e.b == t.buffer && e.b.path == t.doc.path && e.b.version == t.doc.version && e.b.cursor == t.cursor
}

func (e *editor) requestAction(t actionTarget, method string, params any, callback func(json.RawMessage)) {
	s := e.session(t.buffer, true)
	if s == nil {
		e.notify("No language server for this buffer")
		return
	}
	// Capture other open documents in this server's workspace so rename and
	// refactors see their unsaved content.
	var others []document
	for _, b := range e.buffers {
		if b != t.buffer && b.path != "" && e.session(b, false) == s {
			others = append(others, e.document(b))
		}
	}
	go func() {
		s.mu.Lock()
		err := s.connect()
		for _, doc := range others {
			if err == nil {
				err = s.sync(doc)
			}
		}
		if err == nil {
			err = s.sync(t.doc)
		}
		var raw json.RawMessage
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			err = s.client.Request(ctx, method, params, &raw)
			cancel()
		}
		s.mu.Unlock()
		e.async <- func() {
			if s.ctx.Err() != nil || !t.current(e) || e.menu != nil || e.directory != nil || e.tree.focused {
				return
			}
			if err != nil {
				e.notify("LSP: " + err.Error())
				return
			}
			callback(raw)
		}
	}()
}

func (e *editor) codeActions() {
	if e.directory != nil {
		return
	}
	t := e.actionTarget()
	at := t.doc.position
	r := lspRange{at, at}
	if e.b.anchor >= 0 {
		s := e.b.selection()
		r = lspRange{e.b.lspPosition(s.start), e.b.lspPosition(s.end)}
	}
	diagnostics := make([]diagnostic, 0)
	start, _ := e.b.offset(r.Start)
	end, _ := e.b.offset(r.End)
	for _, d := range e.b.diagnostics {
		a, aok := e.b.offset(d.Range.Start)
		b, bok := e.b.offset(d.Range.End)
		if aok && bok && a <= end && b >= start {
			diagnostics = append(diagnostics, d)
		}
	}
	params := docParams(t.doc)
	params["range"] = r
	params["context"] = map[string]any{"diagnostics": diagnostics, "triggerKind": 1}
	e.notify("Requesting code actions…")
	e.requestAction(t, "textDocument/codeAction", params, func(raw json.RawMessage) {
		var actions []json.RawMessage
		if err := json.Unmarshal(raw, &actions); err != nil {
			e.notify(err.Error())
			return
		}
		m := &menu{title: "Code Actions", hidePreview: true}
		for _, wire := range actions {
			var a codeAction
			if json.Unmarshal(wire, &a) != nil || a.Title == "" {
				continue
			}
			hint := a.Kind
			if a.Disabled != nil {
				hint = a.Disabled.Reason
			} else if a.Edit == nil && (len(a.Data) == 0 || string(a.Data) == "null") {
				hint = "command-only · unavailable"
			}
			m.options = append(m.options, option{a.Title, hint, func() { e.selectCodeAction(t, a, wire) }, nil})
		}
		if len(m.options) == 0 {
			e.notify("No code actions here")
			return
		}
		e.message = ""
		e.openMenu(m)
	})
}

func (e *editor) selectCodeAction(t actionTarget, a codeAction, wire json.RawMessage) {
	if !t.current(e) {
		e.notify("Request fresh code actions after editing")
		return
	}
	if a.Disabled != nil {
		e.notify(a.Disabled.Reason)
		return
	}
	if a.Edit != nil && (len(a.Edit.Changes) > 0 || len(a.Edit.DocumentChanges) > 0) {
		e.reviewWorkspaceEdit(t, a.Title, *a.Edit)
		return
	}
	if len(a.Data) > 0 && string(a.Data) != "null" {
		e.requestAction(t, "codeAction/resolve", wire, func(raw json.RawMessage) {
			var resolved codeAction
			if json.Unmarshal(raw, &resolved) == nil && resolved.Edit != nil {
				e.reviewWorkspaceEdit(t, a.Title, *resolved.Edit)
			} else {
				e.notify("This server action requires a command")
			}
		})
		return
	}
	e.notify("This server action requires a command")
}

func (e *editor) renameSymbol() {
	if e.directory != nil {
		return
	}
	if e.session(e.b, true) == nil {
		e.notify("No language server for this buffer")
		return
	}
	t := e.actionTarget()
	name := ""
	if s, ok := textObject(e.b, 'w', false); ok {
		name = string(e.b.text[s.start:s.end])
	}
	e.openMenu(&menu{title: "Rename Symbol", input: name, submit: func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || !t.current(e) {
			return
		}
		params := docParams(t.doc)
		params["position"], params["newName"] = t.doc.position, name
		e.notify("Requesting symbol rename…")
		e.requestAction(t, "textDocument/rename", params, func(raw json.RawMessage) {
			var edit workspaceEdit
			if err := json.Unmarshal(raw, &edit); err != nil {
				e.notify(err.Error())
				return
			}
			e.reviewWorkspaceEdit(t, "Rename to "+name, edit)
		})
	}})
}

type stagedEdit struct {
	path    string
	buffer  *buffer
	open    bool
	version int
	edits   []edit
	cursor  int
	disk    string
}

func (e *editor) stageWorkspaceEdit(t actionTarget, change workspaceEdit) ([]stagedEdit, error) {
	type fileChange struct {
		items   []textEdit
		version *int
	}
	files := map[string]fileChange{}
	for uri, items := range change.Changes {
		files[uri] = fileChange{items, nil}
	}
	for _, doc := range change.DocumentChanges {
		if doc.Kind != "" {
			return nil, fmt.Errorf("file operations in workspace edits are unsupported")
		}
		uri := doc.TextDocument.URI
		if _, ok := files[uri]; ok {
			return nil, fmt.Errorf("multiple edits for document %s", uri)
		}
		files[uri] = fileChange{doc.Edits, doc.TextDocument.Version}
	}
	var result []stagedEdit
	seen := map[string]bool{}
	for _, uri := range sortedKeys(files) {
		f := files[uri]
		if len(f.items) == 0 {
			continue
		}
		path := pathFromURI(uri)
		if !filepath.IsAbs(path) {
			return nil, fmt.Errorf("invalid workspace file URI")
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, err
		}
		path = resolved
		s := stagedEdit{path: path}
		if seen[path] {
			return nil, fmt.Errorf("duplicate file in workspace edit: %s", path)
		}
		seen[path] = true
		for _, b := range e.buffers {
			if b.path == path {
				s.buffer = b
				s.open = true
				break
			}
		}
		b := s.buffer
		if b == nil {
			if t.versions[path] != 0 {
				return nil, fmt.Errorf("tab closed since request: %s", path)
			}
			b, err = loadBuffer(path)
			if err != nil {
				return nil, err
			}
			s.buffer = b
		} else if v, ok := t.versions[path]; !ok || b.version != v {
			return nil, fmt.Errorf("buffer changed since request: %s", path)
		}
		if f.version != nil && *f.version > 0 && (!s.open || b.version != *f.version) {
			return nil, fmt.Errorf("stale server version for %s", path)
		}
		state, err := readDisk(path)
		if err != nil || !state.exists || !b.diskMatches(state) {
			return nil, fmt.Errorf("file changed on disk: %s", path)
		}
		s.version, s.disk = b.version, state.text
		for _, item := range f.items {
			if item.Range == nil {
				return nil, fmt.Errorf("missing text range in %s", path)
			}
			start, ok := b.offset(item.Range.Start)
			end, valid := b.offset(item.Range.End)
			if !ok || !valid || start > end {
				return nil, fmt.Errorf("invalid text range in %s", path)
			}
			s.edits = append(s.edits, edit{span{start, end}, item.NewText})
		}
		slices.SortStableFunc(s.edits, func(a, b edit) int { return a.start - b.start })
		s.cursor = b.cursor
		last := 0
		for i, ed := range s.edits {
			if i > 0 && ed.start < last {
				return nil, fmt.Errorf("overlapping text edits in %s", path)
			}
			last = ed.end
			if ed.start <= b.cursor {
				s.cursor += len([]rune(ed.text)) - (min(b.cursor, ed.end) - ed.start)
			}
		}
		result = append(result, s)
	}
	return result, nil
}

func (e *editor) reviewWorkspaceEdit(t actionTarget, title string, change workspaceEdit) {
	staged, err := e.stageWorkspaceEdit(t, change)
	if err != nil {
		e.notify(err.Error())
		return
	}
	if len(staged) == 0 {
		e.notify("No changes returned")
		return
	}
	m := &menu{title: fmt.Sprintf("%s · %d files", title, len(staged)), confirmation: true}
	for _, s := range staged {
		path, _ := filepath.Rel(e.root, s.path)
		if path == "" {
			path = s.path
		}
		m.options = append(m.options, option{path, fmt.Sprintf("%d edits", len(s.edits)), nil, nil})
		for _, ed := range s.edits {
			label := fmt.Sprintf("  Line %d", s.buffer.rowAt(ed.start)+1)
			before := strings.ReplaceAll(string(s.buffer.text[ed.start:ed.end]), "\n", " ↵ ")
			after := strings.ReplaceAll(ed.text, "\n", " ↵ ")
			m.options = append(m.options, option{label, before + " → " + after, nil, func() itemPreview {
				return itemPreview{title: path + " · " + strings.TrimSpace(label), message: "Before\n" + string(s.buffer.text[ed.start:ed.end]) + "\n\nAfter\n" + ed.text}
			}})
		}
	}
	m.submit = func(string) {
		if err := e.applyWorkspaceEdit(staged); err != nil {
			e.notify(err.Error())
			return
		}
		e.notify(fmt.Sprintf("Applied to %d buffers · review and save", len(staged)))
	}
	e.openMenu(m)
}

func (e *editor) applyWorkspaceEdit(staged []stagedEdit) error {
	for _, s := range staged {
		state, err := readDisk(s.path)
		if err != nil || !state.exists || state.text != s.disk {
			return fmt.Errorf("file changed since preview: %s", s.path)
		}
		if s.buffer.version != s.version || s.buffer.path != s.path {
			return fmt.Errorf("buffer changed since preview: %s", s.path)
		}
		for _, b := range e.buffers {
			if b.path == s.path && (!s.open || b != s.buffer) {
				return fmt.Errorf("tab changed since preview: %s", s.path)
			}
		}
		if s.open && !slices.Contains(e.buffers, s.buffer) {
			return fmt.Errorf("tab closed since preview: %s", s.path)
		}
	}
	for _, s := range staged {
		b := s.buffer
		b.mergeUndo = false
		b.lastInsert = time.Time{}
		b.replaceAll(s.edits, max(0, s.cursor))
		if !s.open {
			e.buffers = append(e.buffers, b)
		}
	}
	e.clampModalCursor()
	e.invalidateCompletion()
	e.scheduleLanguage()
	e.scheduleGit()

	return nil
}
