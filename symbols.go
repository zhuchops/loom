package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"
)

// symbolInfo covers both shapes servers send: DocumentSymbol (hierarchical,
// with range and children) and SymbolInformation (flat, with a location).
type symbolInfo struct {
	Name           string       `json:"name"`
	Detail         string       `json:"detail"`
	Kind           int          `json:"kind"`
	Range          *lspRange    `json:"range"`
	SelectionRange *lspRange    `json:"selectionRange"`
	Location       *location    `json:"location"`
	ContainerName  string       `json:"containerName"`
	Children       []symbolInfo `json:"children"`
}

type flatSymbol struct {
	name, kind, detail, path string
	depth, line, char        int
}

var symbolKinds = map[int]string{
	1: "file", 2: "module", 3: "namespace", 4: "package", 5: "class", 6: "method", 7: "property",
	8: "field", 9: "constructor", 10: "enum", 11: "interface", 12: "function", 13: "variable",
	14: "constant", 15: "string", 16: "number", 17: "boolean", 18: "array", 19: "object", 20: "key",
	21: "null", 22: "enum member", 23: "struct", 24: "event", 25: "operator", 26: "type parameter",
}

// flattenSymbols turns a symbol list into rows. Children follow their parent,
// indented one level deeper. Symbols without a file path use fallbackPath.
func flattenSymbols(syms []symbolInfo, fallbackPath string, depth int, out []flatSymbol) []flatSymbol {
	for _, s := range syms {
		row := flatSymbol{name: s.Name, kind: symbolKinds[s.Kind], detail: s.Detail, path: fallbackPath, depth: depth}
		if s.Location != nil {
			row.path = pathFromURI(s.Location.URI)
			row.line, row.char = s.Location.Range.Start.Line, s.Location.Range.Start.Character
		} else if s.SelectionRange != nil {
			row.line, row.char = s.SelectionRange.Start.Line, s.SelectionRange.Start.Character
		} else if s.Range != nil {
			row.line, row.char = s.Range.Start.Line, s.Range.Start.Character
		}
		if row.path != "" {
			out = append(out, row)
		}
		out = flattenSymbols(s.Children, fallbackPath, depth+1, out)
	}
	return out
}

func symbolOptions(e *editor, syms []flatSymbol) []option {
	var options []option
	for _, s := range syms {
		label, _ := filepath.Rel(e.root, s.path)
		if label == "" || strings.HasPrefix(label, "..") {
			label = s.path
		}
		name := strings.Repeat("  ", s.depth) + s.name
		hint := strings.TrimSpace(s.kind + " " + s.detail)
		if label != "" {
			hint = strings.TrimSpace(hint + "  " + label)
		}
		path, line, char := s.path, s.line, s.char
		options = append(options, option{name, hint, func() { e.jumpTo(path, line, char, true) }, e.filePreview(path, line, char, true, nil)})
	}
	return options
}

// lspRequest runs one request on the session, syncing doc first.
func (e *editor) lspRequest(s *languageSession, doc document, method string, params map[string]any) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.connect(); err != nil {
		return nil, err
	}
	if err := s.sync(doc); err != nil {
		return nil, err
	}
	var raw json.RawMessage
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.client.request(ctx, method, params, &raw)
	return raw, err
}

func parseSymbols(raw json.RawMessage) ([]symbolInfo, error) {
	var syms []symbolInfo
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	return syms, json.Unmarshal(raw, &syms)
}

// documentSymbols opens a picker of the symbols in the current file.
func (e *editor) documentSymbols() {
	b := e.b
	s := e.session(b, true)
	if s == nil {
		e.notify("No language server for this buffer")
		return
	}
	doc := e.document(b)
	e.notify("Requesting symbols…")
	go func() {
		raw, err := e.lspRequest(s, doc, "textDocument/documentSymbol", docParams(doc))
		e.async <- func() {
			if err != nil {
				e.notify("LSP: " + err.Error())
				return
			}
			syms, err := parseSymbols(raw)
			if err != nil {
				e.notify("LSP: " + err.Error())
				return
			}
			flat := flattenSymbols(syms, doc.path, 0, nil)
			if len(flat) == 0 {
				e.notify("No symbols in this file")
				return
			}
			e.message = ""
			e.openMenu(&menu{title: "Symbols in " + filepath.Base(doc.path), options: symbolOptions(e, flat)})
		}
	}()
}

// workspaceSymbols opens a picker that asks the server as you type.
func (e *editor) workspaceSymbols() {
	b := e.b
	s := e.session(b, true)
	if s == nil {
		e.notify("No language server for this buffer")
		return
	}
	doc := e.document(b)
	m := &menu{title: "Symbols in Workspace"}
	cache := map[string][]option{}
	pending := map[string]bool{}
	m.suggestions = func(query string) []option {
		if options, ok := cache[query]; ok || pending[query] {
			return options
		}
		pending[query] = true
		go func() {
			raw, err := e.lspRequest(s, doc, "workspace/symbol", map[string]any{"query": query})
			var syms []symbolInfo
			if err == nil {
				syms, err = parseSymbols(raw)
			}
			e.async <- func() {
				delete(pending, query)
				if err != nil {
					if e.menu == m {
						e.notify("LSP: " + err.Error())
					}
					return
				}
				cache[query] = symbolOptions(e, flattenSymbols(syms, doc.path, 0, nil))
				if e.menu == m {
					m.filter()
				}
			}
		}()
		return nil
	}
	e.openMenu(m)
}
