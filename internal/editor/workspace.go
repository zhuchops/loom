package editor

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func (e *editor) workspaceCommands() {
	e.add("split-join", "Toggle Split Join", func(e *editor) { e.toggleSplitJoin() })
	e.add("open-project", "Open Project", func(e *editor) { e.projectMenu() })
	e.add("close-project", "Close Project", func(e *editor) { e.closeProject() })
	e.add("dashboard", "Open Dashboard", func(e *editor) { e.showDashboard() })
	e.add("explorer-popup", "Open Explorer Popup", func(e *editor) { e.openDirectory("") })
	e.add("toggle-sidebar", "Toggle Sidebar", func(e *editor) {
		if e.tree.sidebar {
			e.hideExplorer()
		} else {
			e.tree.sidebar = true
		}
	})
	e.add("directory-jump", "Jump to Directory", func(e *editor) { e.directoryPicker() })
	e.layoutCommands()
	e.add("close-tab", "Close Tab", func(e *editor) { e.closeBuffer(e.b) })
	e.add("project-search", "Search Project", func(e *editor) { e.projectSearchPrompt("") })
	e.add("refresh-git", "Refresh Git Status", func(e *editor) { e.refreshGit() })
	e.add("format", "Format Buffer", func(e *editor) {
		if err := e.formatBuffer(); err != nil {
			e.notify(err.Error())
		}
	})
	e.add("repeat", "Repeat Last Change", func(e *editor) { e.repeatChange() })
	e.add("recover-buffers", "Recover Unsaved Buffers", func(e *editor) { e.offerRecovery() })
	e.add("jump-back", "Jump Back", func(e *editor) { e.travelJump(false) })
	e.add("jump-forward", "Jump Forward", func(e *editor) { e.travelJump(true) })
	e.languageCommands()
	e.gitCommands()
}

func (e *editor) closeBuffer(b *buffer) {
	if b.dirty() {
		e.notify("Save this tab before closing it")
		return
	}
	at := slices.Index(e.buffers, b)
	if at < 0 {
		return
	}
	if b == e.b {
		e.normal()
	}
	e.dropPreviews(b)
	e.buffers = slices.Delete(e.buffers, at, at+1)
	if len(e.buffers) == 0 {
		blank, _ := loadBuffer("")
		e.buffers = []*buffer{blank}
	}
	next := e.buffers[min(at, len(e.buffers)-1)]
	for _, p := range e.layout.leaves() {
		if p.dashboardPrevious != nil && p.dashboardPrevious.b == b {
			p.dashboardPrevious = nil
		}
		if p.b == b {
			p.b, p.cursor, p.top, p.left = next, next.cursor, next.top, next.left
			if previous := p.dashboardPrevious; previous != nil {
				p.b, p.cursor, p.top, p.left = previous.b, min(previous.cursor, len(previous.b.text)), previous.top, previous.left
				p.dashboardPrevious = nil
			}
		}
	}
	if b == e.b {
		e.b = e.activePane.b
		e.b.cursor, e.b.top, e.b.left = e.activePane.cursor, e.activePane.top, e.activePane.left
		e.afterBufferSwitch()
	}
	e.closeDocument(b.path)
}

func (e *editor) closePath(path string) {
	for _, b := range slices.Clone(e.buffers) {
		if b.path != "" && within(path, b.path) {
			e.closeBuffer(b)
		}
	}
}

func (e *editor) formatBuffer() error {
	b := e.b
	lang := e.cfg.languageFor(b.path)
	old := string(b.text)
	if lang == "go" {
		var source []byte
		var err error
		if e.cfg.OrganizeImports {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "goimports", "-srcdir", b.path)
			cmd.Stdin = strings.NewReader(old)
			source, err = cmd.Output()
		} else {
			source, err = format.Source([]byte(old))
		}
		if err != nil {
			return fmt.Errorf("format: %w", err)
		}
		e.replaceFormatted(old, string(source))
		return nil
	}
	argv := e.cfg.Languages[lang].Format
	if len(argv) == 0 {
		return nil
	}
	// A missing formatter is skipped so that saving never fails for lack of a tool.
	if _, err := exec.LookPath(argv[0]); err != nil {
		return nil
	}
	args := make([]string, 0, len(argv)-1)
	for _, arg := range argv[1:] {
		args = append(args, strings.ReplaceAll(arg, "{path}", b.path))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], args...)
	cmd.Stdin = strings.NewReader(old)
	source, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("format with %s: %w", argv[0], err)
	}
	e.replaceFormatted(old, string(source))
	return nil
}

func (e *editor) replaceFormatted(old, formatted string) {
	b := e.b
	if b.newline() == "\r\n" {
		formatted = strings.ReplaceAll(formatted, "\r\n", "\n")
		formatted = strings.ReplaceAll(formatted, "\n", "\r\n")
	}
	if formatted == old {
		return
	}
	row := b.rowAt(b.cursor)
	oldCol := b.cursor - b.lines[row]
	b.replace(span{0, len(b.text)}, formatted)
	row = min(row, len(b.lines)-1)
	col := min(oldCol, b.lineEnd(row)-b.lines[row])
	b.cursor = b.lines[row] + col
	e.clampModalCursor()
	e.scheduleLanguage()
}

func (e *editor) projectSearchPrompt(initial string) {
	e.openMenu(&menu{title: "Search Project · literal text", input: initial, submit: func(query string) {
		if query != "" {
			e.projectSearch(query)
		}
	}})
}

func (e *editor) projectSearch(query string) {
	m := &menu{title: "Search: " + query}
	e.openMenu(m)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, "rg", "--no-config", "--sort", "path", "--json", "--fixed-strings", "--color", "never", "--", query, ".")
		cmd.Dir = e.root
		out, err := cmd.StdoutPipe()
		if err == nil {
			err = cmd.Start()
		}
		var options []option
		if err == nil {
			scanner := bufio.NewScanner(out)
			scanner.Buffer(make([]byte, 65536), 2<<20)
			for len(options) < 1000 && scanner.Scan() {
				var event struct {
					Type string
					Data struct {
						Path, Lines struct{ Text string }
						Line        int                   `json:"line_number"`
						Matches     []struct{ Start int } `json:"submatches"`
					}
				}
				if json.Unmarshal(scanner.Bytes(), &event) != nil || event.Type != "match" || len(event.Data.Matches) == 0 {
					continue
				}
				data := event.Data
				path := filepath.Join(e.root, data.Path.Text)
				label := fmt.Sprintf("%s:%d", strings.TrimPrefix(data.Path.Text, "./"), data.Line)
				col := utf8.RuneCountInString(data.Lines.Text[:min(data.Matches[0].Start, len(data.Lines.Text))])
				options = append(options, option{label, strings.TrimSpace(data.Lines.Text), func() { e.jumpTo(path, data.Line-1, col, false) }, e.filePreview(path, data.Line-1, col, false, nil)})
			}
			limited := len(options) >= 1000
			if limited || scanner.Err() != nil {
				_ = cmd.Process.Kill()
			}
			waitErr := cmd.Wait()
			err = scanner.Err()
			if err == nil && waitErr != nil && !limited && ctx.Err() == nil {
				if exit, ok := waitErr.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
					err = waitErr
				}
			}
		}
		// Capture the result before canceling. The UI callback runs later.
		timedOut := ctx.Err() == context.DeadlineExceeded
		cancel()
		e.async <- func() {
			if e.menu != m {
				return
			}
			m.options = options
			m.filter()
			switch {
			case err != nil:
				e.notify("Project search: " + err.Error())
			case timedOut:
				e.notify("Search timed out")
			case len(options) == 0:
				e.notify("No project matches")
			case len(options) == 1000:
				e.notify("Showing first 1,000 matching lines")
			}
		}
	}()
}

func (e *editor) jumpTo(path string, row, col int, utf16 bool) {
	from := e.jumpPoint()
	if !e.open(path) {
		return
	}
	row = max(0, min(len(e.b.lines)-1, row))
	if utf16 {
		offset, ok := e.b.offset(position{row, col})
		if ok {
			e.b.cursor = offset
		} else {
			e.b.cursor = e.b.lines[row]
		}
	} else {
		e.b.cursor = min(e.b.lines[row]+max(0, col), e.b.lineEnd(row))
	}
	e.clampModalCursor()
	e.scrollLock = false
	e.rememberJump(from)
}

type gitState struct {
	files   map[string]byte
	lines   map[int]byte
	path    string
	version int
	branch  string
}

func (e *editor) scheduleGit() {
	if e.gitTimer != nil {
		e.gitTimer.Stop()
	}
	if e.b.path == "" {
		return
	}
	e.gitTimer = time.AfterFunc(600*time.Millisecond, func() {
		select {
		case e.gitWake <- struct{}{}:
		default:
		}
	})
}

func (e *editor) refreshGit() {
	e.gitToken++
	token := e.gitToken
	b := e.b
	path, version, text := b.path, b.version, string(b.text)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		run := func(args ...string) []byte {
			c := exec.CommandContext(ctx, "git", args...)
			c.Dir = e.root
			c.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
			out, _ := c.Output()
			return out
		}
		files := map[string]byte{}
		root := strings.TrimSuffix(string(run("rev-parse", "--show-toplevel")), "\n")
		entries := strings.Split(string(run("status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames")), "\x00")
		for _, entry := range entries {
			if len(entry) < 4 || root == "" {
				continue
			}
			p := filepath.Join(root, entry[3:])
			state := entry[1]
			if state == ' ' {
				state = entry[0]
			}
			for within(e.root, p) {
				if files[p] == 0 || files[p] == '?' {
					files[p] = state
				}
				if p == e.root {
					break
				}
				p = filepath.Dir(p)
			}
		}
		lines := map[int]byte{}
		if path != "" && root != "" && within(root, path) {
			rel, _ := filepath.Rel(root, path)
			old := run("show", "HEAD:"+filepath.ToSlash(rel))
			if !strings.ContainsRune(text, 0) && len(text) < 2<<20 {
				lines = changedLines(string(old), text)
			}
		}
		branch := gitBranch(e.root)
		e.async <- func() {
			if token == e.gitToken {
				e.git = gitState{files: files, lines: lines, path: path, version: version, branch: branch}
			}
		}
	}()
}

func changedLines(old, current string) map[int]byte {
	if old == current {
		return nil
	}
	b := strings.Split(current, "\n")
	result := map[int]byte{}
	// Diff snapshots through Git's line matcher, including unsaved edits.
	dir, err := os.MkdirTemp("", "loom-diff-")
	if err != nil {
		return result
	}
	defer os.RemoveAll(dir)
	first, second := filepath.Join(dir, "old"), filepath.Join(dir, "new")
	if os.WriteFile(first, []byte(old), 0600) != nil || os.WriteFile(second, []byte(current), 0600) != nil {
		return result
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "diff", "--no-index", "--no-ext-diff", "--no-textconv", "--no-color", "--unified=0", "--", first, second)
	out, err := cmd.Output()
	if exit, ok := err.(*exec.ExitError); err != nil && (!ok || exit.ExitCode() != 1) {
		return result
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pos, n, has := strings.Cut(strings.TrimPrefix(fields[2], "+"), ",")
		start, err := strconv.Atoi(pos)
		if err != nil {
			continue
		}
		count := 1
		if has {
			count, _ = strconv.Atoi(n)
		}
		marker := byte('+')
		if !strings.HasSuffix(fields[1], ",0") {
			marker = '~'
		}
		if count == 0 {
			result[max(0, min(len(b)-1, start-1))] = '-'
		}
		for i := max(0, start-1); i < start-1+count && i < len(b); i++ {
			result[i] = marker
		}
	}
	return result
}

type savedTab struct {
	Path              string
	Cursor, Top, Left int
}
type savedSession struct {
	Layout          *savedPane
	RelativeNumbers *bool
	Tabs            []savedTab
	Active, Other   string
	Right, Sidebar  bool
	Width           int
}

func (e *editor) sessionPath() string {
	root := os.Getenv("XDG_STATE_HOME")
	if root == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, ".local", "state")
	}
	if root == "" || root == ".local/state" {
		return ""
	}
	return filepath.Join(root, "loom", fmt.Sprintf("%x.json", sha256.Sum256([]byte(e.root))))
}

func (e *editor) restoreSession(explicit bool) {
	if !e.cfg.RestoreSession || e.noProject {
		return
	}
	raw, err := os.ReadFile(e.sessionPath())
	if err != nil {
		return
	}
	var s savedSession
	if json.Unmarshal(raw, &s) != nil {
		return
	}
	e.tree.sidebar, e.tree.width = s.Sidebar, max(16, s.Width)
	initial := e.b
	e.buffers = nil
	byPath := map[string]*buffer{}
	for _, tab := range s.Tabs {
		if !filepath.IsAbs(tab.Path) || byPath[tab.Path] != nil {
			continue
		}
		if info, err := os.Stat(tab.Path); err != nil || !info.Mode().IsRegular() {
			continue
		}
		b, err := loadBuffer(tab.Path)
		if err != nil {
			continue
		}
		b.cursor, b.top, b.left = max(0, min(len(b.text), tab.Cursor)), max(0, min(tab.Top, len(b.lines)-1)), max(0, tab.Left)
		if explicit && initial.path == tab.Path {
			initial.cursor, initial.top, initial.left = b.cursor, b.top, b.left
			b = initial
		}
		e.buffers = append(e.buffers, b)
		byPath[tab.Path] = b
	}
	if explicit || len(e.buffers) == 0 {
		e.b = initial
		if byPath[initial.path] == nil {
			e.buffers = append(e.buffers, initial)
		}
	} else if b := byPath[s.Active]; b != nil {
		e.b = b
	} else {
		e.b = e.buffers[0]
	}
	if s.RelativeNumbers != nil {
		e.relativeNumbers = *s.RelativeNumbers
	}
	e.layout, e.activePane = nil, nil
	if !explicit {
		e.layout, e.activePane = restorePaneLayout(s.Layout, byPath, 0)
		if e.layout == nil && byPath[s.Other] != nil && byPath[s.Other] != e.b {
			other, active := &pane{b: byPath[s.Other]}, &pane{b: e.b, cursor: e.b.cursor, top: e.b.top, left: e.b.left}
			e.layout = &pane{first: active, second: other, vertical: true}
			if s.Right {
				e.layout.first, e.layout.second = other, active
			}
			e.activePane = active
		}
	}
	if e.layout == nil {
		e.layout = &pane{b: e.b, cursor: e.b.cursor, top: e.b.top, left: e.b.left}
	}
	if e.activePane == nil {
		e.activePane = e.layout.leaves()[0]
	}
	e.b = e.activePane.b
	e.b.cursor, e.b.top, e.b.left = e.activePane.cursor, e.activePane.top, e.activePane.left
	e.normal()
	e.revealFile()
}

func (e *editor) saveSession() {
	if !e.cfg.RestoreSession || e.noProject {
		return
	}
	e.rememberPane()
	s := savedSession{Active: e.b.path, Sidebar: e.tree.sidebar, Width: e.tree.width, Layout: savePaneLayout(e.layout, e.activePane), RelativeNumbers: &e.relativeNumbers}
	for _, b := range e.buffers {
		if b.path == "" {
			continue
		}
		if _, err := os.Stat(b.path); err != nil {
			continue
		}
		s.Tabs = append(s.Tabs, savedTab{b.path, b.cursor, b.top, b.left})
	}
	path := e.sessionPath()
	if path == "" || os.MkdirAll(filepath.Dir(path), 0700) != nil {
		return
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".session-")
	if err != nil {
		return
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(s)
	closeErr := f.Close()
	if err == nil && closeErr == nil {
		_ = os.Rename(f.Name(), path)
	}
}

type changeState struct {
	inputs, last       []keyInput
	changed, repeating bool
}

func (e *editor) repeatChange() {
	if e.change.repeating || len(e.change.last) == 0 {
		return
	}
	keys := slices.Clone(e.change.last)
	b, version := e.b, e.b.version
	initial := b.snapshot()
	history := slices.Clone(b.undo)
	e.change.repeating = true
	defer func() {
		e.change.repeating = false
		if e.b == b && b.version != version {
			b.undo = pushHistory(history, initial)
		}
	}()
	for range min(100, e.takeCount()) {
		for _, in := range keys {
			e.handle(in)
		}
	}
}

func (e *editor) trackChange(in keyInput, b *buffer, version int, before string, undoLen int, now time.Time) {
	if e.replaying || e.change.repeating || in.key == "." || e.b != b {
		return
	}
	state := &e.change
	edited := b.version != version && len(b.undo) >= undoLen && (before == "insert" || in.key != "u" && in.key != "ctrl+r")

	if before == "insert" && e.mode == "insert" && edited {
		b.lastInsert = now
	} else {
		b.lastInsert = time.Time{}
	}
	state.inputs = append(state.inputs, in)
	state.changed = state.changed || edited
	if len(state.inputs) > 20000 {
		state.inputs = nil
		state.changed = false
	}
	if e.mode != "insert" && e.mode != "select" && e.prefix == "" && e.pending.kind == "" && e.count == 0 {
		if state.changed {
			state.last = slices.Clone(state.inputs)
		}
		state.inputs = state.inputs[:0]
		state.changed = false
	}
}
