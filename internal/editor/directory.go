package editor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

type directoryEntry struct {
	name string
	info os.FileInfo
}

// The child editor runs the standard modal engine on the directory listing.
// The outer editor owns filesystem operations, menus, and the project root.
type directoryEditor struct {
	dir     string
	doc     *editor
	entries []directoryEntry
}

func (e *editor) openDirectory(dir string) {
	if e.directory != nil && e.directory.doc.b.dirty() {
		e.notify("Write the directory buffer or use :q! to discard its edits")
		return
	}
	if dir == "" {
		dir = e.root
		if e.b.path != "" {
			dir = filepath.Dir(e.b.path)
		}
	}
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		e.notify(err.Error())
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		e.notify(err.Error())
		return
	}
	slices.SortStableFunc(entries, func(a, b os.DirEntry) int {
		if a.IsDir() != b.IsDir() {
			if a.IsDir() {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name()))
	})
	d := &directoryEditor{dir: dir}
	b, _ := loadBuffer("")
	// The parent row lives in the text so normal motions can reach it.
	// Its reserved tag lets planning distinguish it from filesystem entries.
	b.text = []rune("../\n")
	b.tags = []int{0, 0, 0, -1}
	for _, entry := range entries {
		if entry.Name() == ".git" || !utf8.ValidString(entry.Name()) || strings.ContainsAny(entry.Name(), "\r\n\t") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		name := entry.Name()
		if info.IsDir() {
			name += "/"
		}
		d.entries = append(d.entries, directoryEntry{name, info})
		b.text = append(b.text, []rune(name)...)
		b.tags = append(b.tags, make([]int, len([]rune(name)))...)
		b.text = append(b.text, '\n')
		b.tags = append(b.tags, len(d.entries))
	}
	b.indexLines()
	b.saved = string(b.text)
	d.doc = &editor{b: b, mode: "normal", cfg: e.cfg, commands: e.commands, keys: e.keys, macros: map[string][]keyInput{}, notificationParent: e}
	d.doc.cfg.AutoComplete = false
	d.doc.cfg.AutoPairs = false
	d.doc.relativeNumbers = e.relativeNumbers
	e.directory = d
	e.tree.focused = false
	e.invalidateCompletion()
	e.message = ""
}

func (e *editor) directoryInput(in keyInput) {
	d := e.directory
	doc := d.doc
	key := in.key
	if doc.pending.kind == "" && doc.prefix == "" && doc.mode != "insert" {
		switch key {
		case ":":
			e.openPalette()
			return
		case "enter":
			e.directoryOpenEntry()
			return
		case "-":
			if parent := filepath.Dir(d.dir); parent != d.dir {
				e.openDirectory(parent)
			}
			return
		case "q", "esc":
			if key == "q" || doc.mode == "normal" {
				e.closeDirectory(false)
				return
			}
		}
	}
	if key == "ctrl+s" {
		e.writeDirectory()
		return
	}
	sequence := key
	if doc.prefix != "" {
		sequence = doc.prefix + " " + key
	}
	id := doc.keys[doc.cfg.Profile+"."+doc.mode][sequence]
	switch id {
	case "palette", "ex":
		doc.prefix = ""
		e.openPalette()
		return
	case "directory-jump", "messages":
		doc.prefix = ""
		e.commands[id].run(e)
		return
	case "close-explorer":
		doc.prefix = ""
		e.hideExplorer()
		return
	case "relative-numbers":
		doc.prefix = ""
		e.commands[id].run(e)
		doc.relativeNumbers = e.relativeNumbers
		return
	case "explorer-popup", "explorer":
		doc.prefix = ""
		e.closeDirectory(false)
		return
	case "project-search":
		doc.prefix = ""
		if !doc.b.dirty() {
			e.closeDirectory(false)
			e.commands[id].run(e)
		} else {
			e.notify("Write directory changes before searching the project")
		}
		return
	case "save":
		doc.prefix = ""
		e.writeDirectory()
		return
	case "files", "buffers":
		doc.prefix = ""
		if !doc.b.dirty() {
			e.closeDirectory(false)
			e.commands[id].run(e)
		} else {
			e.notify("Write directory changes before switching files")
		}
		return
	case "search":
		e.openMenu(&menu{title: "Find in Directory", submit: func(q string) { doc.query = q; doc.find(false) }})
		return
	}
	// This surface uses the same motions and text operations with workspace commands scoped to the outer editor.
	if id != "" && !directoryTextCommand(id) {
		doc.prefix = ""
		return
	}
	doc.handle(in)
}

func directoryTextCommand(id string) bool {
	switch id {
	case "normal", "select", "select-line", "select-other-end", "insert", "append", "append-line", "insert-line", "open-below", "open-above", "newline", "insert-tab",
		"backspace", "delete-forward", "undo", "redo", "paste-after", "paste-before", "repeat",
		"surround-selection", "surround-delete", "surround-replace", "inner-object", "around-object",
		"mini-add-motion", "mini-add-selection", "mini-delete", "mini-replace", "mini-find", "mini-find-back", "mini-highlight",
		"delete-op", "change-op", "yank-op", "delete-selection", "change-selection", "yank-selection",
		"delete-to-end", "change-to-end", "yank-line", "replace-character", "match-bracket",
		"find-char-f", "find-char-F", "find-char-t", "find-char-T", "find-char-next", "find-char-prev",
		"left", "right", "up", "down", "home", "first-nonblank", "end", "top", "bottom", "word", "back-word", "word-end",
		"insert-left", "insert-right", "insert-up", "insert-down", "next-match", "prev-match":
		return true
	}
	return false
}

func (e *editor) closeDirectory(force bool) {
	if !force && e.directory.doc.b.dirty() {
		e.notify("Unwritten directory edits · :w to apply or :q! to discard")
		return
	}
	e.directory = nil
	e.message = ""
}

func (e *editor) directoryOpenEntry() {
	d := e.directory
	b := d.doc.b
	row := b.rowAt(b.cursor)
	line := string(b.text[b.lines[row]:b.lineEnd(row)])
	if line == "" {
		return
	}
	if b.dirty() {
		e.notify("Write directory edits before opening an entry")
		return
	}
	if line == "../" {
		e.openDirectory(filepath.Dir(d.dir))
		return
	}
	path := filepath.Join(d.dir, strings.TrimSuffix(line, "/"))
	info, err := os.Stat(path)
	if err != nil {
		e.notify(err.Error())
		return
	}
	if info.IsDir() {
		e.openDirectory(path)
		return
	}
	if e.open(path) {
		e.directory = nil
	}
}

type directoryChange struct {
	from, to string
	folder   bool
	info     os.FileInfo
}

// Build and validate the complete edit plan before touching any files. A
// positive newline tag is a stable entry identity, even after a name changes.
func (e *editor) directoryPlan() ([]directoryChange, error) {
	d := e.directory
	b := d.doc.b
	seen := map[int]bool{}
	destinations := map[string]bool{}
	var changes []directoryChange
	for row, start := range b.lines {
		end := b.lineEnd(row)
		line := string(b.text[start:end])
		if line == "" {
			if end < len(b.tags) && b.tags[end] != 0 {
				return nil, fmt.Errorf("line %d: delete the entire row to remove an entry", row+1)
			}
			continue
		}
		if strings.ContainsAny(line, "\t\r") {
			return nil, fmt.Errorf("line %d: tabs and carriage returns are unsupported", row+1)
		}
		id := 0
		if end < len(b.tags) {
			id = b.tags[end]
		}
		if line == "../" && id <= 0 {
			continue
		}
		if id == -1 {
			if line != "../" {
				return nil, fmt.Errorf("the parent row must remain ../")
			}
			continue
		}
		folder := strings.HasSuffix(line, "/")
		to, err := e.directoryDestination(d.dir, strings.TrimSuffix(line, "/"))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", row+1, err)
		}
		if destinations[to] {
			return nil, fmt.Errorf("duplicate destination: %s", line)
		}
		destinations[to] = true
		change := directoryChange{to: to, folder: folder}
		if id != 0 {
			if seen[id] {
				return nil, fmt.Errorf("entry appears twice: %s", line)
			}
			seen[id] = true
			entry := d.entries[id-1]
			change.from = filepath.Join(d.dir, entry.name)
			change.info = entry.info
			if folder != entry.info.IsDir() {
				return nil, fmt.Errorf("keep the trailing slash on directories: %s", entry.name)
			}
			if change.from == change.to {
				continue
			}
		}
		changes = append(changes, change)
	}
	for i, entry := range d.entries {
		if !seen[i+1] {
			changes = append(changes, directoryChange{from: filepath.Join(d.dir, entry.name), folder: entry.info.IsDir(), info: entry.info})
		}
	}
	return changes, e.checkDirectoryPlan(changes)
}

func (e *editor) checkDirectoryPlan(changes []directoryChange) error {
	sources := map[string]os.FileInfo{}
	for _, c := range changes {
		if c.from == "" {
			continue
		}
		info, err := os.Lstat(c.from)
		if err != nil || !os.SameFile(c.info, info) {
			return fmt.Errorf("entry changed on disk: %s", c.from)
		}
		sources[c.from] = info
		if c.to == "" {
			for _, b := range e.buffers {
				if b.path != "" && within(c.from, b.path) && b.dirty() {
					return fmt.Errorf("save changes to %s before removing it", b.path)
				}
			}
		}
	}
	for _, c := range changes {
		if c.to == "" {
			continue
		}
		rel, _ := filepath.Rel(filepath.VolumeName(c.to)+string(filepath.Separator), c.to)
		for _, part := range strings.Split(rel, string(filepath.Separator)) {
			if part == ".git" {
				return fmt.Errorf("cannot edit Git metadata")
			}
		}
		if info, err := os.Stat(filepath.Dir(c.to)); err != nil || !info.IsDir() {
			return fmt.Errorf("destination folder must exist: %s", filepath.Dir(c.to))
		}
		for source := range sources {
			if source != c.to && within(source, c.to) {
				return fmt.Errorf("destination is inside a directory being moved or removed: %s", c.to)
			}
		}
		for _, b := range e.buffers {
			if b.path != "" && within(c.to, b.path) && !within(c.from, b.path) {
				moving := false
				for source := range sources {
					if within(source, b.path) {
						moving = true
						break
					}
				}
				if !moving {
					return fmt.Errorf("destination is already open: %s", b.path)
				}
			}
		}
		info, err := os.Lstat(c.to)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if sources[c.to] != nil || c.info != nil && os.SameFile(info, c.info) && strings.EqualFold(c.from, c.to) {
			continue
		}
		return fmt.Errorf("destination already exists: %s", c.to)
	}
	for _, parent := range changes {
		if parent.to == "" {
			continue
		}
		for _, child := range changes {
			if child.to != "" && child.to != parent.to && within(parent.to, child.to) {
				return fmt.Errorf("apply nested destinations in separate writes: %s", child.to)
			}
		}
	}
	return nil
}

func (e *editor) writeDirectory() {
	changes, err := e.directoryPlan()
	if err != nil {
		e.notify(err.Error())
		return
	}
	if len(changes) == 0 {
		e.directory.doc.b.saved = string(e.directory.doc.b.text)
		e.notify("No directory changes")
		return
	}
	m := &menu{title: fmt.Sprintf("Apply %d Directory Changes", len(changes)), confirmation: true}
	for _, c := range changes {
		from, _ := filepath.Rel(e.root, c.from)
		to, _ := filepath.Rel(e.root, c.to)
		switch {
		case c.from == "":
			m.options = append(m.options, option{"Create " + to, "", func() {}, nil})
		case c.to == "":
			m.options = append(m.options, option{"Trash " + from, "", func() {}, nil})
		default:
			m.options = append(m.options, option{from + " → " + to, "", func() {}, nil})
		}
	}
	// Enter applies the displayed plan; Esc returns to the directory buffer.
	m.submit = func(string) {
		if err := e.applyDirectory(changes); err != nil {
			e.notify(err.Error())
			return
		}
		e.notify(fmt.Sprintf("Applied %d directory changes", len(changes)))
	}
	e.openMenu(m)
}

// Stage every source first, so rename cycles are safe. Reverse completed
// transfers on failure and leave the staging path visible if rollback fails.
func (e *editor) applyDirectory(changes []directoryChange) error {
	if err := e.checkDirectoryPlan(changes); err != nil {
		return err
	}
	d := e.directory
	staging, err := os.MkdirTemp(d.dir, ".loom-directory-")
	if err != nil {
		return err
	}
	type transfer struct{ from, to string }
	var transfers []transfer
	var created []string
	rename := func(from, to string) error {
		if _, err := os.Lstat(to); !os.IsNotExist(err) {
			return fmt.Errorf("destination unavailable: %s", to)
		}
		if err := os.Rename(from, to); err != nil {
			return err
		}
		transfers = append(transfers, transfer{from, to})
		return nil
	}
	rollback := func(cause error) error {
		var failure error
		for i := len(created) - 1; i >= 0; i-- {
			if err := os.Remove(created[i]); err != nil {
				failure = err
			}
		}
		for i := len(transfers) - 1; i >= 0; i-- {
			r := transfers[i]
			if _, err := os.Lstat(r.from); !os.IsNotExist(err) {
				failure = fmt.Errorf("rollback destination unavailable: %s", r.from)
				continue
			}
			if err := os.Rename(r.to, r.from); err != nil {
				failure = err
			}
		}
		if failure != nil {
			return fmt.Errorf("%v; rollback needs attention at %s: %w", cause, staging, failure)
		}
		_ = os.Remove(staging)
		return cause
	}
	staged := make([]string, len(changes))
	for i, c := range changes {
		if c.from == "" {
			continue
		}
		staged[i] = filepath.Join(staging, fmt.Sprint(i))
		if err := rename(c.from, staged[i]); err != nil {
			return rollback(err)
		}
	}
	for i, c := range changes {
		to := c.to
		if to == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return rollback(err)
			}
			trash := filepath.Join(home, ".Trash")
			if err := os.MkdirAll(trash, 0700); err != nil {
				return rollback(err)
			}
			to = filepath.Join(trash, filepath.Base(c.from))
			if _, err := os.Lstat(to); !os.IsNotExist(err) {
				to += "." + filepath.Base(staging)
			}
		}
		if c.from != "" {
			if err := rename(staged[i], to); err != nil {
				return rollback(err)
			}
			continue
		}
		if c.folder {
			if err := os.Mkdir(to, 0755); err != nil {
				return rollback(err)
			}
			created = append(created, to)
		} else {
			f, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				return rollback(err)
			}
			created = append(created, to)
			if err := f.Close(); err != nil {
				return rollback(err)
			}
		}
	}
	_ = os.Remove(staging)
	for _, b := range slices.Clone(e.buffers) {
		if b.path == "" {
			continue
		}
		for _, c := range changes {
			if c.from == "" || !within(c.from, b.path) {
				continue
			}
			if c.to == "" {
				e.closeBuffer(b)
				break
			}
			old := b.path
			rel, _ := filepath.Rel(c.from, old)
			b.path = filepath.Join(c.to, rel)
			b.version++
			b.diagnostics = nil
			e.closeDocument(old)
			break
		}
	}
	row := d.doc.b.rowAt(d.doc.b.cursor)
	e.directory = nil
	e.openDirectory(d.dir)
	if e.directory != nil {
		b := e.directory.doc.b
		b.cursor = b.lines[min(row, len(b.lines)-1)]
	}
	e.refreshTree("")
	e.revealFile()
	e.refreshGit()
	e.scheduleLanguage()
	return nil
}

func (e *editor) directoryEx(line string) bool {
	d := e.directory
	if d == nil {
		return false
	}
	if d.doc.exAddress(line) {
		return true
	}
	switch line {
	case "w", "write":
		e.writeDirectory()
	case "cd":
		e.directoryPicker()
	case "wq":
		if !d.doc.b.dirty() {
			e.closeDirectory(false)
		} else {
			e.writeDirectory()
			if !d.doc.b.dirty() {
				e.closeDirectory(false)
			} else if e.menu != nil {
				apply := e.menu.submit
				e.menu.submit = func(input string) {
					apply(input)
					if e.directory != d && e.directory != nil {
						e.closeDirectory(false)
					}
				}
			}
		}
	case "q", "quit":
		e.closeDirectory(false)
	case "q!", "quit!":
		e.closeDirectory(true)
	case "noh", "nohlsearch":
		d.doc.searchVisible = false
	case "vim":
		e.profile(line)
		d.doc.normal()
		d.doc.cfg.Profile = line
	default:
		if arg, ok := strings.CutPrefix(line, "cd "); ok {
			e.goDirectory(strings.TrimSpace(arg))
			return true
		}
		if strings.HasPrefix(line, "w ") || strings.HasPrefix(line, "write ") {
			e.notify("Use :w to apply edits to this directory")
			return true
		}
		return false
	}
	return true
}

// Outside the original project, relative targets are scoped to the directory
// currently being edited. Navigation itself has no project-root boundary.
func (e *editor) directoryDestination(dir, input string) (string, error) {
	if within(e.root, dir) {
		return e.treePath(dir, input)
	}
	path := filepath.Join(dir, input)
	if filepath.IsAbs(input) || path == dir || !within(dir, path) {
		return "", fmt.Errorf("enter a relative path inside this directory")
	}
	resolved, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	if resolved != filepath.Dir(path) {
		return "", fmt.Errorf("destination goes through a symbolic link")
	}
	return path, nil
}

func (e *editor) goDirectory(input string) {
	if input == "" {
		return
	}
	base := e.root
	if e.directory != nil {
		base = e.directory.dir
	} else if e.b.path != "" {
		base = filepath.Dir(e.b.path)
	}
	switch {
	case input == "~" || strings.HasPrefix(input, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			e.notify(err.Error())
			return
		}
		input = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(input, "~"), "/"))
	case !filepath.IsAbs(input):
		input = filepath.Join(base, input)
	}
	e.openDirectory(input)
}

func (e *editor) directoryPicker() {
	if e.directory != nil && e.directory.doc.b.dirty() {
		e.notify("Write or discard directory edits before jumping")
		return
	}
	m := &menu{title: "Jump to Directory", submit: e.goDirectory, exact: func(input string) bool {
		if filepath.IsAbs(input) || strings.HasPrefix(input, "~") || strings.HasPrefix(input, ".") {
			return true
		}
		base := e.root
		if e.directory != nil {
			base = e.directory.dir
		} else if e.b.path != "" {
			base = filepath.Dir(e.b.path)
		}
		info, err := os.Stat(filepath.Join(base, input))
		return input != "" && err == nil && info.IsDir()
	}}
	m.options = append(m.options, option{"Enter a path", "absolute, relative, or ~/…", func() {
		e.openMenu(&menu{title: "Directory Path", submit: e.goDirectory})
	}, nil})
	base := e.root
	if e.directory != nil {
		base = e.directory.dir
	}
	for _, path := range []string{filepath.Dir(base), base, e.root} {
		found := slices.ContainsFunc(m.options, func(o option) bool { return o.hint == path })
		if !found {
			m.options = append(m.options, option{filepath.Base(path) + "/", path, func() { e.openDirectory(path) }, func() itemPreview { return directoryPreview(path) }})
		}
	}
	e.openMenu(m)
	go func() {
		var options []option
		_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() || path == base {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "vendor" || d.Name() == "target" {
				return fs.SkipDir
			}
			rel, _ := filepath.Rel(base, path)
			options = append(options, option{rel + "/", path, func() { e.openDirectory(path) }, func() itemPreview { return directoryPreview(path) }})
			if len(options) >= 2500 {
				return fs.SkipAll
			}
			return nil
		})
		e.async <- func() {
			if e.menu == m {
				m.options = append(m.options, options...)
				m.filter()
			}
		}
	}()
}
