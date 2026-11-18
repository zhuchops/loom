package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var tokenTypes = []string{"namespace", "type", "class", "enum", "interface", "struct", "typeParameter", "parameter", "variable", "property", "enumMember", "event", "function", "method", "macro", "keyword", "modifier", "comment", "string", "number", "regexp", "operator", "decorator"}
var tokenModifiers = []string{"declaration", "definition", "readonly", "static", "deprecated", "abstract", "async", "modification", "documentation", "defaultLibrary"}

type semanticResult struct{ Data []int }
type diagnostic struct {
	Range    lspRange        `json:"range"`
	Severity int             `json:"severity,omitempty"`
	Message  string          `json:"message"`
	Code     json.RawMessage `json:"code,omitempty"`
	Source   string          `json:"source,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}
type diagnosticsEvent struct {
	URI         string
	Version     int
	Diagnostics []diagnostic
}

// problemCount is the number of errors and warnings in one file.
type problemCount struct{ errors, warnings int }

func countProblems(ds []diagnostic) problemCount {
	var c problemCount
	for _, d := range ds {
		if d.Severity <= 1 {
			c.errors++
		} else if d.Severity == 2 {
			c.warnings++
		}
	}
	return c
}

// projectProblems adds up the counts for every file the language servers reported in the project.
func (e *editor) projectProblems() problemCount {
	var total problemCount
	for _, diags := range e.problems {
		c := countProblems(diags)
		total.errors += c.errors
		total.warnings += c.warnings
	}
	return total
}

type location struct {
	URI                  string
	Range                lspRange
	TargetURI            string
	TargetSelectionRange lspRange
}

func pathFromURI(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") {
		return ""
	}
	return filepath.FromSlash(u.Path)
}

func (e *editor) handleServerNotification(s *languageSession, msg rpcMessage) {
	if msg.Method == "window/showMessage" {
		var params struct{ Message string }
		if json.Unmarshal(msg.Params, &params) == nil && params.Message != "" {
			select {
			case e.async <- func() {
				if s == nil || s.ctx.Err() == nil {
					e.notify("LSP: " + params.Message)
				}
			}:
			default:
			}
		}
		return
	}
	if msg.Method != "textDocument/publishDiagnostics" {
		return
	}
	var event diagnosticsEvent
	if json.Unmarshal(msg.Params, &event) != nil {
		return
	}
	path := pathFromURI(event.URI)
	inProject := path != "" && within(e.root, path)
	select {
	case e.async <- func() {
		if s != nil && s.ctx.Err() != nil {
			return
		}
		for _, b := range e.buffers {
			if b.path == path {
				if event.Version != 0 && b.version != event.Version {
					return
				}
				b.diagnostics = event.Diagnostics
				break
			}
		}
		if inProject {
			if len(event.Diagnostics) == 0 {
				delete(e.problems, path)
				delete(e.problemServers, path)
			} else {
				e.problems[path] = event.Diagnostics
				if e.problemServers == nil {
					e.problemServers = make(map[string]*languageSession)
				}
				e.problemServers[path] = s
			}
		}
	}:
	default:
	}
}

func (e *editor) scheduleLanguage() {
	e.languageToken++
	token := e.languageToken
	if e.languageTimer != nil {
		e.languageTimer.Stop()
	}
	if e.b.path == "" || len(e.cfg.Languages[e.cfg.languageFor(e.b.path)].Server) == 0 {
		return
	}
	e.languageTimer = time.AfterFunc(450*time.Millisecond, func() {
		select {
		case e.languageWake <- token:
		default:
		}
	})
}

func (e *editor) refreshLanguage() {
	if e.languageRunning {
		e.languageQueued = true
		return
	}
	b := e.b
	s := e.session(b, true)
	if s == nil {
		return
	}
	doc := e.document(b)
	if b.semanticVersion == doc.version && b.semanticPath == doc.path {
		return
	}
	e.languageRunning = true
	go func() {
		var result semanticResult
		s.mu.Lock()
		err := s.connect()
		if err == nil {
			err = s.sync(doc)
		}
		var types, modifiers []string
		if err == nil && s.semanticFull {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			err = s.client.request(ctx, "textDocument/semanticTokens/full", docParams(doc), &result)
			cancel()
			types, modifiers = s.semanticTypes, s.semanticModifiers
		}
		s.mu.Unlock()
		e.async <- func() {
			e.languageRunning = false
			if err == nil && s.ctx.Err() == nil && b.version == doc.version && b.path == doc.path {
				b.semanticVersion, b.semanticPath = doc.version, doc.path
				b.semantic = decodeSemantic(b, result.Data, types, modifiers)

			}
			if e.languageQueued {
				e.languageQueued = false
				e.refreshLanguage()
			}
		}
	}()
}

func decodeSemantic(b *buffer, data []int, types, modifiers []string) []highlight {
	var result []highlight
	line, col := 0, 0
	for i := 0; i+4 < len(data); i += 5 {
		if data[i] < 0 || data[i+1] < 0 || data[i+2] <= 0 {
			continue
		}
		if data[i] > 0 {
			col = 0
		}
		line += data[i]
		col += data[i+1]
		kind := data[i+3]
		if kind < 0 || kind >= len(types) {
			continue
		}
		start, ok := b.offset(position{line, col})
		end, valid := b.offset(position{line, col + data[i+2]})
		if !ok || !valid {
			continue
		}
		hex := semanticColor(types[kind])
		mask := data[i+4]
		for j, m := range modifiers {
			if j < 31 && mask&(1<<j) != 0 && m == "readonly" && types[kind] == "variable" {
				hex = int32(th.purple)
			}
		}
		if hex != 0 {
			result = append(result, highlight{span{start, end}, baseStyle.Foreground(rgb(hex))})
		}
	}
	return result
}

func semanticColor(kind string) int32 {
	switch kind {
	case "type", "class", "struct", "interface", "enum", "typeParameter":
		return int32(th.typ)
	case "function", "method", "macro":
		return int32(th.accent)
	case "parameter":
		return int32(th.orange)
	case "property", "namespace":
		return int32(th.aqua)
	case "enumMember", "number", "constant":
		return int32(th.purple)
	case "string", "regexp":
		return int32(th.yellow)
	case "keyword", "modifier":
		return int32(th.red)
	case "comment":
		return int32(th.comment)
	case "operator":
		return int32(th.comment)
	case "decorator", "label":
		return int32(th.purple)
	}
	return 0
}

func docParams(doc document) map[string]any {
	return map[string]any{"textDocument": map[string]string{"uri": fileURI(doc.path)}}
}

func (e *editor) closeDocument(path string) {
	if path == "" {
		return
	}
	for _, s := range e.sessions {
		go func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			uri := fileURI(path)
			if _, ok := s.opened[uri]; ok && s.client != nil {
				delete(s.opened, uri)
				_ = s.client.notify("textDocument/didClose", map[string]any{"textDocument": map[string]string{"uri": uri}})
			}
		}()
	}
}

func (e *editor) languageCommands() {
	e.add("lsp-restart", "Restart Language Server", func(e *editor) { e.restartLanguageServer() })
	e.add("code-actions", "Code Actions", func(e *editor) { e.codeActions() })
	e.add("rename-symbol", "Rename Symbol", func(e *editor) { e.renameSymbol() })
	e.add("definition", "Go to Definition", func(e *editor) { e.languageAction("definition") })
	e.add("references", "Find References", func(e *editor) { e.languageAction("references") })
	e.add("implementation", "Go to Implementation", func(e *editor) { e.languageAction("implementation") })
	e.add("symbols", "Go to Symbol in File", func(e *editor) { e.documentSymbols() })
	e.add("workspace-symbols", "Go to Symbol in Workspace", func(e *editor) { e.workspaceSymbols() })
	e.add("hover", "Show Hover Documentation", func(e *editor) { e.languageAction("hover") })
	e.add("problems", "Project Problems", func(e *editor) { e.showProjectProblems() })
	e.add("file-problems", "File Problems", func(e *editor) { e.showFileProblems() })
	e.add("next-problem", "Next Diagnostic", func(e *editor) { e.stepProblem(false) })
	e.add("prev-problem", "Previous Diagnostic", func(e *editor) { e.stepProblem(true) })
}

func (e *editor) restartLanguageServer() {
	key, root, argv := e.sessionKey(e.b)
	if key == "" || e.directory != nil {
		e.notify("No language server for this buffer")
		return
	}
	old := e.sessions[key]
	if old != nil && old.restarting {
		return
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		e.notify("LSP: " + err.Error())
		return
	}
	if old != nil {
		old.cancel()
		for path, server := range e.problemServers {
			if server == old {
				delete(e.problemServers, path)
				delete(e.problems, path)
			}
		}
	}
	s := e.newLanguageSession(key, root, argv)
	s.restarting = true
	e.invalidateCompletion()
	var docs []document
	for _, b := range e.buffers {
		if e.session(b, false) == s {
			b.semantic, b.diagnostics = nil, nil
			b.semanticVersion, b.semanticPath = 0, ""
			docs = append(docs, e.document(b))
		}
	}
	e.notify("Restarting language server…")
	// Hold the fresh session until the previous process stops and our buffer
	// snapshots are sent. Later requests can then sync newer versions normally.
	s.mu.Lock()
	go func() {
		if old != nil {
			old.mu.Lock()
			if old.client != nil {
				<-old.client.done
			}
			old.mu.Unlock()
		}
		err := s.connect()
		for _, doc := range docs {
			if err != nil {
				break
			}
			err = s.sync(doc)
		}
		s.mu.Unlock()
		e.async <- func() {
			s.restarting = false
			if err != nil {
				e.notify("LSP restart failed: " + err.Error())
				return
			}
			e.notify("Language server restarted")
			e.refreshLanguage()
		}
	}()
}

func (e *editor) languageAction(action string) {
	b := e.b
	var hoverMenu *menu
	if action == "hover" {
		hoverMenu = e.openDiagnosticHover(b, b.cursor)
	}
	s := e.session(b, true)
	if s == nil {
		if hoverMenu == nil {
			e.notify("No language server for this buffer")
		}
		return
	}
	doc := e.document(b)
	cursor := b.cursor
	e.notify("Requesting " + action + "…")
	go func() {
		var raw json.RawMessage
		s.mu.Lock()
		err := s.connect()
		if err == nil {
			err = s.sync(doc)
		}
		if err == nil {
			params := docParams(doc)
			params["position"] = doc.position
			if action == "references" {
				params["context"] = map[string]bool{"includeDeclaration": true}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err = s.client.request(ctx, "textDocument/"+action, params, &raw)
			cancel()
		}
		s.mu.Unlock()
		e.async <- func() {
			if s.ctx.Err() != nil || e.b != b || b.version != doc.version || b.path != doc.path || b.cursor != cursor || (e.menu != nil && e.menu != hoverMenu) || e.tree.focused || e.directory != nil || (hoverMenu != nil && e.menu != hoverMenu) {
				return
			}
			if err != nil {
				if hoverMenu == nil {
					e.notify("LSP: " + err.Error())
				}
				return
			}
			if action == "hover" {
				if hoverMenu != nil {
					if content := hoverContent(raw); content != "" {
						e.addHoverOption(hoverMenu, "Documentation", content)
						hoverMenu.filter()
					}
				} else {
					e.showHover(raw)
				}
				return
			}
			e.showLocations(action, raw)
		}
	}()
}

func (e *editor) showLocations(action string, raw json.RawMessage) {
	var locations []location
	if json.Unmarshal(raw, &locations) != nil {
		var item location
		if json.Unmarshal(raw, &item) == nil {
			locations = []location{item}
		}
	}
	m := &menu{title: "References"}
	switch action {
	case "definition":
		m.title = "Definitions"
	case "implementation":
		m.title = "Implementations"
	}
	for _, loc := range locations {
		if loc.TargetURI != "" {
			loc.URI, loc.Range = loc.TargetURI, loc.TargetSelectionRange
		}
		path := pathFromURI(loc.URI)
		if path == "" {
			continue
		}
		p := loc.Range.Start
		label, _ := filepath.Rel(e.root, path)
		if label == "" || strings.HasPrefix(label, "..") {
			label = path
		}
		m.options = append(m.options, option{fmt.Sprintf("%s:%d:%d", label, p.Line+1, p.Character+1), "", func() { e.jumpTo(path, p.Line, p.Character, true) }, e.filePreview(path, p.Line, p.Character, true, nil)})
	}
	if len(m.options) == 0 {
		e.notify("No " + action + " results")
		return
	}
	e.message = ""
	if (action == "definition" || action == "implementation") && len(m.options) == 1 {
		m.options[0].run()
		return
	}
	e.openMenu(m)
}

func hoverContent(raw json.RawMessage) string {
	var result struct{ Contents json.RawMessage }
	_ = json.Unmarshal(raw, &result)
	var content string
	if json.Unmarshal(result.Contents, &content) != nil {
		var markup struct{ Value string }
		if json.Unmarshal(result.Contents, &markup) == nil {
			content = markup.Value
		} else {
			var values []json.RawMessage
			_ = json.Unmarshal(result.Contents, &values)
			for _, value := range values {
				var line string
				if json.Unmarshal(value, &line) != nil {
					var marked struct{ Value string }
					_ = json.Unmarshal(value, &marked)
					line = marked.Value
				}
				content += line + "\n"
			}
		}
	}
	return strings.TrimSpace(content)
}

func (e *editor) showHover(raw json.RawMessage) {
	content := hoverContent(raw)
	if content == "" {
		e.notify("No hover information")
		return
	}
	m := &menu{title: "Hover Documentation", details: true}
	e.addHoverOption(m, "Documentation", content)
	e.openMenu(m)
}

func (e *editor) addHoverOption(m *menu, title, content string) {
	m.options = append(m.options, option{title, "", func() {
		e.showDetails(m, title, content, nil)
	}, func() itemPreview { return itemPreview{title: title, message: content} }})
}

func (e *editor) openDiagnosticHover(b *buffer, at int) *menu {
	m := &menu{title: "Hover Documentation", details: true}
	for _, d := range b.diagnostics {
		start, okStart := b.offset(d.Range.Start)
		end, okEnd := b.offset(d.Range.End)
		if !okStart || !okEnd || start > end || at < start || at >= max(start+1, end) {
			continue
		}
		e.addHoverOption(m, humanize(severityName(d.Severity)), d.Message)
	}
	if len(m.options) == 0 {
		return nil
	}
	e.openMenu(m)
	return m
}

type problemEntry struct {
	path string
	diag diagnostic
}

// showProblemList opens a filterable popup. Enter jumps to the diagnostic.
func (e *editor) showProblemList(title string, entries []problemEntry) {
	if len(entries) == 0 {
		e.notify("No problems")
		return
	}
	slices.SortStableFunc(entries, func(a, b problemEntry) int {
		if c := strings.Compare(a.path, b.path); c != 0 {
			return c
		}
		if c := cmp.Compare(a.diag.Range.Start.Line, b.diag.Range.Start.Line); c != 0 {
			return c
		}
		return cmp.Compare(a.diag.Range.Start.Character, b.diag.Range.Start.Character)
	})
	m := &menu{title: title}
	for _, entry := range entries {
		entry := entry
		p := entry.diag.Range.Start
		rel := filepath.Base(entry.path)
		if r, err := filepath.Rel(e.root, entry.path); err == nil && within(e.root, entry.path) {
			rel = r
		}
		label := fmt.Sprintf("%s:%d:%d  %s", rel, p.Line+1, p.Character+1, strings.Join(strings.Fields(entry.diag.Message), " "))
		m.options = append(m.options, option{label, severityName(entry.diag.Severity), func() {
			e.jumpTo(entry.path, p.Line, p.Character, true)
		}, e.filePreview(entry.path, p.Line, p.Character, true, &entry.diag)})
	}
	e.openMenu(m)
}

func severityName(severity int) string {
	switch severity {
	case 2:
		return "warning"
	case 3:
		return "info"
	case 4:
		return "hint"
	}
	return "error"
}

// showProjectProblems lists diagnostics from every file the language servers reported.
func (e *editor) showProjectProblems() {
	var entries []problemEntry
	for path, diags := range e.problems {
		for _, d := range diags {
			entries = append(entries, problemEntry{path, d})
		}
	}
	c := e.projectProblems()
	e.showProblemList(fmt.Sprintf("Project Problems · %d errors, %d warnings", c.errors, c.warnings), entries)
}

// showFileProblems lists diagnostics for the current file.
func (e *editor) showFileProblems() {
	if e.b.path == "" {
		e.notify("Save this file to see its problems")
		return
	}
	var entries []problemEntry
	for _, d := range e.b.diagnostics {
		entries = append(entries, problemEntry{e.b.path, d})
	}
	c := countProblems(e.b.diagnostics)
	e.showProblemList(fmt.Sprintf("File Problems · %d errors, %d warnings", c.errors, c.warnings), entries)
}

func (e *editor) stepProblem(reverse bool) {
	b := e.b
	next, wrap := -1, -1
	for _, d := range b.diagnostics {
		at, ok := b.offset(d.Range.Start)
		if !ok {
			continue
		}
		if wrap < 0 || (!reverse && at < wrap) || (reverse && at > wrap) {
			wrap = at
		}
		if !reverse && at > b.cursor && (next < 0 || at < next) {
			next = at
		}
		if reverse && at < b.cursor && at > next {
			next = at
		}
	}
	if next < 0 {
		next = wrap
	}
	if next < 0 {
		e.notify("No diagnostics in this file")
		return
	}
	b.cursor = next
	e.clampModalCursor()
	e.scrollLock = false
	for _, d := range b.diagnostics {
		if at, ok := b.offset(d.Range.Start); ok && at == next {
			e.notify(d.Message)
			break
		}
	}
}
