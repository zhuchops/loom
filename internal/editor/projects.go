package editor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

const projectLimit = 20

func lastProjectPath() string { return filepath.Join(filepath.Dir(configPath()), "last-project") }
func projectsPath() string    { return filepath.Join(filepath.Dir(configPath()), "projects") }

func readLines(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// lastProject is the project that was open when loom last ran, or "" when
// none was (or it has since disappeared).
func lastProject() string {
	lines := readLines(lastProjectPath())
	if len(lines) == 0 {
		return ""
	}
	if info, err := os.Stat(lines[0]); err != nil || !info.IsDir() {
		return ""
	}
	return lines[0]
}

func recentProjects() []string {
	var out []string
	for _, p := range readLines(projectsPath()) {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// rememberProject makes root the project loom reopens by default.
func rememberProject(root string) {
	list := slices.DeleteFunc(readLines(projectsPath()), func(p string) bool { return p == root })
	list = append([]string{root}, list...)
	if len(list) > projectLimit {
		list = list[:projectLimit]
	}
	if os.MkdirAll(filepath.Dir(projectsPath()), 0o755) != nil {
		return
	}
	_ = os.WriteFile(projectsPath(), []byte(strings.Join(list, "\n")+"\n"), 0o644)
	_ = os.WriteFile(lastProjectPath(), []byte(root+"\n"), 0o644)
}

func forgetLastProject() { _ = os.Remove(lastProjectPath()) }

// shortPath abbreviates the home directory the way a shell prompt does.
func shortPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && within(home, path) {
		if rel, err := filepath.Rel(home, path); err == nil {
			if rel == "." {
				return "~"
			}
			return "~/" + rel
		}
	}
	return path
}

// enterNoProject puts the editor in its start state: no folder is open, so
// project commands are unavailable until one is.
func (e *editor) enterNoProject() {
	e.noProject = true
	e.gitToken++ // drop any git result still on its way
	e.tree.sidebar = false
	e.tree.rows = nil
	e.git = gitState{}
}

// resetWorkspace tears down everything tied to the current project. It
// refuses while there are unsaved changes.
func (e *editor) resetWorkspace() bool {
	if e.directory != nil && e.directory.doc.b.dirty() {
		e.notify("Write or discard directory edits before switching projects")
		return false
	}
	for _, b := range e.buffers {
		if b.dirty() && !b.dashboard {
			e.notify("Save or discard unsaved buffers before switching projects")
			return false
		}
	}
	if !e.noProject {
		e.saveSession()
	}
	e.invalidateCompletion()
	e.closeServers()
	e.sessions = make(map[string]*languageSession)
	e.problems = make(map[string][]diagnostic)
	e.problemServers = nil
	e.directory, e.menu, e.panel, e.diff = nil, nil, nil, nil
	e.jumps = jumpHistory{}
	e.git = gitState{}
	b, _ := loadBuffer("")
	e.buffers = []*buffer{b}
	e.b = b
	e.layout = &pane{b: b, cursor: b.cursor, top: b.top, left: b.left}
	e.activePane = e.layout
	e.mode = "normal"
	e.dashboardSelection = 0
	return true
}

func (e *editor) openProject(dir string) {
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(dir, "~"), "/"))
	} else if !filepath.IsAbs(dir) {
		dir = filepath.Join(e.root, dir)
	}
	dir = filepath.Clean(dir)
	if canonical, err := filepath.EvalSymlinks(dir); err == nil {
		dir = canonical
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		e.notify("Not a folder: " + dir)
		return
	}
	if !e.noProject && dir == e.root {
		e.showDashboard()
		return
	}
	if !e.resetWorkspace() {
		return
	}
	if err := os.Chdir(dir); err != nil {
		e.notify(err.Error())
	}
	e.root, e.noProject = dir, false
	e.followConfig()
	e.tree = fileTree{sidebar: true, width: 28, expanded: map[string]bool{dir: true}}
	e.refreshTree("")
	e.restoreSession(false)
	if e.recovery != nil {
		// Crash recovery is kept per project.
		_ = e.saveRecovery()
		e.startRecovery()
	}
	e.showDashboard()
	e.refreshGit()
	rememberProject(dir)
	e.notify("Opened " + shortPath(dir))
}

func (e *editor) closeProject() {
	if e.noProject {
		return
	}
	if !e.resetWorkspace() {
		return
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/"
	}
	_ = os.Chdir(home)
	e.root = home
	e.enterNoProject()
	e.followConfig()
	forgetLastProject()
	e.showDashboard()
}

// projectMenu lists remembered projects and lets you type or browse to a new
// one.
func (e *editor) projectMenu() {
	m := &menu{title: "Open Project", submit: e.openProject, exact: func(input string) bool {
		return filepath.IsAbs(input) || strings.HasPrefix(input, "~") || strings.HasPrefix(input, ".")
	}}
	start := e.root
	if e.noProject {
		start, _ = os.UserHomeDir()
	}
	m.options = append(m.options, option{"Browse in Loom…", shortPath(start), func() { e.browseProjects(start) }, nil})
	for _, p := range recentProjects() {
		if !e.noProject && p == e.root {
			continue
		}
		p := p
		m.options = append(m.options, option{filepath.Base(p) + "/", shortPath(p), func() { e.openProject(p) }, func() itemPreview { return directoryPreview(p) }})
	}
	e.openMenu(m)
}

// browseProjects walks the folder tree one level at a time.
func (e *editor) browseProjects(dir string) {
	m := &menu{title: "Open Project · " + shortPath(dir), submit: e.openProject, exact: func(input string) bool {
		return filepath.IsAbs(input) || strings.HasPrefix(input, "~") || strings.HasPrefix(input, ".")
	}}
	m.options = append(m.options, option{"Open this folder", shortPath(dir), func() { e.openProject(dir) }, func() itemPreview { return directoryPreview(dir) }})
	if parent := filepath.Dir(dir); parent != dir {
		m.options = append(m.options, option{"../", shortPath(parent), func() { e.browseProjects(parent) }, nil})
	}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !entry.IsDir() {
			if !entry.IsDir() && entry.Type()&os.ModeSymlink != 0 {
				if info, err := os.Stat(filepath.Join(dir, name)); err != nil || !info.IsDir() {
					continue
				}
			} else {
				continue
			}
		}
		child := filepath.Join(dir, name)
		m.options = append(m.options, option{name + "/", "browse", func() { e.browseProjects(child) }, func() itemPreview { return directoryPreview(child) }})
	}
	e.openMenu(m)
}

// needsProject wraps commands that only make sense inside a folder.
func (e *editor) needsProject(ids ...string) {
	for _, id := range ids {
		c, ok := e.commands[id]
		if !ok {
			continue
		}
		run := c.run
		e.commands[id] = command{c.title, func(e *editor) {
			if e.noProject {
				e.notify("No project open. Use Open Project first")
				return
			}
			run(e)
		}}
	}
}

// chooseProject asks for a folder with the system dialog (Finder on macOS).
// Elsewhere, or when the dialog is unavailable, it falls back to the menu.
func (e *editor) chooseProject() {
	start := e.root
	if e.noProject {
		start, _ = os.UserHomeDir()
	}
	if e.pickFolder != nil {
		e.pickFolder(start, e.openProject)
		return
	}
	if runtime.GOOS != "darwin" {
		e.projectMenu()
		return
	}
	e.notify("Choose a folder in Finder…")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		script := `POSIX path of (choose folder with prompt "Open Project"` + ` default location (POSIX file ` + appleQuote(start) + `))`
		out, err := exec.CommandContext(ctx, "osascript", "-e", script).Output()
		path := strings.TrimSpace(string(out))
		e.async <- func() {
			switch {
			case err == nil && path != "":
				e.openProject(path)
			case err != nil:
				var exit *exec.ExitError
				if !errors.As(err, &exit) { // a cancelled dialog is not an error worth showing
					e.projectMenu()
				}
			}
		}
	}()
}

func appleQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// followConfig applies the config of the project just switched to, and drops
// what the last one set.
func (e *editor) followConfig() {
	cfg, err := loadConfig(e.cfg.path, e.cfg.explicit, e.configRoot())
	if err != nil {
		e.notify(err.Error())
		// Keep going with just the user's settings, not the last project's.
		if cfg, err = readConfig(e.cfg.path, e.cfg.explicit); err != nil {
			return
		}
	}
	cfg.Profile = e.cfg.Profile
	keys, err := compileKeys(cfg, e.commands)
	if err == nil {
		err = e.useConfig(cfg, keys)
	}
	if err != nil {
		e.notify("Config: " + err.Error())
		return
	}
	e.relativeNumbers = cfg.RelativeNumbers
}
