package editor

import (
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Command runs an action of the menu bar or of a button.
func (b *Bridge) Command(id string) {
	if b.peer != nil {
		if id == "paste" {
			// The editor is elsewhere; give it this computer's clipboard first.
			if text, ok := systemPaste(); ok {
				b.peer.forward("Clipboard", text)
			}
		}
		b.peer.forward("Command", id)
		return
	}
	b.call(func() { b.command(id) })
}

func (b *Bridge) command(id string) {
	if b.dialog != nil {
		return
	}
	e := b.m.editor
	if s := b.settings; s != nil && b.settingsHaveKeys() {
		switch id {
		case "paste":
			if text, ok := clipboardPaste(); ok {
				b.settingsKey(keyInput{paste: text})
			}
			return
		case "copy", "cut", "undo", "redo", "select-all":
			return
		case "search":
			s.focusSearch()
			return
		case "close-tab":
			b.settings = nil
			return
		case "zoom-in", "zoom-out", "zoom-reset", "quit", "settings":
		default:
			b.settings = nil
		}
	}
	doc := e
	if e.directory != nil {
		doc = e.directory.doc
	}
	// Text actions mean the field or document that has the keyboard.
	if b.fieldCommand(id) {
		return
	}
	switch id {
	case "copy", "cut":
		if e.menu != nil || e.bar != nil || doc.b.dashboard || doc.mode != "select" || doc.b.anchor < 0 {
			return
		}
		op := "yank"
		if id == "cut" {
			op = "delete"
		}
		doc.operate(op, doc.b.selection(), doc.b.visualLine)
		if id == "cut" {
			e.scheduleLanguage()
		}
		return
	case "paste":
		text, ok := clipboardPaste()
		if !ok || text == "" || doc.b.dashboard && e.menu == nil {
			return
		}
		if e.menu == nil && e.bar == nil && doc.mode == "select" && doc.b.anchor >= 0 {
			doc.operate("delete", doc.b.selection(), doc.b.visualLine)
		}
		b.send(tea.PasteMsg{Content: text})
		return
	case "select-all", "undo", "redo":
		if e.menu != nil || e.bar != nil || doc.b.dashboard {
			return
		}
		if doc.mode == "insert" && id != "select-all" {
			id += "-here" // stays in insert mode
		}
		doc.commands[id].run(doc)
		e.scheduleLanguage()
		e.scheduleGit()
		return
	case "zoom-in":
		b.zoom(1)
		return
	case "zoom-out":
		b.zoom(-1)
		return
	case "zoom-reset":
		b.zoom(0)
		return
	case "quit":
		if b.quitApp != nil {
			b.quitApp()
		} else {
			b.requestQuit()
		}
		return
	}

	// Everything else starts from the editor, not from whatever popup is up.
	e.menu, e.notificationDetail, e.diff = nil, nil, nil
	if id != "git" && id != "branch" {
		e.panel = nil
	}
	if id == "save" {
		b.save()
		return
	}
	if e.directory != nil {
		if e.directory.doc.b.dirty() {
			e.notify("Write or discard directory edits first")
			return
		}
		e.directory = nil
	}
	run := func(name string) {
		if c, ok := e.commands[name]; ok {
			c.run(e)
		}
	}
	switch id {
	case "new-file":
		b.newFile()
	case "open-file":
		b.openFile()
	case "open-project":
		e.chooseProject()
	case "projects":
		if e.noProject && len(recentProjects()) == 0 {
			e.chooseProject()
		} else {
			e.projectMenu()
		}
	case "close-project":
		e.closeProject()
	case "save-as":
		b.saveAs()
	case "close-tab":
		if len(e.buffers) == 1 && e.b.dashboard {
			b.requestQuit()
		} else {
			b.closeTab(e.b)
		}
	case "files":
		if e.noProject {
			e.chooseProject()
		} else {
			run("files")
		}
	case "palette":
		e.openPalette()
	case "sidebar":
		run("toggle-sidebar")
	case "git":
		if e.panel != nil {
			e.panel = nil
		} else {
			run("git-panel")
		}
	case "branch":
		run("git-branch")
	case "search", "replace", "goto-line":
		if e.b.dashboard {
			return
		}
		e.tree.focused = false
		if e.mode == "insert" {
			e.normal()
		}
		if id == "goto-line" {
			e.openMenu(&menu{title: "Go to Line", hidePreview: true, submit: e.ex})
		} else {
			run(id)
		}
	case "shortcuts":
		e.showKeys()
	case "welcome":
		e.showDashboard()
	default: // project-search, settings, theme, problems, sessions
		run(id)
	}
}

// fieldCommand runs Copy, Cut and Select All in the text field of an open
// picker or of the find bar. Paste goes there as typed text.
func (b *Bridge) fieldCommand(id string) bool {
	e := b.m.editor
	var text *string
	var f *lineField
	switch {
	case e.menu != nil && !e.menu.confirmation:
		text, f = &e.menu.input, &e.menu.field
	case e.menu == nil && e.bar != nil:
		text, f = e.barText(), e.barField()
	default:
		return false
	}
	switch id {
	case "select-all":
		f.selectAll(*text)
	case "copy", "cut":
		lo, hi, ok := f.selection(*text)
		if !ok {
			return true
		}
		clipboardCopy(f.selected(*text))
		if id == "cut" {
			f.replace(text, lo, hi, "")
			if e.menu != nil {
				e.menu.filter()
			} else if e.bar.field == 0 {
				e.barSync(true)
			}
		}
	case "undo", "redo":
	default:
		return false
	}
	return true
}

// zoom changes the saved font size. Zero goes back to the default.
func (b *Bridge) zoom(delta float64) {
	e := b.m.editor
	var err error
	if delta == 0 {
		err = e.resetSetting("font_size")
	} else {
		s := settingByKey["font_size"]
		err = e.setSetting(s.key, max(s.min, min(s.max, e.cfg.FontSize+delta)))
	}
	if err != nil {
		e.notify(err.Error())
	}
}

func (b *Bridge) newFile() {
	e := b.m.editor
	buf, err := loadBuffer("")
	if err != nil {
		e.notify(err.Error())
		return
	}
	e.buffers = append(e.buffers, buf)
	e.switchBuffer(buf)
	e.mode = "insert"
}

// pick asks the host for a system panel and falls back to a prompt in the
// window, as a remote editor has no panel to show.
func (b *Bridge) pick(kind pickKind, start string, title string, chosen func(string)) {
	if b.host != nil && b.host.pick(kind, start, chosen) {
		return
	}
	b.m.editor.openMenu(&menu{title: title, hidePreview: true, input: start + string(filepath.Separator), submit: func(path string) {
		if strings.TrimSpace(path) != "" {
			chosen(path)
		}
	}})
}

type pickKind int

const (
	pickFolder pickKind = iota
	pickFile
	pickSave
)

func (b *Bridge) openFile() {
	e := b.m.editor
	start := e.root
	if e.b.path != "" {
		start = filepath.Dir(e.b.path)
	}
	b.pick(pickFile, start, "Open File", func(path string) {
		if e.hub.show(e) {
			e.open(path)
		}
	})
}

func (b *Bridge) save() {
	e := b.m.editor
	if e.directory == nil && !e.b.dashboard && e.b.path == "" {
		b.saveAs()
		return
	}
	e.save()
}

func (b *Bridge) saveAs() {
	e := b.m.editor
	buf := e.b
	if buf.dashboard {
		return
	}
	start := e.root
	if buf.path != "" {
		start = filepath.Dir(buf.path)
	}
	// The system panel has already asked before replacing a file.
	b.pick(pickSave, start, "Save As", func(path string) {
		if e.hub.show(e) {
			b.writeTo(buf, path)
		}
	})
}

func (b *Bridge) writeTo(buf *buffer, path string) {
	e := b.m.editor
	if !slices.Contains(e.buffers, buf) {
		return
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	for _, other := range e.buffers {
		if other != buf && other.path == path {
			e.notify("That file is open in another tab")
			return
		}
	}
	e.switchBuffer(buf)
	old := buf.path
	if path == old {
		e.save()
		return
	}
	state, err := readDisk(path)
	if err != nil {
		e.notify(err.Error())
		return
	}
	buf.path = path
	if !e.saveAgainst(&state, false) {
		buf.path = old
		return
	}
	e.invalidateCompletion()
	e.closeDocument(old)
	e.refreshTree("")
	e.revealFile()
	e.scheduleLanguage()
}

// dialog is a question in the middle of the window.
type dialog struct {
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Choices  []string `json:"choices"`
	Selected int      `json:"selected"`
	runs     []func()
}

func (b *Bridge) ask(title, body string, choices ...dialogChoice) {
	q := &dialog{Title: title, Body: body}
	for _, c := range choices {
		q.Choices = append(q.Choices, c.label)
		q.runs = append(q.runs, c.run)
	}
	b.dialog = q
}

type dialogChoice struct {
	label string
	run   func()
}

// Answer picks a dialog's button; -1 is Escape, which is the last one.
func (b *Bridge) Answer(i int) {
	if b.peer != nil {
		b.peer.forward("Answer", i)
		return
	}
	b.call(func() { b.answer(i) })
}

func (b *Bridge) answer(i int) {
	q := b.dialog
	if q == nil {
		return
	}
	if i < 0 || i >= len(q.runs) {
		i = len(q.runs) - 1
	}
	b.dialog = nil
	if run := q.runs[i]; run != nil {
		run()
	}
}

func (b *Bridge) closeTab(buf *buffer) {
	e := b.m.editor
	finish := func() {
		if !slices.Contains(e.buffers, buf) || buf.dirty() {
			return
		}
		e.closeBuffer(buf)
		// The last tab gives way to the start page rather than an empty file.
		if len(e.buffers) == 1 && e.b.path == "" && !e.b.dashboard && !e.b.dirty() {
			e.showDashboard()
		}
	}
	if !buf.dirty() {
		finish()
		return
	}
	b.ask("Save changes to "+bufferName(buf)+"?", "Your changes will be lost if you don't save them.",
		dialogChoice{"Save", func() {
			e.switchBuffer(buf)
			if buf.path == "" {
				b.saveAs()
				return
			}
			e.save()
			finish()
		}},
		dialogChoice{"Don't Save", func() {
			buf.saved = string(buf.text)
			finish()
		}},
		dialogChoice{"Cancel", nil})
}

func (b *Bridge) unsaved() bool { return len(b.m.editor.hub.unsaved()) > 0 }

// requestQuit closes this window's editor, asking first about unsaved work.
func (b *Bridge) requestQuit() {
	if !b.unsaved() {
		b.m.editor.hub.quitAll(false)
		return
	}
	b.askQuit(nil)
}

// askQuit asks what to do with this window's unsaved work before its editor
// quits. Cancel runs cancelled.
func (b *Bridge) askQuit(cancelled func()) {
	h := b.m.editor.hub
	waiting := h.unsaved()
	body := "Some files have changes that haven't been saved."
	if len(h.live) > 1 {
		var names []string
		for _, e := range waiting {
			names = append(names, e.sessionName())
		}
		body = "Unsaved changes in these sessions: " + strings.Join(names, ", ") + "."
	}
	b.ask("Save your changes before closing?", body,
		dialogChoice{"Save All", func() {
			if b.saveEverySession() {
				h.quitAll(false)
			} else if cancelled != nil {
				cancelled()
			}
		}},
		dialogChoice{"Don't Save", func() { h.quitAll(true) }},
		dialogChoice{"Cancel", cancelled})
}

// saveEverySession saves in each session as if it were on screen, which is
// what formatters need. It stops, showing the session, where it can't.
func (b *Bridge) saveEverySession() bool {
	for _, e := range b.m.editor.hub.unsaved() {
		if !e.hub.show(e) {
			return false
		}
		if e.directory != nil && e.directory.doc.b.dirty() {
			e.notify("Apply or discard the directory edits first")
			return false
		}
		e.menu, e.directory = nil, nil
		for _, buf := range slices.Clone(e.buffers) {
			if buf.dashboard || !buf.dirty() {
				continue
			}
			e.switchBuffer(buf)
			if buf.path == "" {
				b.saveAs()
				return false
			}
			e.save()
			if buf.dirty() {
				return false // the editor is showing why it could not save
			}
		}
	}
	return true
}
