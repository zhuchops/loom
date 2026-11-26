package editor

import (
	"cmp"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

type option struct {
	label, hint string
	run         func()
	preview     func() itemPreview
}
type menu struct {
	title, input                string
	options, matches            []option
	selected                    int
	submit                      func(string)
	exact                       func(string) bool
	suggestions                 func(string) []option
	confirmation                bool
	remove                      map[string]func() // what Ctrl+X does, by the option's hint
	window                      map[string]func() // what Ctrl+O does: open it in a new window
	hidePreview                 bool
	details                     bool // Hover selects an entry; Enter opens its full text.
	hovering                    bool
	empty                       string
	previewIndex, previewScroll int
	previewReady                bool
	preview                     itemPreview
	field                       lineField // caret and selection in input
}

// caret is the rune index of the caret in the input.
func (m *menu) caret() int { return m.field.caret(m.input) }

// inputParts splits the input at the caret.
func (m *menu) inputParts() (before, after string) {
	r := []rune(m.input)
	at := m.caret()
	return string(r[:at]), string(r[at:])
}

// wordBefore and wordAfter are where Alt+Left and Alt+Right put the caret.
func wordBefore(r []rune, at int) int {
	at = min(at, len(r))
	for at > 0 && !isWordRune(r[at-1]) {
		at--
	}
	for at > 0 && isWordRune(r[at-1]) {
		at--
	}
	return at
}

func wordAfter(r []rune, at int) int {
	at = max(at, 0)
	for at < len(r) && !isWordRune(r[at]) {
		at++
	}
	for at < len(r) && isWordRune(r[at]) {
		at++
	}
	return at
}

// moveCaret puts the caret at rune index at.
func (m *menu) moveCaret(at int) { m.field.place(m.input, at, false) }

// caretInput edits the input around the caret. It reports whether the key
// was one of its own.
func (m *menu) caretInput(in keyInput) bool {
	handled, changed := m.field.edit(&m.input, in)
	if changed {
		m.filter()
	}
	return handled
}

func (e *editor) openMenu(m *menu) {
	e.invalidateCompletion()
	e.menu = m
	m.filter()
}

func (m *menu) filter() {
	m.matches = m.matches[:0]
	m.previewReady, m.previewScroll = false, 0
	query := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(m.input), ":")))
	if m.suggestions != nil {
		m.matches = append(m.matches, m.suggestions(query)...)
	}
	type ranked struct {
		option
		rank int
	}
	var found []ranked
	for _, o := range m.options {
		if fuzzyMatch(strings.ToLower(o.label+" "+o.hint), query) {
			found = append(found, ranked{o, matchRank(o, query)})
		}
	}
	slices.SortStableFunc(found, func(a, b ranked) int { return cmp.Compare(a.rank, b.rank) })
	for _, r := range found {
		m.matches = append(m.matches, r.option)
	}
	m.selected = 0
}

func (m *menu) selectedPreview() itemPreview {
	if len(m.matches) == 0 {
		return itemPreview{message: m.input}
	}
	if !m.previewReady || m.previewIndex != m.selected {
		o := m.matches[m.selected]
		m.preview = itemPreview{title: o.label, message: o.hint}
		if o.preview != nil {
			m.preview = o.preview()
		}
		m.previewIndex, m.previewScroll, m.previewReady = m.selected, 0, true
	}
	return m.preview
}

// matchRank orders results: exact command names and labels first, then label prefixes,
// word starts, substrings, and loose fuzzy matches. Lower ranks come first.
func matchRank(o option, query string) int {
	if query == "" {
		return 0
	}
	for _, token := range strings.Fields(strings.ToLower(o.hint)) {
		if name, ok := strings.CutPrefix(token, ":"); ok && name == query {
			return 0
		}
	}
	label := strings.ToLower(o.label)
	switch {
	case label == query:
		return 0
	case strings.HasPrefix(label, query):
		return 1
	case slices.ContainsFunc(strings.Fields(label), func(word string) bool { return strings.HasPrefix(word, query) }):
		return 2
	case strings.Contains(label, query):
		return 3
	case fuzzyMatch(label, query):
		return 4
	}
	return 5 // only the hint matches
}
func fuzzyMatch(text, query string) bool {
	for _, r := range query {
		at := strings.IndexRune(text, r)
		if at < 0 {
			return false
		}
		text = text[at+utf8.RuneLen(r):]
	}
	return true
}

func (e *editor) menuInput(in keyInput) {
	m := e.menu
	m.hovering = false
	if m.confirmation && in.key != "esc" && in.key != "enter" && in.key != "up" && in.key != "down" && in.key != "tab" && in.key != "ctrl+p" && in.key != "ctrl+n" && in.key != "pageup" && in.key != "pagedown" {
		return
	}
	switch in.key {
	case "pageup":
		m.previewScroll -= 4
	case "pagedown":
		m.previewScroll += 4
	case "esc":
		e.menu = nil
	case "up", "ctrl+p":
		m.selected = max(0, m.selected-1)
	case "down", "ctrl+n", "tab":
		m.selected = max(0, min(len(m.matches)-1, m.selected+1))
	case "enter":
		if m.submit == nil && len(m.matches) > 0 && m.matches[m.selected].run == nil {
			return
		}
		e.menu = nil
		if m.submit != nil && (m.exact == nil || m.selected == 0 && m.exact(m.input)) {
			m.submit(m.input)
		} else if len(m.matches) > 0 {
			m.matches[m.selected].run()
		} else if m.submit != nil {
			m.submit(m.input)
		}
	case "ctrl+x":
		if m.remove != nil {
			e.removeSelected()
		}
	case "ctrl+o":
		if m.window != nil {
			e.windowSelected()
		}
	default:
		m.caretInput(in)
	}
}

// Commands that only make sense as key presses, not as palette actions.
var keyOnlyCommands = map[string]bool{
	"palette": true, "normal": true, "select": true, "insert": true, "append": true, "append-line": true,
	"insert-line": true, "open-below": true, "open-above": true, "newline": true, "insert-tab": true,
	"backspace": true, "delete-forward": true, "paste-after": true, "paste-before": true,
	"delete-selection": true, "change-selection": true, "yank-selection": true,
	"indent-selection": true, "outdent-selection": true,
	"macro-record": true, "macro-play": true, "macro-toggle": true, "macro-last": true,
	"surround-selection": true, "surround-delete": true, "surround-replace": true,
	"inner-object": true, "around-object": true, "next-match": true, "prev-match": true,
	"select-line": true, "select-other-end": true, "visual-paste": true, "visual-paste-keep": true,
}

// Palette entries show the Ex command that does the same thing, when there is one.
var exAliases = map[string]string{
	"save": ":w", "quit": ":q", "profile-vim": ":vim",
	"directory-jump": ":cd", "relative-numbers": ":set rnu!", "close-tab": ":tabclose",
	"tab-next": ":tabnext", "tab-prev": ":tabprev", "tab-first": ":tabfirst", "tab-last": ":tablast",
	"tab-left": ":tabmove -1", "tab-right": ":tabmove +1", "split": ":vsplit", "split-below": ":split",
	"close-split": ":close", "only-pane": ":only", "split-join": ":split-join",
	"messages":    ":messages",
	"dashboard":   ":dashboard",
	"sessions":    ":sessions",
	"lsp-restart": ":lsp-restart",
}

func paletteHidden(id string) bool {
	if keyOnlyCommands[id] {
		return true
	}
	n, numbered := strings.CutPrefix(id, "tab-")
	_, err := strconv.Atoi(n)
	return numbered && err == nil
}

// formatKey turns a stored binding such as "ctrl+w v" into "Ctrl+W v".
func formatKey(sequence string) string {
	parts := strings.Fields(sequence)
	for i, part := range parts {
		words := strings.Split(part, "+")
		for j, word := range words {
			switch {
			case utf8.RuneCountInString(word) == 1 && j > 0:
				words[j] = strings.ToUpper(word)
			case utf8.RuneCountInString(word) > 1:
				words[j] = strings.ToUpper(word[:1]) + word[1:]
			}
		}
		parts[i] = strings.Join(words, "+")
	}
	return strings.Join(parts, " ")
}

// shortcut returns the shortest key bound to a command in the active profile.
func (e *editor) shortcut(id string) string {
	best := ""
	for scope, keys := range e.keys {
		if !strings.HasPrefix(scope, e.cfg.Profile+".") {
			continue
		}
		for sequence, bound := range keys {
			if bound == id && (best == "" || len(sequence) < len(best) || len(sequence) == len(best) && sequence < best) {
				best = sequence
			}
		}
	}
	return formatKey(best)
}

func paletteHint(parts ...string) string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, "  ")
}

func (e *editor) openPalette() {
	run := func(id string, cmd command) {
		if e.directory != nil && directoryTextCommand(id) {
			cmd.run(e.directory.doc)
			return
		}
		e.runCommand(id)
	}
	m := &menu{title: "Command Palette", hidePreview: true, submit: e.ex, exact: e.paletteExact, suggestions: func(input string) []option {
		if n, ok := exAddress(input); ok {
			return []option{{fmt.Sprintf("Go to Line %d", n), ":" + input, func() { e.ex(input) }, nil}}
		}
		if input == "$" {
			return []option{{"Go to Last Line", ":$", func() { e.ex(input) }, nil}}
		}
		return nil
	}}
	if e.mode == "select" && e.b.anchor >= 0 {
		e.b.markSelection()
		m.input = "'<,'>"
	}
	for _, item := range exCommands {
		m.options = append(m.options, option{item.title, ":" + item.syntax, func() { e.ex(item.syntax) }, nil})
	}
	m.options = append(m.options, option{"Go to Line", ":42", func() {
		e.openMenu(&menu{title: "Go to Line", hidePreview: true, submit: e.ex})
	}, nil})
	for _, id := range sortedKeys(e.commands) {
		cmd := e.commands[id]
		if cmd.title != "" && !paletteHidden(id) {
			m.options = append(m.options, option{cmd.title, paletteHint(e.shortcut(id), exAliases[id]), func() { run(id, cmd) }, nil})
		}
	}
	slices.SortStableFunc(m.options, func(a, b option) int {
		return strings.Compare(strings.ToLower(a.label), strings.ToLower(b.label))
	})
	e.openMenu(m)
}

func (e *editor) showKeys() {
	m := &menu{title: "Keybindings · " + e.cfg.Profile + " / " + e.mode}
	keys := e.keys[e.cfg.Profile+"."+e.mode]
	for _, key := range sortedKeys(keys) {
		id := keys[key]
		title := e.commands[id].title
		if title == "" {
			title = humanize(id)
		}
		m.options = append(m.options, option{formatKey(key), title, func() { e.notify(key + " → " + id) }, nil})
	}
	e.openMenu(m)
}

func (e *editor) openBuffers() {
	m := &menu{title: "Open Buffers"}
	for _, b := range e.buffers {
		label := b.path
		if b.dashboard {
			label = "Dashboard"
		} else if label == "" {
			label = "[untitled]"
		}
		if b.dirty() {
			label += " *"
		}
		m.options = append(m.options, option{label, "", func() { e.switchBuffer(b) }, func() itemPreview {
			if b.dashboard {
				return itemPreview{title: label, message: "Project start page"}
			}
			return bufferPreview(b, label, -1, 0, false, nil)
		}})
	}
	e.openMenu(m)
}

func (e *editor) openFiles() {
	m := &menu{title: "Go to File"}
	e.openMenu(m)
	root := e.root
	go func() {
		var options []option
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path != root && (strings.HasPrefix(d.Name(), ".") || slices.Contains([]string{"node_modules", "vendor", "target"}, d.Name())) {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			options = append(options, option{rel, "", func() { e.openPreview(path) }, e.filePreview(path, -1, 0, false, nil)})
			if len(options) >= 20000 {
				return fs.SkipAll
			}
			return nil
		})
		e.async <- func() {
			if e.menu != m {
				return
			}
			if err != nil {
				e.notify(err.Error())
			}
			m.options = append(e.recentOptions(true), options...)
			m.filter()
		}
	}()
}
