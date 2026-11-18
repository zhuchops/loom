package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
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
	// all is the server's list for the word starting at start. Typing more
	// letters filters all of it locally instead of asking the server again.
	all   []completionItem
	start int
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
	c.all = nil
	c.selected = 0
}

// autoTriggerAt reports whether a letter sits before the cursor. Only then may
// the popup open on its own; punctuation such as { , ( never opens it.
func autoTriggerAt(b *buffer) bool {
	return b.cursor > 0 && unicode.IsLetter(b.text[b.cursor-1])
}

// wordStartAt is where the identifier ending at cursor begins.
func wordStartAt(b *buffer, cursor int) int {
	at := cursor
	for at > 0 && wordClass(b.text[at-1]) == 1 {
		at--
	}
	return at
}

// refreshCompletion keeps the open list while the user types more letters of
// the same word. It returns false when a fresh server request is needed.
func (e *editor) refreshCompletion(key string, cursorBefore int) bool {
	c := &e.completion
	r, ok := inputRune(key)
	b := e.b
	if !ok || wordClass(r) != 1 || len(c.all) == 0 || e.menu != nil || b.cursor != cursorBefore+1 {
		return false
	}
	if c.start < 0 || c.start >= b.cursor || b.rowAt(c.start) != b.rowAt(b.cursor) {
		return false
	}
	items := filterCompletion(c.all, string(b.text[c.start:b.cursor]))
	if len(items) == 0 {
		return false
	}
	// Bumping the token drops any request still in flight for the old prefix.
	c.token++
	c.queued = false
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.items, c.selected = items, 0
	return true
}

// filterCompletion keeps the items that match query as a case-insensitive
// subsequence. Prefix matches rank first; ties keep the server's order.
func filterCompletion(all []completionItem, query string) []completionItem {
	type scored struct {
		item  completionItem
		score int
	}
	q := []rune(strings.ToLower(query))
	var out []scored
	for _, item := range all {
		text := item.FilterText
		if text == "" {
			text = item.Label
		}
		if score, ok := fuzzyScore(q, []rune(strings.ToLower(text))); ok {
			out = append(out, scored{item, score})
		}
	}
	slices.SortStableFunc(out, func(a, b scored) int { return b.score - a.score })
	items := make([]completionItem, len(out))
	for i, s := range out {
		items[i] = s.item
	}
	return items
}

func fuzzyScore(q, text []rune) (int, bool) {
	if len(q) == 0 {
		return 0, true // keep the source order when nothing is typed yet
	}
	if len(q) <= len(text) && slices.Equal(q, text[:len(q)]) {
		return 1000 - len(text), true
	}
	score, at := 0, 0
	for _, r := range q {
		i := slices.Index(text[at:], r)
		if i < 0 {
			return 0, false
		}
		if i == 0 && at > 0 {
			score += 5 // consecutive characters
		}
		score += 10 - min(i, 9)
		at += i + 1
	}
	return score, true
}

func (e *editor) scheduleCompletion(key string) {
	c := &e.completion
	if _, ok := inputRune(key); !ok && key != "backspace" {
		return
	}
	if e.b.path == "" || !autoTriggerAt(e.b) {
		return
	}
	c.trigger = string(e.b.text[e.b.cursor-1])
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
	// char is set only for automatic requests, which must follow a letter.
	if char != "" && !autoTriggerAt(e.b) {
		return
	}
	if char == "" && e.pathCompletion() {
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
	cursor := e.b.cursor
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
			if token != e.completion.token || e.mode != "insert" || e.menu != nil || e.b.cursor != cursor || (char != "" && !autoTriggerAt(e.b)) {
				return
			}
			if err != nil {
				if !errors.Is(err, errStale) {
					e.notify("LSP: " + err.Error())
				}
				return
			}
			start := wordStartAt(e.b, cursor)
			e.completion.all, e.completion.start = items, start
			e.completion.items = filterCompletion(items, string(e.b.text[start:cursor]))
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
		item, at := c.items[c.selected], c.start
		if at < 0 || at > e.b.cursor || e.b.rowAt(at) != e.b.rowAt(e.b.cursor) {
			at = wordStartAt(e.b, e.b.cursor)
		}
		e.invalidateCompletion()
		if err := applyCompletionAt(e.b, item, at); err != nil {
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
	return applyCompletionAt(b, item, wordStartAt(b, b.cursor))
}

// applyCompletionAt replaces the text from at to the cursor with the item.
func applyCompletionAt(b *buffer, item completionItem, at int) error {
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
	value := item.InsertText
	if value == "" {
		value = item.Label
	}
	primary := edit{span{at, b.cursor}, value}
	if item.TextEdit != nil {
		// Like VS Code, accepting replaces from the word start to the cursor,
		// even when the list was filtered after the server computed its range.
		r := item.TextEdit.Range
		if r == nil {
			r = item.TextEdit.Insert
		}
		if r == nil {
			return fmt.Errorf("completion edit has no range")
		}
		start, ok := b.offset(r.Start)
		if !ok || start > b.cursor {
			start = at
		}
		primary = edit{span{start, b.cursor}, item.TextEdit.NewText}
	}
	callable := (item.Kind == 2 || item.Kind == 3 || item.Kind == 4) && !strings.Contains(primary.text, "(")
	if callable && primary.end < len(b.text) && b.text[primary.end] == '(' {
		callable = false
	}
	if callable {
		primary.text += "()"
	}
	edits := []edit{primary}
	cursor := primary.start + len([]rune(primary.text))
	if callable {
		cursor--
	}
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
	if callable {
		// Typing the closer afterwards steps over it, like an auto-paired one.
		if b.pairs == nil {
			b.pairs = make(map[int]int)
		}
		b.pairs[cursor] = cursor - 1
	}
	return nil
}
