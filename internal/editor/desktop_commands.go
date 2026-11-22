//go:build desktop

package editor

import (
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// command runs an action of the menu bar or of a button.
func (d *desktop) command(id string) {
	if d.dialog != nil {
		return
	}
	e := d.m.editor
	if s := d.settings; s != nil && d.settingsHaveKeys() {
		switch id {
		case "paste":
			if text, ok := clipboardPaste(); ok {
				d.settingsKey(keyInput{paste: text})
			}
			return
		case "copy", "cut", "undo", "redo", "select-all":
			return
		case "search":
			s.focusSearch()
			return
		case "close-tab":
			d.settings = nil
			return
		case "zoom-in", "zoom-out", "zoom-reset", "quit", "settings":
		default:
			d.settings = nil
		}
	}
	doc := e
	if e.directory != nil {
		doc = e.directory.doc
	}
	// Text actions mean the field or document that has the keyboard.
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
		d.send(tea.PasteMsg{Content: text})
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
		d.zoom(1)
		return
	case "zoom-out":
		d.zoom(-1)
		return
	case "zoom-reset":
		d.zoom(0)
		return
	case "quit":
		d.requestQuit()
		return
	}

	// Everything else starts from the editor, not from whatever popup is up.
	e.menu, e.notificationDetail, e.diff = nil, nil, nil
	if id != "git" && id != "branch" {
		e.panel = nil
	}
	if id == "save" {
		d.save()
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
		d.newFile()
	case "open-file":
		d.openFile()
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
		d.saveAs()
	case "close-tab":
		if len(e.buffers) == 1 && e.b.dashboard {
			d.requestQuit()
		} else {
			d.closeTab(e.b)
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
	default: // project-search, settings, theme, problems
		run(id)
	}
}

func (d *desktop) newFile() {
	e := d.m.editor
	b, err := loadBuffer("")
	if err != nil {
		e.notify(err.Error())
		return
	}
	e.buffers = append(e.buffers, b)
	e.switchBuffer(b)
	e.mode = "insert"
}

func (d *desktop) openFile() {
	e := d.m.editor
	start := e.root
	if e.b.path != "" {
		start = filepath.Dir(e.b.path)
	}
	if !d.pickPath(pickFile, start, func(path string) { e.open(path) }) {
		e.openMenu(&menu{title: "Open File", hidePreview: true, submit: func(path string) { e.open(path) }})
	}
}

func (d *desktop) save() {
	e := d.m.editor
	if e.directory == nil && !e.b.dashboard && e.b.path == "" {
		d.saveAs()
		return
	}
	e.save()
}

func (d *desktop) saveAs() {
	e := d.m.editor
	b := e.b
	if b.dashboard {
		return
	}
	start := e.root
	if b.path != "" {
		start = filepath.Dir(b.path)
	}
	// The system panel has already asked before replacing a file.
	if !d.pickPath(pickSave, start, func(path string) { d.writeTo(b, path) }) {
		e.openMenu(&menu{title: "Save As", hidePreview: true, submit: func(path string) {
			if path != "" && slices.Contains(e.buffers, b) {
				e.switchBuffer(b)
				e.ex("w " + path)
			}
		}})
	}
}

func (d *desktop) writeTo(b *buffer, path string) {
	e := d.m.editor
	if !slices.Contains(e.buffers, b) {
		return
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	for _, other := range e.buffers {
		if other != b && other.path == path {
			e.notify("That file is open in another tab")
			return
		}
	}
	e.switchBuffer(b)
	old := b.path
	if path == old {
		e.save()
		return
	}
	state, err := readDisk(path)
	if err != nil {
		e.notify(err.Error())
		return
	}
	b.path = path
	if !e.saveAgainst(&state, false) {
		b.path = old
		return
	}
	e.invalidateCompletion()
	e.closeDocument(old)
	e.refreshTree("")
	e.revealFile()
	e.scheduleLanguage()
}

func (d *desktop) closeTab(b *buffer) {
	e := d.m.editor
	finish := func() {
		if !slices.Contains(e.buffers, b) || b.dirty() {
			return
		}
		e.closeBuffer(b)
		// The last tab gives way to the start page rather than an empty file.
		if len(e.buffers) == 1 && e.b.path == "" && !e.b.dashboard && !e.b.dirty() {
			e.showDashboard()
		}
	}
	if !b.dirty() {
		finish()
		return
	}
	d.dialog = &dialog{
		title: "Save changes to " + bufferName(b) + "?",
		body:  "Your changes will be lost if you don't save them.",
		choices: []dialogChoice{
			{"Save", func() {
				e.switchBuffer(b)
				if b.path == "" {
					d.saveAs()
					return
				}
				e.save()
				finish()
			}},
			{"Don't Save", func() {
				b.saved = string(b.text)
				finish()
			}},
			{"Cancel", nil},
		},
	}
}

func (d *desktop) unsaved() bool {
	e := d.m.editor
	if e.directory != nil && e.directory.doc.b.dirty() {
		return true
	}
	return slices.ContainsFunc(e.buffers, func(b *buffer) bool { return !b.dashboard && b.dirty() })
}

func (d *desktop) requestQuit() {
	e := d.m.editor
	if !d.unsaved() {
		e.quit = true
		return
	}
	d.dialog = &dialog{
		title: "Save your changes before quitting?",
		body:  "Some files have changes that haven't been saved.",
		choices: []dialogChoice{
			{"Save All", func() {
				if e.directory != nil && e.directory.doc.b.dirty() {
					e.notify("Apply or discard the directory edits first")
					return
				}
				e.menu, e.directory = nil, nil
				for _, b := range slices.Clone(e.buffers) {
					if b.dashboard || !b.dirty() {
						continue
					}
					e.switchBuffer(b)
					if b.path == "" {
						d.saveAs()
						return
					}
					e.save()
					if b.dirty() {
						return // the editor is showing why it could not save
					}
				}
				e.quit = true
			}},
			{"Don't Save", func() { e.discardRecovery, e.quit = true, true }},
			{"Cancel", nil},
		},
	}
}

type pickKind int

const (
	pickFolder pickKind = iota
	pickFile
	pickSave
)

// askAboutConfig reports a config that couldn't be used. A toast would be
// gone, or replaced by a language server's, before it was read.
func (d *desktop) askAboutConfig() {
	e := d.m.editor
	if e.configProblem == nil {
		return
	}
	file, problem := e.userConfigPath(), e.configProblem.Error()
	for _, other := range []string{addedPath(file), filepath.Join(e.root, ".loom.json")} {
		if other != "" && strings.Contains(problem, other) {
			file = other
		}
	}
	d.dialog = &dialog{
		title: "Some settings couldn't be loaded",
		body:  problem + "\n\nLoom started without them. Fix the file and save it to apply it.",
		choices: []dialogChoice{
			{"Open File", func() { e.open(file) }},
			{"Continue", nil},
		},
	}
}
