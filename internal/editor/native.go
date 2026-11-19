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
}

func (e *editor) clampCursor() {
	e.b.cursor = max(0, min(e.b.cursor, len(e.b.text)))
}

// insertTab inserts a tab, or spaces up to the next tab stop.
func (e *editor) insertTab() {
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
