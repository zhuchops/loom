package editor

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// A session is a project open in Loom. Every one that is loaded has an editor
// of its own, with its tabs, undo, language servers, git state and tree, and
// the front end shows one of them. Only the shown one owns what is global to
// the process: the working directory, the theme and the tab width.
type sessionHub struct {
	current *editor
	live    []*editor // most recently shown first
	// onShow, set by a front end, follows the switch.
	onShow func(*editor)
	// window, set by the desktop app, starts another window on dir, or on
	// the start page when dir is empty.
	window func(dir string) error
}

// openWindow starts a window of its own on dir.
func (h *sessionHub) openWindow(dir string) {
	if h.window == nil {
		h.current.notify("New windows are part of the desktop app")
		return
	}
	if err := h.window(dir); err != nil {
		h.current.notify("Can't open a new window: " + err.Error())
		return
	}
	if dir != "" {
		h.current.notify("Opened " + shortPath(dir) + " in a new window")
	}
}

// moveToWindow takes a session out of this window and opens it in a new
// one, where its tabs come back. A session with unsaved changes stays: they
// live only here.
func (h *sessionHub) moveToWindow(o *editor) {
	if h.window == nil {
		h.openWindow("")
		return
	}
	if o.noProject {
		h.openWindow("")
		return
	}
	if o.unsavedCount() > 0 {
		h.current.notify("Save or discard the changes in " + o.sessionName() + " before moving it to a new window")
		return
	}
	root := o.root
	if len(h.live) > 1 {
		h.closeSession(o)
	} else {
		o.closeProject()
	}
	if h.find(root) != nil {
		return // it would not close
	}
	h.openWindow(root)
}

func sessionsPath() string {
	root := stateRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "open-sessions")
}

// savedSessions are the sessions that were left open, most recent first. The
// ones not loaded yet start when they are picked.
func savedSessions() []string {
	var out []string
	for _, p := range readLines(sessionsPath()) {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// changeSessions edits the saved list as it is on disk now, as another Loom
// may have changed it.
func changeSessions(change func([]string) []string) {
	path := sessionsPath()
	if path == "" || os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	var text strings.Builder
	for _, root := range change(readLines(path)) {
		text.WriteString(root + "\n")
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".open-sessions-")
	if err != nil {
		return
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString(text.String())
	if closeErr := f.Close(); err == nil && closeErr == nil {
		_ = os.Rename(f.Name(), path)
	}
}

func rememberOpenSession(root string) {
	changeSessions(func(list []string) []string {
		list = slices.DeleteFunc(list, func(p string) bool { return p == root })
		return append([]string{root}, list...)
	})
}

func forgetOpenSession(root string) {
	changeSessions(func(list []string) []string {
		return slices.DeleteFunc(list, func(p string) bool { return p == root })
	})
}

func (e *editor) sessionName() string {
	if e.noProject {
		return "No project"
	}
	return filepath.Base(e.root)
}

// unsavedCount is how much would be lost if the session went away.
func (e *editor) unsavedCount() int {
	n := 0
	for _, b := range e.buffers {
		if !b.dashboard && b.dirty() {
			n++
		}
	}
	if e.directory != nil && e.directory.doc.b.dirty() {
		n++
	}
	return n
}

// blank is the start page with nothing in it, not worth keeping as a session.
func (e *editor) blank() bool {
	return e.noProject && e.unsavedCount() == 0 && !slices.ContainsFunc(e.buffers, func(b *buffer) bool { return b.path != "" })
}

func (h *sessionHub) find(root string) *editor {
	for _, e := range h.live {
		if !e.noProject && e.root == root {
			return e
		}
	}
	return nil
}

// unsaved lists the sessions with changes, the shown one first.
func (h *sessionHub) unsaved() []*editor {
	var out []*editor
	for _, e := range h.live {
		if e.unsavedCount() > 0 {
			out = append(out, e)
		}
	}
	return out
}

func (h *sessionHub) saveRecovery() {
	for _, e := range h.live {
		if err := e.saveRecovery(); err != nil {
			h.current.notify("Recovery: " + err.Error())
		}
	}
}

// show makes e the session on screen. It reports false when e is gone. A
// session whose folder is gone is shown all the same: it may hold changes
// that exist nowhere else.
func (h *sessionHub) show(e *editor) bool {
	if e == h.current {
		return true
	}
	if !slices.Contains(h.live, e) {
		return false
	}
	lost := os.Chdir(e.root)
	if lost != nil {
		if home, err := os.UserHomeDir(); err == nil {
			_ = os.Chdir(home)
		}
	}
	from := h.current
	from.leave()
	e.takeRegisters(from)

	h.current = e
	h.live = slices.DeleteFunc(h.live, func(o *editor) bool { return o == e })
	h.live = append([]*editor{e}, h.live...)
	if from.blank() {
		h.drop(from)
	}
	if h.onShow != nil {
		h.onShow(e)
	}
	e.arrive()
	if lost != nil {
		e.notify("This session's folder can't be opened: " + lost.Error())
	}
	return true
}

// leave puts a session away as it is, without closing anything in it.
func (e *editor) leave() {
	e.normal()
	e.invalidateCompletion()
	e.rememberPane()
	e.menu, e.prefix, e.count = nil, "", 0
	e.signature = signatureHint{}
	e.leftTheme, e.leftItalic = th.name, italicComments
	e.saveSession()
	if err := e.saveRecovery(); err != nil {
		e.notify("Recovery: " + err.Error())
	}
}

// arrive takes over the process and catches up on what changed meanwhile.
func (e *editor) arrive() {
	e.message = ""
	applyTheme(defaultTheme())
	if err := loadTheme(e.cfg); err != nil {
		e.notify(err.Error())
	}
	italicComments = e.cfg.ItalicComments
	// Settings changed in another session hold here too.
	e.syncConfig()
	e.useTabWidth(e.b)
	if e.leftTheme != "" && (e.leftTheme != th.name || e.leftItalic != italicComments) {
		e.refreshHighlights()
	}
	e.recent = nil
	e.loadRecent()
	e.checkDisk()
	if e.noProject {
		forgetLastProject()
		return
	}
	rememberProject(e.root)
	e.refreshTree("")
	e.refreshGit()
	e.scheduleLanguage()
}

// takeRegisters carries what was copied over to the session being shown.
func (e *editor) takeRegisters(from *editor) {
	e.register, e.linewise = from.register, from.linewise
	e.registerTags = nil // they refer to the other session's directory listing
	e.history = slices.Clone(from.history)
	e.lastMacro = from.lastMacro
}

// startSession opens dir in a session of its own, beside the others.
func (e *editor) startSession(dir string) {
	h := e.hub
	if err := os.Chdir(dir); err != nil {
		e.notify("Can't open " + shortPath(dir) + ": " + err.Error())
		return
	}
	fail := func(err error) {
		_ = os.Chdir(e.root)
		e.useTabWidth(e.b)
		italicComments = e.cfg.ItalicComments
		e.notify(err.Error())
	}
	cfg, problem := loadConfig(e.cfg.path, e.cfg.explicit, dir)
	if problem == nil {
		problem = checkTheme(cfg)
	}
	if problem != nil {
		// Start with the settings the shown session already runs on.
		cfg = e.cfg
		if user, ok := usableConfig(e.cfg.path, e.cfg.explicit); ok {
			cfg = user
		}
	}
	cfg.Profile = e.cfg.Profile
	// Plugins are read once, at startup.
	cfg.Plugins = e.cfg.Plugins
	cfg.Languages = maps.Clone(cfg.Languages)

	b, err := loadBuffer("")
	if err != nil {
		fail(err)
		return
	}
	next, err := newEditor(cfg, b, dir)
	if err != nil {
		fail(err)
		return
	}
	next.hub = h
	next.regs, next.macros = e.regs, e.macros
	next.settingsPage, next.pickFolder = e.settingsPage, e.pickFolder
	h.live = append(h.live, next)
	if !h.show(next) {
		h.live = slices.DeleteFunc(h.live, func(o *editor) bool { return o == next })
		fail(fmt.Errorf("can't open %s", shortPath(dir)))
		return
	}
	next.resumeWork()
	next.startRecovery()
	if problem != nil {
		next.notify(problem.Error())
	} else {
		next.notify("Opened " + shortPath(dir))
	}
}

// resumeWork brings back a project's tabs, or its start page when it has none.
func (e *editor) resumeWork() {
	e.restoreSession(false)
	if !slices.ContainsFunc(e.buffers, func(b *buffer) bool { return b.path != "" }) {
		e.showDashboard()
		return
	}
	e.useTabWidth(e.b)
	e.scheduleLanguage()
	e.refreshGit()
}

// closeSession ends a session. With unsaved changes it refuses.
func (h *sessionHub) closeSession(e *editor) {
	if !slices.Contains(h.live, e) {
		return
	}
	if e.unsavedCount() > 0 {
		h.current.notify("Save or discard the changes in " + e.sessionName() + " before closing it")
		return
	}
	if e == h.current {
		i := slices.IndexFunc(h.live, func(o *editor) bool { return o != e })
		if i < 0 || !h.show(h.live[i]) {
			return
		}
	}
	// After show, which lists the session it leaves.
	if !e.noProject {
		forgetOpenSession(e.root)
	}
	h.drop(e)
}

func (h *sessionHub) drop(e *editor) {
	i := slices.Index(h.live, e)
	if i < 0 {
		return // show drops a blank one itself
	}
	h.live = slices.Delete(h.live, i, i+1)
	e.finish()
	for _, t := range []*time.Timer{e.languageTimer, e.gitTimer} {
		if t != nil {
			t.Stop()
		}
	}
	close(e.closed)
	// Work still running must not hang on reporting to a session that's gone.
	go func() {
		for {
			select {
			case <-e.async:
			case <-time.After(time.Minute):
				return
			}
		}
	}()
}

func (h *sessionHub) finish() {
	for _, e := range h.live {
		e.finish()
	}
}

// quitAll leaves Loom. With discard, unsaved changes don't come back.
func (h *sessionHub) quitAll(discard bool) {
	for _, e := range h.live {
		e.discardRecovery = discard
	}
	h.current.quit = true
}

// sessionsMenu lists the open sessions, the previous one first so that Enter
// flips between two, and below them the projects to start another from.
func (e *editor) sessionsMenu(input string, selected int) {
	h := e.hub
	if h == nil {
		return
	}
	m := &menu{title: "Sessions", input: input, remove: map[string]func(){},
		submit: func(path string) { h.current.openProject(path) },
		exact: func(input string) bool {
			return filepath.IsAbs(input) || strings.HasPrefix(input, "~") || strings.HasPrefix(input, ".")
		}}
	if h.window != nil {
		m.window = map[string]func(){}
	}
	listed := map[string]bool{}
	where := func(o *editor) string {
		if o.noProject {
			return "files outside a project"
		}
		return shortPath(o.root)
	}
	add := func(o *editor) {
		var state []string
		if o == h.current {
			state = append(state, "current")
		} else {
			state = append(state, "open")
		}
		if n := o.unsavedCount(); n > 0 {
			state = append(state, fmt.Sprintf("%d unsaved", n))
		}
		hint := strings.Join(append(state, where(o)), sessionSep)
		m.options = append(m.options, option{o.sessionName(), hint, func() { h.show(o) }, o.sessionPreview})
		if o == h.current {
			m.remove[hint] = func() { o.closeProject() }
		} else {
			m.remove[hint] = func() { h.closeSession(o) }
		}
		if m.window != nil {
			m.window[hint] = func() { h.moveToWindow(o) }
		}
		if !o.noProject {
			listed[o.root] = true
		}
	}
	for _, o := range h.live {
		if o != h.current {
			add(o)
		}
	}
	if !h.current.noProject {
		listed[h.current.root] = true
	}
	folder := func(root, state string) string {
		listed[root] = true
		hint := state + sessionSep + shortPath(root)
		m.options = append(m.options, option{filepath.Base(root), hint, func() { h.current.openProject(root) }, func() itemPreview { return directoryPreview(root) }})
		if m.window != nil {
			m.window[hint] = func() { h.openWindow(root) }
		}
		return hint
	}
	for _, root := range savedSessions() {
		if !listed[root] {
			hint := folder(root, "open")
			m.remove[hint] = func() { forgetOpenSession(root) }
		}
	}
	if !h.current.blank() {
		add(h.current)
	}
	m.options = append(m.options, option{"New Session…", "", func() { h.current.chooseProject() }, nil})
	for _, root := range recentProjects() {
		if !listed[root] {
			folder(root, "recent")
		}
	}
	e.openMenu(m)
	m.selected = max(0, min(selected, len(m.matches)-1))
}

const sessionSep = " · "

func (e *editor) sessionPreview() itemPreview {
	p := itemPreview{title: e.sessionName()}
	if !e.noProject {
		p.title = e.root
	}
	var lines []string
	for _, b := range e.buffers {
		if b.dashboard {
			continue
		}
		name := b.path
		if name == "" {
			if !b.dirty() {
				continue
			}
			name = "Untitled"
		} else if rel, err := filepath.Rel(e.root, name); err == nil && within(e.root, name) {
			name = rel
		}
		if b.dirty() {
			name += "  • unsaved"
		}
		lines = append(lines, name)
	}
	p.message = strings.Join(lines, "\n")
	if len(lines) == 0 {
		p.message = "No files open"
	}
	return p
}

// removeSelected closes the selected session and keeps the list up.
func (e *editor) removeSelected() {
	m := e.menu
	if m.selected >= len(m.matches) {
		return
	}
	remove := m.remove[m.matches[m.selected].hint]
	if remove == nil {
		return
	}
	h, input, at := e.hub, m.input, m.selected
	e.menu = nil
	remove()
	h.current.sessionsMenu(input, at)
}

// windowSelected opens the selected session in a new window.
func (e *editor) windowSelected() {
	m := e.menu
	if m.selected >= len(m.matches) {
		return
	}
	if open := m.window[m.matches[m.selected].hint]; open != nil {
		e.menu = nil
		open()
	}
}
