package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type treeEntry struct {
	path  string
	depth int
	dir   bool
}

type fileTree struct {
	rows                                                []treeEntry
	expanded                                            map[string]bool
	selected, scroll                                    int
	sidebar, popup, focused, jump, manualScroll, reveal bool
	width                                               int
	cut                                                 string
	sortMode                                            int
	filter                                              string
	filtering                                           bool
}

var treeSorts = []string{"name", "modified", "size", "type"}

// sortTreeEntries keeps folders first, then orders by the chosen mode.
func sortTreeEntries(entries []os.DirEntry, mode int) {
	info := func(e os.DirEntry) os.FileInfo {
		i, _ := e.Info()
		return i
	}
	byName := func(a, b os.DirEntry) int {
		return strings.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name()))
	}
	slices.SortStableFunc(entries, func(a, b os.DirEntry) int {
		if a.IsDir() != b.IsDir() {
			if a.IsDir() {
				return -1
			}
			return 1
		}
		if !a.IsDir() || mode == 0 {
			switch mode {
			case 1:
				ia, ib := info(a), info(b)
				if ia != nil && ib != nil && !ia.ModTime().Equal(ib.ModTime()) {
					return ib.ModTime().Compare(ia.ModTime())
				}
			case 2:
				ia, ib := info(a), info(b)
				if ia != nil && ib != nil && ia.Size() != ib.Size() {
					return int(max(-1, min(1, ib.Size()-ia.Size())))
				}
			case 3:
				if c := strings.Compare(strings.ToLower(filepath.Ext(a.Name())), strings.ToLower(filepath.Ext(b.Name()))); c != 0 {
					return c
				}
			}
		}
		return byName(a, b)
	})
}

func treeMatches(query []rune, name string) bool {
	_, ok := fuzzyScore(query, []rune(strings.ToLower(name)))
	return ok
}

func (t *fileTree) status() string {
	text := "Explorer · " + treeSorts[t.sortMode]
	if t.filtering {
		return "Explorer /" + t.filter + "▏"
	}
	if t.filter != "" {
		text += " · /" + t.filter
	}
	return text
}

// filterInput handles keys while the filter prompt is open.
func (e *editor) filterInput(key string) bool {
	t := &e.tree
	switch key {
	case "enter":
		t.filtering = false
	case "esc":
		t.filtering, t.filter = false, ""
	case "backspace":
		if r := []rune(t.filter); len(r) > 0 {
			t.filter = string(r[:len(r)-1])
		}
	case "space":
		t.filter += " "
	default:
		r, ok := inputRune(key)
		if !ok {
			return true
		}
		t.filter += string(r)
	}
	e.refreshTree("")
	if t.filter != "" {
		for i, row := range t.rows {
			if !row.dir {
				t.selected = i
				break
			}
		}
	}
	e.revealTreeSelection()
	return true
}

func (e *editor) refreshTree(target string) {
	t := &e.tree
	if t.expanded == nil {
		t.expanded = map[string]bool{}
	}
	if target == "" && len(t.rows) > 0 {
		target = t.rows[t.selected].path
	}
	t.rows = []treeEntry{{e.root, 0, true}}
	filtering := t.filter != ""
	query := []rune(strings.ToLower(t.filter))
	visited := 0
	var walk func(string, int)
	walk = func(dir string, depth int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		sortTreeEntries(entries, t.sortMode)
		for _, entry := range entries {
			name := entry.Name()
			if name == ".git" || len(t.rows) >= 10000 || visited >= 100000 {
				continue
			}
			visited++
			path := filepath.Join(dir, name)
			if !entry.IsDir() {
				if !filtering || treeMatches(query, name) {
					t.rows = append(t.rows, treeEntry{path, depth, false})
				}
				continue
			}
			if filtering && name == "node_modules" {
				continue
			}
			mark := len(t.rows)
			t.rows = append(t.rows, treeEntry{path, depth, true})
			if filtering {
				walk(path, depth+1)
				if len(t.rows) == mark+1 && !treeMatches(query, name) {
					t.rows = t.rows[:mark]
				}
			} else if t.expanded[path] {
				walk(path, depth+1)
			}
		}
	}
	if t.expanded[e.root] || filtering {
		walk(e.root, 1)
	}
	t.selected = min(t.selected, len(t.rows)-1)
	t.reveal = true
	for i, row := range t.rows {
		if row.path == target {
			t.selected = i
			break
		}
	}
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (e *editor) revealFile() {
	if e.b.path == "" || !within(e.root, e.b.path) {
		return
	}
	for p := filepath.Dir(e.b.path); within(e.root, p); p = filepath.Dir(p) {
		e.tree.expanded[p] = true
		if p == e.root {
			break
		}
	}
	e.refreshTree(e.b.path)
}

func (e *editor) showTree(popup bool) {
	e.invalidateCompletion()
	e.zen = false
	e.tree.popup, e.tree.focused = popup, true
	if !popup {
		e.tree.sidebar = true
	}
	e.refreshTree("")
	e.revealFile()
	e.revealTreeSelection()
}

func (e *editor) treeInput(key string) bool {
	t := &e.tree
	if !t.focused || len(t.rows) == 0 {
		return false
	}
	row := t.rows[t.selected]
	switch key {
	case "q":
		e.hideExplorer()
	case "esc":
		if t.filter != "" {
			t.filter = ""
			e.refreshTree(row.path)
			break
		}
		t.popup, t.focused, t.jump = false, false, false
	case "/":
		t.filtering = true
		return true
	case "s":
		t.sortMode = (t.sortMode + 1) % len(treeSorts)
		e.refreshTree(row.path)
		e.notify("Explorer sorted by " + treeSorts[t.sortMode])
	case "j", "down":
		t.selected = min(len(t.rows)-1, t.selected+1)
	case "k", "up":
		t.selected = max(0, t.selected-1)
	case "g":
		if t.jump {
			t.selected = 0
			e.revealTreeSelection()
		}
		t.jump = !t.jump
		return true
	case "G":
		t.selected = len(t.rows) - 1
	case "ctrl+d", "pagedown":
		t.selected = min(len(t.rows)-1, t.selected+12)
	case "ctrl+u", "pageup":
		t.selected = max(0, t.selected-12)
	case "h", "left":
		if row.dir && t.expanded[row.path] {
			delete(t.expanded, row.path)
			e.refreshTree(row.path)
		} else if row.path != e.root {
			e.refreshTree(filepath.Dir(row.path))
		}
	case "l", "right", "enter", "o":
		if row.dir {
			if t.expanded[row.path] && key != "l" && key != "right" {
				delete(t.expanded, row.path)
			} else if t.expanded[row.path] && t.selected+1 < len(t.rows) && t.rows[t.selected+1].depth > row.depth {
				t.selected++
				break
			} else {
				t.expanded[row.path] = true
			}
			e.refreshTree(row.path)
		} else {
			t.focused, t.popup = false, false
			e.open(row.path)
		}
	case "a", "A":
		dir := treeDir(row)
		title := "New File"
		if key == "A" {
			title = "New Folder"
		}
		e.treePrompt(title, "", func(input string) error {
			path, err := e.treePath(dir, input)
			if err != nil {
				return err
			}
			if key == "A" {
				err = os.MkdirAll(path, 0755)
			} else {
				if err = os.MkdirAll(filepath.Dir(path), 0755); err == nil {
					var f *os.File
					f, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
					if err == nil {
						err = f.Close()
					}
				}
			}
			if err == nil {
				e.expandTo(path)
				e.refreshTree(path)
			}
			return err
		})
	case "r", "m":
		if row.path == e.root {
			break
		}
		title, initial, base := "Rename", filepath.Base(row.path), filepath.Dir(row.path)
		if key == "m" {
			title, initial, base = "Move to Project Path", "", e.root
		}
		e.treePrompt(title, initial, func(input string) error {
			to, err := e.treePath(base, input)
			if err != nil {
				return err
			}
			if info, err := os.Stat(to); key == "m" && err == nil && info.IsDir() {
				to = filepath.Join(to, filepath.Base(row.path))
			}
			return e.moveEntry(row.path, to)
		})
	case "x":
		if row.path != e.root {
			t.cut = row.path
			e.notify("Cut " + filepath.Base(row.path) + " · p to move here")
		}
	case "p":
		if t.cut != "" {
			if err := e.moveEntry(t.cut, filepath.Join(treeDir(row), filepath.Base(t.cut))); err != nil {
				e.notify(err.Error())
			} else {
				t.cut = ""
				e.refreshGit()
			}
		}
	case "d":
		if row.path == e.root {
			break
		}
		e.treePrompt("Move "+filepath.Base(row.path)+" to Trash? Type yes", "", func(input string) error {
			if input != "yes" {
				return nil
			}
			for _, b := range e.buffers {
				if b.path != "" && within(row.path, b.path) && b.dirty() {
					return fmt.Errorf("save open changes before removing this path")
				}
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			trash := filepath.Join(home, ".Trash")
			if err := os.MkdirAll(trash, 0700); err != nil {
				return err
			}
			dest := filepath.Join(trash, filepath.Base(row.path))
			if _, err := os.Lstat(dest); !os.IsNotExist(err) {
				dest += fmt.Sprintf(".%d", time.Now().UnixNano())
			}
			if err := os.Rename(row.path, dest); err != nil {
				return err
			}
			e.closePath(row.path)
			e.refreshTree(filepath.Dir(row.path))
			return nil
		})
	case "R":
		e.refreshTree(row.path)
		e.refreshGit()
	case "?":
		e.notify("j/k move · h/l collapse/open · / filter · s sort · a/A new · r rename · m move · x/p cut/paste · d trash · R refresh")
	default:
		return false
	}
	t.jump = false
	e.revealTreeSelection()
	return true
}

func treeDir(row treeEntry) string {
	if row.dir {
		return row.path
	}
	return filepath.Dir(row.path)
}

func (e *editor) expandTo(path string) {
	for dir := filepath.Dir(path); within(e.root, dir); dir = filepath.Dir(dir) {
		e.tree.expanded[dir] = true
		if dir == e.root {
			break
		}
	}
}

func (e *editor) treePrompt(title, initial string, action func(string) error) {
	e.openMenu(&menu{title: title, input: initial, submit: func(input string) {
		if input == "" {
			return
		}
		if err := action(input); err != nil {
			e.notify(err.Error())
			return
		}
		e.refreshGit()
	}})
}

func (e *editor) treePath(base, input string) (string, error) {
	path := filepath.Clean(filepath.Join(base, input))
	if filepath.IsAbs(input) || path == e.root || !within(e.root, path) {
		return "", fmt.Errorf("enter a path inside the project")
	}
	for parent := filepath.Dir(path); within(e.root, parent); parent = filepath.Dir(parent) {
		if info, err := os.Lstat(parent); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("directory is a symbolic link")
		}
		if parent == e.root {
			break
		}
	}
	return path, nil
}

func (e *editor) moveEntry(from, to string) error {
	if from == to {
		return nil
	}
	if from == e.root || !within(e.root, from) || !within(e.root, to) {
		return fmt.Errorf("move must stay inside the project")
	}
	for _, b := range e.buffers {
		if b.path != "" && within(to, b.path) && !within(from, b.path) {
			return fmt.Errorf("destination is already open in another tab")
		}
	}
	if within(from, to) {
		return fmt.Errorf("cannot move a folder inside itself")
	}
	if _, err := os.Lstat(to); !os.IsNotExist(err) {
		return fmt.Errorf("destination already exists or is inaccessible")
	}
	if err := os.Rename(from, to); err != nil {
		return err
	}
	for _, b := range e.buffers {
		if b.path != "" && within(from, b.path) {
			oldPath := b.path
			rel, _ := filepath.Rel(from, oldPath)
			b.path = filepath.Join(to, rel)
			b.version++
			b.diagnostics = nil
			e.closeDocument(oldPath)
		}
	}
	var moved []string
	for p := range e.tree.expanded {
		if within(from, p) {
			rel, _ := filepath.Rel(from, p)
			moved = append(moved, filepath.Join(to, rel))
			delete(e.tree.expanded, p)
		}
	}
	for _, p := range moved {
		e.tree.expanded[p] = true
	}
	e.expandTo(to)
	e.refreshTree(to)
	e.scheduleLanguage()
	return nil
}

func (e *editor) revealTreeSelection() { e.tree.reveal = true; e.tree.manualScroll = false }
