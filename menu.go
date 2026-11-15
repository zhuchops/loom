package main

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
}
type menu struct {
	title, input     string
	options, matches []option
	selected         int
	submit           func(string)
	exact            func(string) bool
	suggestions      func(string) []option
	confirmation     bool
	empty            string
}

func (e *editor) openMenu(m *menu) {
	e.invalidateCompletion()
	e.menu = m
	m.filter()
}

func (m *menu) filter() {
	m.matches = m.matches[:0]
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
	}
	return 4
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
	if m.confirmation && in.key != "esc" && in.key != "enter" && in.key != "up" && in.key != "down" && in.key != "tab" && in.key != "ctrl+p" && in.key != "ctrl+n" {
		return
	}
	switch in.key {
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
	case "ctrl+u":
		m.input = ""
		m.filter()
	case "backspace":
		if m.input != "" {
			_, size := utf8.DecodeLastRuneInString(m.input)
			m.input = m.input[:len(m.input)-size]
			m.filter()
		}
	default:
		if in.paste != "" {
			m.input += strings.ReplaceAll(in.paste, "\n", " ")
			m.filter()
			return
		}
		if r, ok := inputRune(in.key); ok {
			m.input += string(r)
			m.filter()
		}
	}
}

// Commands that only make sense as key presses, not as palette actions.
var keyOnlyCommands = map[string]bool{
	"palette": true, "normal": true, "select": true, "insert": true, "append": true, "append-line": true,
	"insert-line": true, "open-below": true, "open-above": true, "newline": true, "insert-tab": true,
	"backspace": true, "delete-forward": true, "paste-after": true, "paste-before": true,
	"delete-selection": true, "change-selection": true, "yank-selection": true,
	"macro-record": true, "macro-play": true, "macro-toggle": true, "macro-last": true,
	"surround-selection": true, "surround-delete": true, "surround-replace": true,
	"inner-object": true, "around-object": true, "next-match": true, "prev-match": true,
	"select-line": true, "select-other-end": true, "visual-paste": true, "visual-paste-keep": true,
}

// Palette entries show the Ex command that does the same thing, when there is one.
var exAliases = map[string]string{
	"save": ":w", "quit": ":q", "profile-vim": ":vim", "profile-helix": ":helix",
	"directory-jump": ":cd", "relative-numbers": ":set rnu!", "close-tab": ":tabclose",
	"tab-next": ":tabnext", "tab-prev": ":tabprev", "tab-first": ":tabfirst", "tab-last": ":tablast",
	"tab-left": ":tabmove -1", "tab-right": ":tabmove +1", "split": ":vsplit", "split-below": ":split",
	"close-split": ":close", "only-pane": ":only", "split-join": ":split-join",
	"messages":    ":messages",
	"dashboard":   ":dashboard",
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
	m := &menu{title: "Command Palette", submit: e.ex, exact: e.paletteExact, suggestions: func(input string) []option {
		if n, ok := exAddress(input); ok {
			return []option{{fmt.Sprintf("Go to Line %d", n), ":" + input, func() { e.ex(input) }}}
		}
		if input == "$" {
			return []option{{"Go to Last Line", ":$", func() { e.ex(input) }}}
		}
		return nil
	}}
	for _, item := range exCommands {
		m.options = append(m.options, option{item.title, ":" + item.syntax, func() { e.ex(item.syntax) }})
	}
	m.options = append(m.options, option{"Go to Line", ":42", func() {
		e.openMenu(&menu{title: "Go to Line", submit: e.ex})
	}})
	for _, id := range sortedKeys(e.commands) {
		cmd := e.commands[id]
		if cmd.title != "" && !paletteHidden(id) {
			m.options = append(m.options, option{cmd.title, paletteHint(e.shortcut(id), exAliases[id]), func() { run(id, cmd) }})
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
		m.options = append(m.options, option{formatKey(key), title, func() { e.notify(key + " → " + id) }})
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
		m.options = append(m.options, option{label, "", func() { e.switchBuffer(b) }})
	}
	e.openMenu(m)
}

func (e *editor) openFiles() {
	m := &menu{title: "Go to File"}
	e.openMenu(m)
	go func() {
		var options []option
		err := filepath.WalkDir(e.root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path != e.root && (strings.HasPrefix(d.Name(), ".") || slices.Contains([]string{"node_modules", "vendor", "target"}, d.Name())) {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(e.root, path)
			if err != nil {
				return nil
			}
			options = append(options, option{rel, "", func() { e.open(path) }})
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
			m.options = options
			m.filter()
		}
	}()
}

func (e *editor) openSearch() {
	e.openMenu(&menu{title: "Find in Buffer", submit: func(query string) { e.query = query; e.find(false) }})
}
func (e *editor) find(reverse bool) {
	if e.query == "" {
		return
	}
	b := e.b
	text := string(b.text)
	prefix := len(string(b.text[:b.cursor]))
	var match int
	if reverse {
		match = strings.LastIndex(text[:prefix], e.query)
		if match < 0 {
			match = strings.LastIndex(text, e.query)
		}
	} else {
		from := prefix
		if b.cursor < len(b.text) {
			from += utf8.RuneLen(b.text[b.cursor])
		}
		match = strings.Index(text[from:], e.query)
		if match >= 0 {
			match += from
		} else {
			match = strings.Index(text, e.query)
		}
	}
	if match < 0 {
		e.notify(fmt.Sprintf("No match for %q", e.query))
		return
	}
	e.searchVisible = true
	b.cursor = utf8.RuneCountInString(text[:match])
	e.clampModalCursor()
}
