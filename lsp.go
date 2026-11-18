package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

type rpcMessage struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type rpcClient struct {
	onNotification func(rpcMessage)
	process        *exec.Cmd
	in             io.WriteCloser
	sendMu, mu     sync.Mutex
	next           int
	waiters        map[string]chan rpcMessage
	done           chan struct{}
}

func startRPC(ctx context.Context, argv []string, root string, onNotification func(rpcMessage)) (*rpcClient, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = root
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		in.Close()
		out.Close()
		return nil, err
	}
	c := &rpcClient{onNotification: onNotification, process: cmd, in: in, waiters: make(map[string]chan rpcMessage), done: make(chan struct{})}
	go func() {
		c.read(out)
		in.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		close(c.done)
	}()
	return c, nil
}

func (c *rpcClient) write(message map[string]any) error {
	message["jsonrpc"] = "2.0"
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	_, err = fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
	return err
}

func (c *rpcClient) read(reader io.Reader) {
	r := bufio.NewReader(reader)
	for {
		length := -1
		for {
			line, err := r.ReadSlice('\n')
			if err != nil {
				return
			}
			header := strings.TrimSpace(string(line))
			if header == "" {
				break
			}
			key, value, ok := strings.Cut(header, ":")
			if ok && strings.EqualFold(key, "Content-Length") {
				length, err = strconv.Atoi(strings.TrimSpace(value))
				if err != nil {
					return
				}
			}
		}
		if length < 0 || length > 32<<20 {
			return
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		var msg rpcMessage
		if json.Unmarshal(body, &msg) != nil {
			continue
		}
		id := string(msg.ID)
		if id == "" || id == "null" {
			if c.onNotification != nil {
				c.onNotification(msg)
			}
			continue
		}
		if msg.Method != "" {
			reply := map[string]any{"id": msg.ID}
			switch msg.Method {
			case "workspace/configuration":
				var p struct{ Items []json.RawMessage }
				_ = json.Unmarshal(msg.Params, &p)
				reply["result"] = make([]any, len(p.Items))
			default:
				reply["error"] = map[string]any{"code": -32601, "message": "Method not supported"}
			}
			_ = c.write(reply)
			continue
		}
		c.mu.Lock()
		ch := c.waiters[id]
		c.mu.Unlock()
		if ch != nil {
			select {
			case ch <- msg:
			default:
			}
		}
	}
}

func (c *rpcClient) notify(method string, params any) error {
	return c.write(map[string]any{"method": method, "params": params})
}

func (c *rpcClient) request(ctx context.Context, method string, params any, dest any) error {
	c.mu.Lock()
	c.next++
	id := c.next
	key := strconv.Itoa(id)
	ch := make(chan rpcMessage, 1)
	c.waiters[key] = ch
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.waiters, key); c.mu.Unlock() }()
	if err := c.write(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return err
	}
	select {
	case msg := <-ch:
		if msg.Error != nil {
			return fmt.Errorf("%s", msg.Error.Message)
		}
		if dest != nil {
			return json.Unmarshal(msg.Result, dest)
		}
		return nil
	case <-c.done:
		return errors.New("language server exited")
	case <-ctx.Done():
		return ctx.Err()
	}
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}
type textEdit struct {
	Range   *lspRange `json:"range"`
	Insert  *lspRange `json:"insert"`
	NewText string    `json:"newText"`
}
type completionItem struct {
	Label            string          `json:"label"`
	FilterText       string          `json:"filterText"`
	Detail           string          `json:"detail"`
	Documentation    json.RawMessage `json:"documentation"`
	InsertText       string          `json:"insertText"`
	InsertTextFormat int             `json:"insertTextFormat"`
	TextEdit         *textEdit       `json:"textEdit"`
	Additional       []textEdit      `json:"additionalTextEdits"`
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
	onNotification                   func(rpcMessage)
	semanticFull                     bool
	semanticTypes, semanticModifiers []string
	mu                               sync.Mutex
	root                             string
	argv                             []string
	client                           *rpcClient
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
		case <-s.client.done:
			s.client = nil
		default:
			return nil
		}
	}
	c, err := startRPC(s.ctx, s.argv, s.root, s.onNotification)
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
			},
		},
	}
	if filepath.Base(s.argv[0]) == "gopls" {
		params["initializationOptions"] = map[string]bool{"semanticTokens": true}
	}
	if err := c.request(ctx, "initialize", params, &result); err != nil {
		_ = c.process.Process.Kill()
		return err
	}
	cap := result.Capabilities
	full := string(cap.SemanticProvider.Full)
	s.semanticFull = full != "" && full != "false" && full != "null"
	s.semanticTypes, s.semanticModifiers = cap.SemanticProvider.Legend.TokenTypes, cap.SemanticProvider.Legend.TokenModifiers
	if cap.PositionEncoding != "" && cap.PositionEncoding != "utf-16" {
		_ = c.process.Process.Kill()
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
	if err := c.notify("initialized", map[string]any{}); err != nil {
		_ = c.process.Process.Kill()
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
		err := c.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
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
		err := c.notify("textDocument/didChange", map[string]any{
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
	if err := s.client.request(ctx, "textDocument/completion", params, &raw); err != nil {
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
	return slices.DeleteFunc(items, func(item completionItem) bool { return item.InsertTextFormat == 2 }), nil
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
	_ = s.client.notify("textDocument/didSave", params)
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
	_ = c.request(ctx, "shutdown", nil, nil)
	_ = c.notify("exit", nil)
	select {
	case <-c.done:
	case <-ctx.Done():
		_ = c.process.Process.Kill()
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
