package editor

import (
	"os"
	"path/filepath"
	"strings"
)

// tabWidth is how wide a tab character is drawn. It follows config tab_size.
var tabWidth = 4

// pageRows is the height of the editor pane, for PageUp and PageDown.
func (e *editor) pageRows() int {
	if e.viewRows > 2 {
		return e.viewRows - 1
	}
	return 20
}

func (e *editor) registerNativeCommands() {
	e.registerMarkCommands()
	e.registerVimExtras()
	e.add("recent-files", "Recent Files", func(e *editor) { e.recentMenu() })
	e.add("jumps", "Jump List", func(e *editor) { e.jumpsMenu() })
	e.add("project-replace", "Replace in Project", func(e *editor) { e.projectReplacePrompt() })
	e.registerRegisterCommands()
	e.registerBlockCommands()
	page := func(name string, dir string) {
		e.add(name, "", func(e *editor) {
			b := e.b
			e.takeCount()
			for range e.pageRows() {
				next := motion(b, dir, b.cursor)
				if next == b.cursor {
					break
				}
				b.cursor = next
			}
			if e.mode != "insert" {
				e.clampModalCursor()
			}
		})
	}
	page("page-down", "down")
	page("page-up", "up")
	e.add("select-all", "Select All", func(e *editor) {
		b := e.b
		if len(b.text) == 0 {
			return
		}
		e.mode, b.anchor, b.visualLine = "select", 0, false
		b.cursor = len(b.text) - 1
	})
	e.add("undo-here", "Undo", func(e *editor) {
		e.b.history(false)
		e.clampCursor()
	})
	e.add("redo-here", "Redo", func(e *editor) {
		e.b.history(true)
		e.clampCursor()
	})
	e.add("insert-home", "", func(e *editor) {
		b := e.b
		row := b.rowAt(b.cursor)
		first := b.lines[row]
		for first < b.lineEnd(row) && (b.text[first] == ' ' || b.text[first] == '\t') {
			first++
		}
		// Like VS Code, the first press goes to the text, the second to column 0.
		if b.cursor == first {
			first = b.lines[row]
		}
		b.cursor = first
	})
	e.add("insert-end", "", func(e *editor) { e.b.cursor = e.b.lineEnd(e.b.rowAt(e.b.cursor)) })
	e.add("insert-word-left", "", func(e *editor) { e.b.cursor = motion(e.b, "back-word", e.b.cursor) })
	e.add("insert-word-right", "", func(e *editor) {
		b := e.b
		next := motion(b, "word", b.cursor)
		b.cursor = min(next, len(b.text))
	})
	e.add("insert-outdent", "Outdent Line", func(e *editor) { e.outdentLineAtCursor() })
	e.add("open-config", "Open Config File", func(e *editor) { e.openConfigFile() })
	e.add("reload-config", "Reload Config", func(e *editor) { e.reloadConfig() })
	e.add("settings", "Settings", func(e *editor) { e.settingsMenu(0) })
	e.add("move-line-up", "Move Line Up", func(e *editor) { e.moveLines(-1) })
	e.add("move-line-down", "Move Line Down", func(e *editor) { e.moveLines(1) })
	e.add("delete-word-back", "Delete Word Backward", func(e *editor) {
		b := e.b
		if b.cursor == 0 {
			return
		}
		start := motion(b, "back-word", b.cursor)
		if start == b.cursor {
			start = b.cursor - 1
		}
		b.replace(span{start, b.cursor}, "")
		b.cursor = start
	})
}

func (e *editor) clampCursor() {
	e.b.cursor = max(0, min(e.b.cursor, len(e.b.text)))
}

// insertTab inserts a tab, or spaces up to the next tab stop.
func (e *editor) insertTab() {
	e.useTabWidth(e.b)
	if e.indentUnit("") == "\t" {
		e.insert("\t")
		return
	}
	b := e.b
	column := b.cursor - b.lines[b.rowAt(b.cursor)]
	e.insert(strings.Repeat(" ", tabWidth-column%tabWidth))
}

func (e *editor) outdentLineAtCursor() {
	b := e.b
	row := b.rowAt(b.cursor)
	start := b.lines[row]
	n := 0
	switch {
	case start < len(b.text) && b.text[start] == '\t':
		n = 1
	default:
		for n < tabWidth && start+n < b.lineEnd(row) && b.text[start+n] == ' ' {
			n++
		}
	}
	if n == 0 {
		return
	}
	cursor := max(start, b.cursor-n)
	b.replace(span{start, start + n}, "")
	b.cursor = cursor
}

// openConfigFile opens the config, creating a minimal one when none exists.
func (e *editor) openConfigFile() {
	path := e.cfg.path
	if path == "" {
		path = configPath()
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
			err = os.WriteFile(path, []byte("{\n  \"tab_size\": 4\n}\n"), 0o644)
		}
	}
	e.open(path)
}

// moveLines swaps the cursor's line, or every selected line, with its neighbour.
func (e *editor) moveLines(dir int) {
	b := e.b
	if b.tags != nil || len(b.lines) < 2 {
		return
	}
	first, last := b.rowAt(b.cursor), b.rowAt(b.cursor)
	if b.anchor >= 0 {
		first, last = b.rowAt(min(b.anchor, b.cursor)), b.rowAt(max(b.anchor, b.cursor))
	}
	if dir < 0 && first == 0 || dir > 0 && last == len(b.lines)-1 {
		return
	}
	nl := b.newline()
	lineText := func(r int) string { return string(b.text[b.lines[r]:b.lineEnd(r)]) }
	var block []string
	for r := first; r <= last; r++ {
		block = append(block, lineText(r))
	}
	neighbour := first - 1
	if dir > 0 {
		neighbour = last + 1
	}
	other := lineText(neighbour)
	from, to := b.lines[min(first, neighbour)], b.lineEnd(max(last, neighbour))
	joined := strings.Join(block, nl)
	var out string
	if dir < 0 {
		out = joined + nl + other
	} else {
		out = other + nl + joined
	}
	shift := len([]rune(other)) + len([]rune(nl))
	if dir < 0 {
		shift = -shift
	}
	cursor, anchor := b.cursor+shift, b.anchor
	if anchor >= 0 {
		anchor += shift
	}
	b.replace(span{from, to}, out)
	b.cursor, b.anchor = cursor, anchor
}
