package editor

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// pathPrefixAt looks for a string literal that ends at cursor and starts with
// ./, ../, / or ~/. It returns the text typed inside the quotes and the offset
// where the last path segment begins.
func pathPrefixAt(b *buffer, cursor int) (string, int, bool) {
	if cursor == 0 {
		return "", 0, false
	}
	lineStart := b.lines[b.rowAt(cursor)]
	for i := cursor - 1; i >= lineStart; i-- {
		r := b.text[i]
		if r == '"' || r == '\'' || r == '`' {
			prefix := string(b.text[i+1 : cursor])
			for _, root := range []string{"./", "../", "/", "~/"} {
				if strings.HasPrefix(prefix, root) {
					base := prefix[strings.LastIndex(prefix, "/")+1:]
					return prefix, cursor - len([]rune(base)), true
				}
			}
			return "", 0, false
		}
		if unicode.IsSpace(r) {
			return "", 0, false
		}
	}
	return "", 0, false
}

// pathItems lists the entries of the directory a typed path points into.
// Directories come first, then names alphabetically.
func pathItems(filePath, prefix string) []completionItem {
	dirPart := prefix[:strings.LastIndex(prefix, "/")+1]
	base := prefix[len(dirPart):]
	dir := filepath.Join(filepath.Dir(filePath), dirPart)
	switch {
	case strings.HasPrefix(dirPart, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		dir = filepath.Join(home, dirPart[2:])
	case strings.HasPrefix(dirPart, "/"):
		dir = dirPart
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var items []completionItem
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		isDir := entry.IsDir()
		if entry.Type()&os.ModeSymlink != 0 {
			if info, err := os.Stat(filepath.Join(dir, name)); err == nil && info.IsDir() {
				isDir = true
			}
		}
		label, detail := name, "file"
		if isDir {
			label, detail = name+"/", "folder"
		}
		items = append(items, completionItem{Label: label, Detail: detail, InsertText: label})
		if len(items) == 500 {
			break
		}
	}
	slices.SortFunc(items, func(a, b completionItem) int {
		if c := cmp.Compare(b.Detail, a.Detail); c != 0 { // "folder" sorts before "file"
			return c
		}
		return cmp.Compare(strings.ToLower(a.Label), strings.ToLower(b.Label))
	})
	return items
}

// pathCompletion fills the popup with directory entries when the cursor is
// inside a path string. It returns false when the cursor is not in one.
func (e *editor) pathCompletion() bool {
	b := e.b
	if e.mode != "insert" || e.menu != nil || b.path == "" {
		return false
	}
	prefix, start, ok := pathPrefixAt(b, b.cursor)
	if !ok {
		return false
	}
	c := &e.completion
	c.token++ // drop server results that are still on their way
	c.queued = false
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.all, c.start = pathItems(b.path, prefix), start
	c.items, c.selected = filterCompletion(c.all, string(b.text[start:b.cursor])), 0
	return true
}
