package editor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zhuchops/loom/internal/jsonrpc"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

func (p position) before(q position) bool {
	return p.Line < q.Line || p.Line == q.Line && p.Character < q.Character
}

// contains reports whether p lies in r, its end included.
func (r lspRange) contains(p position) bool { return !p.before(r.Start) && !r.End.before(p) }
type textEdit struct {
	Range   *lspRange `json:"range"`
	Insert  *lspRange `json:"insert"`
	NewText string    `json:"newText"`
}
type completionItem struct {
	Label            string          `json:"label"`
	Kind             int             `json:"kind"`
	FilterText       string          `json:"filterText"`
	Detail           string          `json:"detail"`
	Documentation    json.RawMessage `json:"documentation"`
	InsertText       string          `json:"insertText"`
	InsertTextFormat int             `json:"insertTextFormat"`
	TextEdit         *textEdit       `json:"textEdit"`
	Additional       []textEdit      `json:"additionalTextEdits"`
	// importPath is set by Loom for an export of a package the file has not
	// imported yet; accepting the item adds the import.
	importPath string
}
type document struct {
	path, language, text string
	version              int
	position, end        position
}
type languageSession struct {
	ctx                              context.Context
	cancel                           context.CancelFunc
	restarting                       bool // Editor event loop only.
	onNotification                   func(jsonrpc.Message)
	semanticFull                     bool
	semanticTypes, semanticModifiers []string
	mu                               sync.Mutex
	root                             string
	argv                             []string
	client                           *jsonrpc.Client
	opened                           map[string]document
	syncKind                         int
	save                             bool
	saveText                         bool
	triggers                         []string
	supportsSignatureHelp            bool
	signatureTriggers                []string
}

func fileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func (s *languageSession) connect() error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if s.client != nil {
		select {
		case <-s.client.Done():
			s.client = nil
		default:
			return nil
		}
	}
	c, err := jsonrpc.Start(s.ctx, s.argv, s.root, s.onNotification)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
	defer cancel()
	var result struct {
		Capabilities struct {
			SemanticProvider struct {
				Legend struct{ TokenTypes, TokenModifiers []string }
				Full   json.RawMessage
			} `json:"semanticTokensProvider"`
			PositionEncoding string          `json:"positionEncoding"`
			TextDocumentSync json.RawMessage `json:"textDocumentSync"`
			SignatureHelp    *struct {
				TriggerCharacters   []string `json:"triggerCharacters"`
				RetriggerCharacters []string `json:"retriggerCharacters"`
			} `json:"signatureHelpProvider"`
			CompletionProvider struct {
				TriggerCharacters []string `json:"triggerCharacters"`
			} `json:"completionProvider"`
		} `json:"capabilities"`
	}
	params := map[string]any{
		"processId": os.Getpid(), "rootUri": fileURI(s.root),
		"workspaceFolders": []map[string]string{{"uri": fileURI(s.root), "name": filepath.Base(s.root)}},
		"capabilities": map[string]any{
			"workspace": map[string]any{"workspaceEdit": map[string]bool{"documentChanges": true}},
			"general":   map[string]any{"positionEncodings": []string{"utf-16"}},
			"textDocument": map[string]any{
				"codeAction": map[string]any{
					"codeActionLiteralSupport": map[string]any{"codeActionKind": map[string]any{"valueSet": []string{"", "quickfix", "refactor", "refactor.extract", "refactor.inline", "refactor.rewrite", "source", "source.organizeImports", "source.fixAll"}}},
					"dataSupport":              true, "resolveSupport": map[string]any{"properties": []string{"edit"}}, "disabledSupport": true,
				},
				"completion": map[string]any{"completionItem": map[string]bool{"snippetSupport": false, "insertReplaceSupport": true}},
				"signatureHelp": map[string]any{"contextSupport": true, "signatureInformation": map[string]any{
					"parameterInformation": map[string]bool{"labelOffsetSupport": true}, "activeParameterSupport": true,
				}},
				"semanticTokens":     map[string]any{"requests": map[string]bool{"full": true}, "tokenTypes": tokenTypes, "tokenModifiers": tokenModifiers, "formats": []string{"relative"}},
				"publishDiagnostics": map[string]bool{"versionSupport": true},
				"hover":              map[string]any{"contentFormat": []string{"plaintext", "markdown"}},
				"typeDefinition":     map[string]bool{"linkSupport": true},
			},
		},
	}
	if filepath.Base(s.argv[0]) == "gopls" {
		params["initializationOptions"] = map[string]bool{"semanticTokens": true}
	}
	if err := c.Request(ctx, "initialize", params, &result); err != nil {
		c.Kill()
		return err
	}
	cap := result.Capabilities
	full := string(cap.SemanticProvider.Full)
	s.semanticFull = full != "" && full != "false" && full != "null"
	s.semanticTypes, s.semanticModifiers = cap.SemanticProvider.Legend.TokenTypes, cap.SemanticProvider.Legend.TokenModifiers
	if cap.PositionEncoding != "" && cap.PositionEncoding != "utf-16" {
		c.Kill()
		return fmt.Errorf("unsupported LSP position encoding %q", cap.PositionEncoding)
	}
	s.syncKind, s.save, s.saveText = 0, false, false
	if json.Unmarshal(cap.TextDocumentSync, &s.syncKind) != nil {
		var options struct {
			Change int             `json:"change"`
			Save   json.RawMessage `json:"save"`
		}
		_ = json.Unmarshal(cap.TextDocumentSync, &options)
		s.syncKind = options.Change
		s.save = len(options.Save) > 0 && string(options.Save) != "false" && string(options.Save) != "null"
		var save struct {
			IncludeText bool `json:"includeText"`
		}
		_ = json.Unmarshal(options.Save, &save)
		s.saveText = save.IncludeText
	}
	if err := c.Notify("initialized", map[string]any{}); err != nil {
		c.Kill()
		return err
	}
	s.client, s.opened, s.triggers = c, make(map[string]document), cap.CompletionProvider.TriggerCharacters
	s.supportsSignatureHelp = cap.SignatureHelp != nil
	s.signatureTriggers = nil
	if cap.SignatureHelp != nil {
		s.signatureTriggers = append(cap.SignatureHelp.TriggerCharacters, cap.SignatureHelp.RetriggerCharacters...)
	}
	return nil
}

var errStale = errors.New("stale document version")

func (s *languageSession) sync(doc document) error {
	c := s.client
	uri := fileURI(doc.path)
	previous, ok := s.opened[uri]
	if previous.version > doc.version {
		return errStale
	}
	if !ok {
		err := c.Notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
			"uri": uri, "version": doc.version, "languageId": doc.language, "text": doc.text,
		}})
		if err != nil {
			return err
		}
	} else if previous.version < doc.version && s.syncKind != 0 {
		change := map[string]any{"text": doc.text}
		if s.syncKind == 2 {
			change["range"] = lspRange{End: previous.end}
		}
		err := c.Notify("textDocument/didChange", map[string]any{
			"textDocument":   map[string]any{"uri": uri, "version": doc.version},
			"contentChanges": []map[string]any{change},
		})
		if err != nil {
			return err
		}
	}
	// Only the version and ending position are needed for the next change.
	s.opened[uri] = document{version: doc.version, end: doc.end}
	return nil
}

func (s *languageSession) complete(doc document, trigger int, char string) ([]completionItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.connect(); err != nil {
		return nil, err
	}
	if err := s.sync(doc); err != nil {
		return nil, err
	}
	contextParams := map[string]any{"triggerKind": trigger}
	if char != "" && slices.Contains(s.triggers, char) {
		contextParams = map[string]any{"triggerKind": 2, "triggerCharacter": char}
	}
	params := map[string]any{"textDocument": map[string]string{"uri": fileURI(doc.path)}, "position": doc.position, "context": contextParams}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var raw json.RawMessage
	if err := s.client.Request(ctx, "textDocument/completion", params, &raw); err != nil {
		return nil, err
	}
	var items []completionItem
	if err := json.Unmarshal(raw, &items); err != nil {
		var list struct {
			Items []completionItem `json:"items"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		items = list.Items
	}
	return plainCompletions(items), nil
}

var snippetPlaceholder = regexp.MustCompile(`\$\{\d+(?::([^}]*))?\}|\$\d+`)

// plainCompletions drops snippet items, which the editor cannot expand, but
// keeps calls such as puts(${1:item}) as the bare name: accepting one adds the
// brackets and leaves the cursor between them.
func plainCompletions(items []completionItem) []completionItem {
	out := items[:0]
	for _, item := range items {
		if item.InsertTextFormat == 2 {
			call := item.Kind == 2 || item.Kind == 3 || item.Kind == 4
			text := item.InsertText
			if item.TextEdit != nil {
				text = item.TextEdit.NewText
			}
			name, _, found := strings.Cut(text, "(")
			if !call || !found || name == "" || snippetPlaceholder.MatchString(name) {
				continue
			}
			if item.TextEdit != nil {
				edit := *item.TextEdit
				edit.NewText = name
				item.TextEdit = &edit
			}
			if item.InsertText != "" {
				item.InsertText = name
			}
			item.InsertTextFormat = 1
		}
		out = append(out, item)
	}
	return out
}

func (s *languageSession) didSave(doc document) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil || s.sync(doc) != nil || !s.save {
		return
	}
	params := map[string]any{"textDocument": map[string]string{"uri": fileURI(doc.path)}}
	if s.saveText {
		params["text"] = doc.text
	}
	_ = s.client.Notify("textDocument/didSave", params)
}

func (s *languageSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.cancel()
	if s.client == nil {
		return
	}
	c := s.client
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_ = c.Request(ctx, "shutdown", nil, nil)
	_ = c.Notify("exit", nil)
	select {
	case <-c.Done():
	case <-ctx.Done():
		c.Kill()
	}
}

func (b *buffer) lspPosition(offset int) position {
	row := b.rowAt(offset)
	col := 0
	for _, r := range b.text[b.lines[row]:offset] {
		col += utf16.RuneLen(r)
	}
	return position{row, col}
}

func (b *buffer) offset(pos position) (int, bool) {
	if pos.Line < 0 || pos.Line >= len(b.lines) || pos.Character < 0 {
		return 0, false
	}
	i, end := b.lines[pos.Line], b.lineEnd(pos.Line)
	col := 0
	for ; i < end && col < pos.Character; i++ {
		col += utf16.RuneLen(b.text[i])
	}
	return i, col == pos.Character
}
