package editor

import (
	"fmt"
	"image/color"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// The chrome is described to the web view as plain data; it lays it out with
// CSS. Colours are resolved here, from the editor theme.

func (b *Bridge) gitInk(mark byte) color.NRGBA {
	c := &b.frame.colors
	switch mark {
	case '?', 'A', '+':
		return c.green
	case 'D', '-':
		return c.red
	}
	return nrgba(gitColor(mark), c.yellow)
}

// fileIcon is a file's monogram and its colour; an empty mark means the
// plain file glyph.
func (b *Bridge) fileIcon(path string) []string {
	mark, ink := fileMarkOf(&b.frame.colors, path)
	return []string{mark, hexColor(readable(ink, b.frame.colors.side, 3.5))}
}

func (b *Bridge) chrome() map[string]any {
	e := b.m.editor
	ui := map[string]any{"title": b.titleState()}
	if s := b.sidebarState(); s != nil {
		ui["sidebar"] = s
	}
	if !e.zen && (len(e.buffers) > 1 || !e.b.dashboard) {
		ui["tabs"] = b.tabsState()
		if !e.b.dashboard && e.b.path != "" && e.cfg.PathBar {
			ui["crumbs"] = b.crumbsState()
		}
	}
	if !e.zen && e.cfg.StatusBar {
		ui["status"] = b.statusState()
	}
	if b.dialog != nil {
		ui["dialog"] = b.dialog
	}
	b.popups(ui)
	b.panels(ui)
	if b.settings != nil {
		ui["settings"] = b.settingsState()
	}
	return ui
}

func (b *Bridge) titleState() map[string]any {
	e := b.m.editor
	name := "Open Project"
	if !e.noProject {
		name = filepath.Base(e.root)
	}
	t := map[string]any{
		"project": name, "noProject": e.noProject, "sidebar": e.tree.sidebar,
		"sessions": len(e.hub.live), "gitDot": len(e.git.files) > 0,
	}
	if e.git.branch != "" && !e.noProject {
		t["branch"] = e.git.branch
	}
	if b.remoteHost != "" {
		t["host"] = b.remoteHost
	}
	if e.noProject {
		t["search"] = []string{"Search commands", shortcutText("⇧⌘P", "Space p"), "palette"}
	} else {
		t["search"] = []string{"Search files", shortcutText("⌘P", "Ctrl+P"), "files"}
	}
	return t
}

func (b *Bridge) sidebarState() map[string]any {
	e, t := b.m.editor, &b.m.editor.tree
	if e.zen || e.noProject || !(t.sidebar || t.popup || t.focused) {
		return nil
	}
	c := &b.frame.colors
	rows := make([]any, 0, len(t.rows))
	for i, row := range t.rows {
		rel, _ := filepath.Rel(e.root, row.path)
		r := map[string]any{"n": filepath.Base(row.path), "d": row.depth, "p": rel}
		if row.dir {
			r["dir"] = true
			if t.expanded[row.path] || t.filter != "" {
				r["open"] = true
			}
		} else {
			r["icon"] = b.fileIcon(row.path)
			if row.path == e.b.path {
				r["cur"] = true
			}
		}
		if i == t.selected && t.focused {
			r["sel"] = true
		}
		if mark := e.git.files[row.path]; mark != 0 {
			label := string(mark)
			if mark == '?' {
				label = "U"
			}
			r["git"] = []string{label, hexColor(readable(b.gitInk(mark), c.side, 3.5))}
		}
		if t.cut == row.path {
			r["cut"] = true
		}
		rows = append(rows, r)
	}
	s := map[string]any{"rows": rows, "width": max(18, e.tree.width) * 9, "selected": t.selected}
	if t.focused || t.reveal {
		s["reveal"] = t.selected
	}
	t.reveal = false
	if t.filtering || t.filter != "" {
		s["filter"] = t.filter
		s["filtering"] = t.filtering
	}
	if t.sortMode != 0 {
		s["sort"] = "by " + treeSorts[t.sortMode]
	}
	return s
}

func (b *Bridge) tabsState() []any {
	e := b.m.editor
	names := make([]string, len(e.buffers))
	for i, buf := range e.buffers {
		names[i] = bufferName(buf)
	}
	tabs := make([]any, 0, len(e.buffers))
	for i, buf := range e.buffers {
		t := map[string]any{"name": names[i]}
		// Equal names get their folder to tell them apart.
		if buf.path != "" && slices.ContainsFunc(e.buffers, func(o *buffer) bool { return o != buf && bufferName(o) == names[i] }) {
			t["folder"] = filepath.Base(filepath.Dir(buf.path))
		}
		if buf == e.b {
			t["active"] = true
		}
		if buf.dirty() {
			t["dirty"] = true
		}
		if buf.dashboard {
			t["welcome"] = true
		} else {
			t["icon"] = b.fileIcon(buf.path)
		}
		tabs = append(tabs, t)
	}
	return tabs
}

func (b *Bridge) crumbsState() map[string]any {
	e := b.m.editor
	path := shortPath(e.b.path)
	if !e.noProject && within(e.root, e.b.path) {
		if rel, err := filepath.Rel(e.root, e.b.path); err == nil {
			path = rel
		}
	}
	parts := strings.Split(strings.Trim(filepath.ToSlash(path), "/"), "/")
	s := map[string]any{"dirs": parts[:len(parts)-1], "name": parts[len(parts)-1], "icon": b.fileIcon(e.b.path)}
	if e.cfg.Breadcrumbs {
		s["scopes"] = e.scopeTrail(e.b)
	}
	return s
}

func (b *Bridge) statusState() map[string]any {
	e := b.m.editor
	c := &b.frame.colors
	doc := e
	if e.directory != nil {
		doc = e.directory.doc
	}
	buf := doc.b
	s := map[string]any{}
	if !buf.dashboard || e.menu != nil || e.tree.focused {
		mode, modeColor := e.modeBadge()
		fill := nrgba(modeColor, c.accent)
		ink := black
		if contrast(white, fill) > contrast(black, fill) {
			ink = white
		}
		s["mode"] = []string{mode, hexColor(fill), hexColor(ink)}
	}
	keys := keyLabel(doc.prefix)
	if doc.pending.kind != "" {
		keys += " " + doc.pending.kind
	}
	if doc.count > 0 {
		keys = fmt.Sprintf("%d %s", doc.count, keys)
	}
	if doc.recording != "" {
		keys = "Recording @" + doc.recording + " " + keys
	}
	if keys = strings.TrimSpace(keys); keys != "" {
		s["keys"] = keys
	}
	if !buf.dashboard && e.problemsVisible(buf) {
		file, project := countProblems(buf.diagnostics), e.projectProblems()
		errors, warnings := file.errors+project.errors, file.warnings+project.warnings
		p := map[string]any{"errors": errors, "warnings": warnings}
		if errors > 0 {
			p["errorInk"] = hexColor(readable(c.red, c.side, 3.5))
		}
		if warnings > 0 {
			p["warningInk"] = hexColor(readable(c.yellow, c.side, 3.5))
		}
		s["problems"] = p
	}
	var segments [][]string // text and the command a click runs, right to left
	switch {
	case buf.dashboard:
		segments = append(segments, []string{"Loom " + version, ""})
	case e.directory != nil:
	default:
		row := buf.rowAt(buf.cursor)
		segments = append(segments, []string{fmt.Sprintf("Ln %d, Col %d", row+1, buf.cursor-buf.lines[row]+1), "goto-line"})
		segments = append(segments, []string{languageName(e.cfg.languageFor(buf.path)), ""})
		ending := "LF"
		if buf.newline() == "\r\n" {
			ending = "CRLF"
		}
		segments = append(segments, []string{ending, ""}, []string{"UTF-8", ""})
		if buf.external {
			segments = append(segments, []string{"Changed on disk", "save"})
		}
	}
	s["segments"] = segments
	return s
}

// Act is a click or drag on the chrome: name says what, i and s carry the
// row, tab or key it was on.
func (b *Bridge) Act(name string, i int, s string) {
	if b.peer != nil {
		b.peer.forward("Act", name, i, s)
		return
	}
	b.call(func() { b.act(name, i, s) })
}

func (b *Bridge) act(name string, i int, s string) {
	if b.popupAct(name, i, s) || b.panelAct(name, i, s) || b.settingsAct(name, i, s) {
		return
	}
	e := b.m.editor
	t := &e.tree
	inTree := i >= 0 && i < len(t.rows)
	inTabs := i >= 0 && i < len(e.buffers)
	switch name {
	case "tree-open", "tree-context":
		if !inTree {
			return
		}
		t.selected, t.focused, t.manualScroll = i, true, false
		if name == "tree-open" {
			e.treeInput("enter")
		} else {
			e.treeContext()
		}
	case "tree-key":
		if !e.b.dashboard && e.mode == "insert" {
			e.normal()
		}
		t.focused = true
		e.treeInput(s)
	case "tree-drop":
		// A drag in the sidebar: s is the dragged path and the one it was
		// dropped on, relative to the project, on two lines.
		from, onto, _ := strings.Cut(s, "\n")
		if err := e.dropEntry(filepath.Join(e.root, from), filepath.Join(e.root, onto)); err != nil {
			e.notify(err.Error())
		}
	case "tree-expand":
		e.expandEntry(i)
	case "tree-clear-filter":
		t.filter, t.filtering = "", false
		e.refreshTree("")
	case "tree-width":
		t.width = max(18, min(64, i/9))
	case "tab":
		if inTabs {
			e.switchBuffer(e.buffers[i])
		}
	case "tab-close":
		if inTabs {
			b.closeTab(e.buffers[i])
		}
	case "tab-over":
		// A tab dragged over the text: i is the cell's column, s its row.
		v := &b.m.view
		y, err := strconv.Atoi(s)
		if err != nil {
			return
		}
		v.tabDrag, v.tabMoving, v.drop = e.b, true, dropZone{}
		for _, pane := range v.panes {
			if pane.leaf != nil && pane.contains(i, y) && !pane.b.dashboard {
				v.drop = dropZoneAt(pane, i, y)
			}
		}
	case "tab-drop":
		v := &b.m.view
		if v.tabMoving && v.drop.leaf != nil {
			v.dropTab(e)
		}
		v.tabDrag, v.tabMoving, v.drop = nil, false, dropZone{}
	case "tab-move":
		// The dragged tab is the active one; i is where it goes.
		if inTabs {
			e.moveTab(i)
		}
	}
}
