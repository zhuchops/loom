package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"
)

type completionState struct {
	running       bool
	queued        bool
	queuedTrigger string
	timer         *time.Timer
	token         uint64
	items         []completionItem
	selected      int
	trigger       string
}

func (e *editor) invalidateCompletion() {
	c := &e.completion
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.token++
	c.queued = false
	c.items = nil
	c.selected = 0
}

func (e *editor) scheduleCompletion(key string) {
	c := &e.completion
	if _, ok := inputRune(key); !ok && key != "backspace" {
		return
	}
	if e.b.cursor == 0 || e.b.path == "" {
		return
	}
	r := e.b.text[e.b.cursor-1]
	if wordClass(r) == 0 {
		return
	}
	c.trigger = string(r)
	token := c.token
	c.timer = time.AfterFunc(140*time.Millisecond, func() {
		select {
		case e.completionWake <- token:
		default:
			select {
			case <-e.completionWake:
			default:
			}
			select {
			case e.completionWake <- token:
			default:
			}
		}
	})
}

var projectMarkers = []string{"go.mod", "package.json", "Cargo.toml", "pyproject.toml", ".git"}

func workspaceRoot(path, lang string, roots []string) string {
	dir := filepath.Dir(path)
	// A Go workspace may sit above a module. Let gopls see the workspace.
	if lang == "go" {
		for p := dir; ; p = filepath.Dir(p) {
			if _, err := os.Stat(filepath.Join(p, "go.work")); err == nil {
				return p
			}
			if filepath.Dir(p) == p {
				break
			}
		}
	}
	marks := append(slices.Clone(roots), projectMarkers...)
	for p := dir; ; p = filepath.Dir(p) {
		for _, mark := range marks {
			if _, err := os.Stat(filepath.Join(p, mark)); err == nil {
				return p
			}
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return dir
}

func (e *editor) sessionKey(b *buffer) (key, root string, argv []string) {
	if b.path == "" {
		return "", "", nil
	}
	lang := e.cfg.languageFor(b.path)
	spec := e.cfg.Languages[lang]
	if len(spec.Server) == 0 {
		return "", "", nil
	}
	root = workspaceRoot(b.path, lang, spec.Roots)
	return lang + "\x00" + root, root, spec.Server
}

func (e *editor) session(b *buffer, create bool) *languageSession {
	key, root, argv := e.sessionKey(b)
	if key == "" {
		return nil
	}
	s := e.sessions[key]
	if s == nil && create {
		if _, err := exec.LookPath(argv[0]); err != nil {
			return nil
		}
		s = e.newLanguageSession(key, root, argv)
	}
	return s
}

func (e *editor) newLanguageSession(key, root string, argv []string) *languageSession {
	ctx, cancel := context.WithCancel(context.Background())
	s := &languageSession{root: root, argv: argv, ctx: ctx, cancel: cancel}
	s.onNotification = func(msg rpcMessage) { e.handleServerNotification(s, msg) }
	e.sessions[key] = s
	return s
}

func (e *editor) document(b *buffer) document {
	return document{path: b.path, language: e.cfg.languageFor(b.path), text: string(b.text), version: b.version, position: b.lspPosition(b.cursor), end: b.lspPosition(len(b.text))}
}
func (e *editor) didSave(b *buffer) {
	if s := e.session(b, false); s != nil {
		doc := e.document(b)
		go s.didSave(doc)
	}
}

func (e *editor) requestCompletion(trigger int, char string) {
	if e.mode != "insert" || e.menu != nil {
		return
	}
	s := e.session(e.b, true)
	if s == nil {
		if char == "" {
			e.notify("No language server for this buffer")
		}
		return
	}
	if e.completion.running {
		e.completion.queued, e.completion.queuedTrigger = true, char
		return
	}
	e.invalidateCompletion()
	token := e.completion.token
	e.completion.running = true
	doc := e.document(e.b)
	go func() {
		items, err := s.complete(doc, trigger, char)
		e.async <- func() {
			e.completion.running = false
			if e.completion.queued {
				queued := e.completion.queuedTrigger
				e.completion.queued = false
				e.requestCompletion(1, queued)
				return
			}
			if token != e.completion.token || e.mode != "insert" || e.menu != nil {
				return
			}
			if err != nil {
				if !errors.Is(err, errStale) {
					e.notify("LSP: " + err.Error())
				}
				return
			}
			e.completion.items = items
			if len(items) == 0 && char == "" {
				e.notify("No completion results")
			}
		}
	}()
}

func (e *editor) completionInput(key string) bool {
	c := &e.completion
	if len(c.items) == 0 {
		return false
	}
	switch key {
	case "down":
		c.selected = (c.selected + 1) % len(c.items)
	case "up":
		c.selected = (c.selected + len(c.items) - 1) % len(c.items)
	case "tab", "enter":
		item := c.items[c.selected]
		e.invalidateCompletion()
		if err := applyCompletion(e.b, item); err != nil {
			e.notify(err.Error())
		}
	case "esc":
		e.invalidateCompletion()
	default:
		return false
	}
	return true
}

func applyCompletion(b *buffer, item completionItem) error {
	makeEdit := func(in textEdit) (edit, error) {
		r := in.Range
		if r == nil {
			r = in.Insert
		}
		if r == nil {
			return edit{}, fmt.Errorf("completion edit has no range")
		}
		start, ok := b.offset(r.Start)
		end, valid := b.offset(r.End)
		if !ok || !valid || start > end {
			return edit{}, fmt.Errorf("invalid completion range")
		}
		return edit{span{start, end}, in.NewText}, nil
	}
	at := b.cursor
	for at > 0 && wordClass(b.text[at-1]) == 1 {
		at--
	}
	value := item.InsertText
	if value == "" {
		value = item.Label
	}
	primary := edit{span{at, b.cursor}, value}
	if item.TextEdit != nil {
		var err error
		primary, err = makeEdit(*item.TextEdit)
		if err != nil {
			return err
		}
	}
	edits := []edit{primary}
	cursor := primary.start + len([]rune(primary.text))
	for _, add := range item.Additional {
		next, err := makeEdit(add)
		if err != nil {
			return err
		}
		if next.start < primary.start {
			cursor += len([]rune(next.text)) - (next.end - next.start)
		}
		edits = append(edits, next)
	}
	slices.SortStableFunc(edits, func(a, b edit) int { return a.start - b.start })
	for i := 1; i < len(edits); i++ {
		if edits[i].start < edits[i-1].end || edits[i].start == edits[i-1].start {
			return fmt.Errorf("overlapping completion edits")
		}
	}
	b.replaceAll(edits, cursor)
	return nil
}
