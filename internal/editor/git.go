package editor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type gitChange struct {
	path, rel   string
	index, work byte
}

// sourceControl is the git panel popup. Rows are grouped into Staged, Changes, and Untracked.
type sourceControl struct {
	top      string
	branch   string
	changes  []gitChange
	selected int
	scroll   int
	token    int
}

type gitRow struct {
	header string
	change gitChange
	staged bool
}

type gitHunk struct{ line, start int }

// diffView shows `git diff` for one file. Hunks are the lines that start with "@@".
type diffView struct {
	title   string
	path    string
	lines   []string
	hunks   []gitHunk
	cursor  int
	scroll  int
	height  int
	pending string
}

// gitExec returns stderr on failure so callers can show the full error.
func gitExec(dir string, timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return out, errors.New(msg)
	}
	return out, nil
}

func gitBranch(dir string) string {
	out, err := gitExec(dir, 5*time.Second, "branch", "--show-current")
	if err == nil && strings.TrimSpace(string(out)) != "" {
		return strings.TrimSpace(string(out))
	}
	out, err = gitExec(dir, 5*time.Second, "rev-parse", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func parseGitStatus(top, out string) []gitChange {
	var changes []gitChange
	for _, entry := range strings.Split(out, "\x00") {
		if len(entry) < 4 {
			continue
		}
		rel := entry[3:]
		changes = append(changes, gitChange{path: filepath.Join(top, rel), rel: rel, index: entry[0], work: entry[1]})
	}
	return changes
}

func (p *sourceControl) entries() []gitRow {
	var staged, changed, untracked []gitChange
	for _, c := range p.changes {
		if c.index == '?' {
			untracked = append(untracked, c)
			continue
		}
		if c.index != ' ' {
			staged = append(staged, c)
		}
		if c.work != ' ' {
			changed = append(changed, c)
		}
	}
	var rows []gitRow
	section := func(title string, items []gitChange, isStaged bool) {
		if len(items) == 0 {
			return
		}
		rows = append(rows, gitRow{header: fmt.Sprintf("%s (%d)", title, len(items))})
		for _, c := range items {
			rows = append(rows, gitRow{change: c, staged: isStaged})
		}
	}
	section("Staged", staged, true)
	section("Changes", changed, false)
	section("Untracked", untracked, false)
	return rows
}

func (p *sourceControl) move(delta int) {
	rows := p.entries()
	i := p.selected
	for next := i + delta; next >= 0 && next < len(rows); next += delta {
		i = next
		if rows[i].header == "" {
			break
		}
	}
	p.selected = i
}

func (p *sourceControl) normalize() {
	rows := p.entries()
	if len(rows) == 0 {
		p.selected = 0
		return
	}
	p.selected = max(0, min(p.selected, len(rows)-1))
	if rows[p.selected].header != "" {
		p.move(1)
		if rows[p.selected].header != "" {
			p.move(-1)
		}
	}
}

func (p *sourceControl) current() (gitRow, bool) {
	rows := p.entries()
	if p.selected < 0 || p.selected >= len(rows) || rows[p.selected].header != "" {
		return gitRow{}, false
	}
	return rows[p.selected], true
}

func (p *sourceControl) hasStaged() bool {
	for _, c := range p.changes {
		if c.index != ' ' && c.index != '?' {
			return true
		}
	}
	return false
}

func (e *editor) gitCommands() {
	e.add("git-panel", "Source Control", func(e *editor) { e.openSourceControl() })
	e.add("git-diff", "Show Git Diff", func(e *editor) { e.openDiffFor(e.b.path) })
	e.add("git-branch", "Switch Branch", func(e *editor) { e.openBranchPicker() })
}

func (e *editor) openSourceControl() {
	e.panel = &sourceControl{}
	e.reloadSourceControl()
}

// reloadSourceControl reads git status in the background and applies it if the panel is still open.
func (e *editor) reloadSourceControl() {
	p := e.panel
	if p == nil {
		return
	}
	p.token++
	token := p.token
	root := e.root
	go func() {
		result := &sourceControl{}
		out, err := gitExec(root, 5*time.Second, "rev-parse", "--show-toplevel")
		if err == nil {
			result.top = strings.TrimSpace(string(out))
			result.branch = gitBranch(result.top)
			status, err2 := gitExec(result.top, 5*time.Second, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames")
			err = err2
			result.changes = parseGitStatus(result.top, string(status))
		}
		e.async <- func() {
			if e.panel != p || p.token != token {
				return
			}
			if err != nil {
				e.panel = nil
				e.notify(err.Error())
				return
			}
			p.top, p.branch, p.changes = result.top, result.branch, result.changes
			p.normalize()
		}
	}()
}

// gitMutate runs a local git change and refreshes the panel and explorer marks.
func (e *editor) gitMutate(dir string, args ...string) bool {
	if _, err := gitExec(dir, 10*time.Second, args...); err != nil {
		e.notify(err.Error())
		return false
	}
	e.reloadSourceControl()
	e.refreshGit()
	return true
}

func (e *editor) gitRemote(dir string, args ...string) {
	e.notify("git " + args[0] + "…")
	go func() {
		out, err := gitExec(dir, 3*time.Minute, args...)
		e.async <- func() {
			if err != nil {
				e.notify(err.Error())
				return
			}
			message := strings.TrimSpace(string(out))
			if message == "" {
				message = "git " + args[0] + " finished"
			}
			e.notify(message)
			e.reloadSourceControl()
			e.refreshGit()
		}
	}()
}

func (e *editor) sourceControlInput(in keyInput) {
	p := e.panel
	if p.top == "" && in.key != "esc" && in.key != "q" && in.key != "R" {
		return
	}
	switch in.key {
	case "esc", "q":
		e.panel = nil
	case "j", "down":
		p.move(1)
	case "k", "up":
		p.move(-1)
	case "g", "home":
		p.selected = 0
		p.normalize()
	case "G", "end":
		p.selected = len(p.entries()) - 1
		p.normalize()
	case "s", "space":
		row, ok := p.current()
		if !ok {
			break
		}
		if row.staged {
			e.gitMutate(p.top, "reset", "-q", "--", row.change.path)
		} else {
			e.gitMutate(p.top, "add", "--", row.change.path)
		}
	case "a":
		e.gitMutate(p.top, "add", "-A")
	case "A":
		e.gitMutate(p.top, "reset", "-q")
	case "d":
		e.discardRow(p)
	case "enter":
		if row, ok := p.current(); ok {
			e.openDiffFor(row.change.path)
		}
	case "c":
		e.commitPrompt(p)
	case "p":
		e.gitRemote(p.top, "push")
	case "P":
		e.gitRemote(p.top, "pull", "--ff-only")
	case "b":
		e.openBranchPicker()
	case "R":
		e.reloadSourceControl()
		e.refreshGit()
	}
}

func (e *editor) discardRow(p *sourceControl) {
	row, ok := p.current()
	if !ok {
		return
	}
	c := row.change
	switch {
	case row.staged:
		e.notify("Unstage with s before discarding")
		return
	case c.index == '?':
		e.notify("Delete untracked files from the explorer")
		return
	}
	for _, b := range e.buffers {
		if b.path == c.path && b.dirty() {
			e.notify("Save the open tab before discarding")
			return
		}
	}
	e.openMenu(&menu{title: "Discard changes to " + c.rel + "? Type yes", submit: func(input string) {
		if input != "yes" {
			return
		}
		if e.gitMutate(p.top, "restore", "--", c.path) {
			e.notify("Discarded " + c.rel)
		}
	}})
}

func (e *editor) commitPrompt(p *sourceControl) {
	if !p.hasStaged() {
		e.notify("Nothing staged to commit")
		return
	}
	e.openMenu(&menu{title: "Commit message", submit: func(message string) {
		message = strings.TrimSpace(message)
		if message == "" {
			e.notify("Commit message is empty")
			return
		}
		out, err := gitExec(p.top, 30*time.Second, "commit", "-m", message)
		if err != nil {
			e.notify(err.Error())
			return
		}
		e.notify(strings.TrimSpace(string(out)))
		e.reloadSourceControl()
		e.refreshGit()
	}})
}

func (e *editor) openBranchPicker() {
	dir := e.root
	if e.panel != nil && e.panel.top != "" {
		dir = e.panel.top
	}
	out, err := gitExec(dir, 5*time.Second, "branch", "--format=%(refname:short)")
	if err != nil {
		e.notify(err.Error())
		return
	}
	current := gitBranch(dir)
	m := &menu{title: "Switch Branch"}
	for _, name := range strings.Fields(string(out)) {
		hint := ""
		if name == current {
			hint = "current"
		}
		m.options = append(m.options, option{name, hint, func() { e.switchBranch(dir, name) }, nil})
	}
	e.openMenu(m)
}

func (e *editor) switchBranch(dir, name string) {
	out, err := gitExec(dir, 10*time.Second, "switch", name)
	if err != nil {
		e.notify(err.Error())
		return
	}
	message := strings.TrimSpace(string(out))
	if message == "" {
		message = "Switched to " + name
	}
	e.notify(message)
	e.reloadSourceControl()
	e.refreshGit()
}

// gitDiffText returns the diff of one file against HEAD, or against nothing for untracked files.
func gitDiffText(path string) (string, error) {
	dir, name := filepath.Dir(path), filepath.Base(path)
	const timeout = 10 * time.Second
	if _, err := gitExec(dir, timeout, "rev-parse", "--show-toplevel"); err != nil {
		return "", err
	}
	if _, err := gitExec(dir, timeout, "ls-files", "--error-unmatch", "--", name); err != nil {
		out, err := gitExec(dir, timeout, "diff", "--no-index", "--no-color", "--no-ext-diff", "--", os.DevNull, name)
		if err != nil && len(out) == 0 {
			return "", err
		}
		return string(out), nil
	}
	out, err := gitExec(dir, timeout, "diff", "--no-color", "--no-ext-diff", "HEAD", "--", name)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (e *editor) openDiffFor(path string) {
	if path == "" {
		e.notify("Save this file before viewing its diff")
		return
	}
	text, err := gitDiffText(path)
	if err != nil {
		e.notify(err.Error())
		return
	}
	title := filepath.Base(path)
	if rel, err := filepath.Rel(e.root, path); err == nil && within(e.root, path) {
		title = rel
	}
	e.diff = newDiffView("Diff · "+title, path, text)
}

func newDiffView(title, path, text string) *diffView {
	d := &diffView{title: title, path: path, lines: strings.Split(strings.TrimRight(text, "\n"), "\n")}
	if strings.TrimSpace(text) == "" {
		d.lines = []string{"No changes against HEAD."}
	}
	// The title already names the file, so skip the git header before the first hunk.
	for i, line := range d.lines {
		if strings.HasPrefix(line, "@@ ") {
			d.lines = d.lines[i:]
			break
		}
	}
	for i, line := range d.lines {
		fields := strings.Fields(line)
		if !strings.HasPrefix(line, "@@ ") || len(fields) < 3 {
			continue
		}
		start, _, _ := strings.Cut(strings.TrimPrefix(fields[2], "+"), ",")
		if n, err := strconv.Atoi(start); err == nil {
			d.hunks = append(d.hunks, gitHunk{line: i, start: n})
		}
	}
	if len(d.hunks) > 0 {
		d.cursor = d.hunks[0].line
	}
	return d
}

// jump moves the cursor to the next or previous hunk. It reports whether one was found.
func (d *diffView) jump(next bool) bool {
	if next {
		for _, h := range d.hunks {
			if h.line > d.cursor {
				d.cursor = h.line
				return true
			}
		}
		return false
	}
	for i := len(d.hunks) - 1; i >= 0; i-- {
		if d.hunks[i].line < d.cursor {
			d.cursor = d.hunks[i].line
			return true
		}
	}
	return false
}

func (e *editor) diffInput(in keyInput) {
	d := e.diff
	if d.pending != "" {
		next := d.pending == "]"
		d.pending = ""
		if in.key == "c" && !d.jump(next) {
			e.notify("No more hunks")
		}
		return
	}
	page := max(1, d.height-1)
	last := len(d.lines) - 1
	switch in.key {
	case "q", "esc":
		e.diff = nil
	case "j", "down":
		d.cursor = min(last, d.cursor+1)
	case "k", "up":
		d.cursor = max(0, d.cursor-1)
	case "ctrl+d", "pagedown":
		d.cursor = min(last, d.cursor+page)
	case "ctrl+u", "pageup":
		d.cursor = max(0, d.cursor-page)
	case "g", "home":
		d.cursor = 0
	case "G", "end":
		d.cursor = last
	case "]", "[":
		d.pending = in.key
	case "enter":
		e.openDiffHunk(d)
	}
}

func (e *editor) openDiffHunk(d *diffView) {
	if len(d.hunks) == 0 {
		e.notify("No hunk to open")
		return
	}
	start := d.hunks[0].start
	for _, h := range d.hunks {
		if h.line <= d.cursor {
			start = h.start
		}
	}
	e.diff, e.panel = nil, nil
	e.jumpTo(d.path, max(0, start-1), 0, false)
}

func (v *renderer) drawSourceControl(e *editor, w, h int) {
	p := e.panel
	v.dim(w, h-1)
	box := centerBox(w, h, min(76, w-2), min(26, h-2))
	title := "Source Control"
	if p.branch != "" {
		title += " · " + p.branch
	}
	v.frame(box, title)
	rows := p.entries()
	height := max(1, box.h-5)
	if len(rows) == 0 {
		v.write(box.x+2, box.y+1, box.w-4, "Working tree clean", mutedStyle.Background(menuStyle.GetBackground()))
	}
	p.scroll = max(0, min(p.scroll, len(rows)-height))
	if p.selected < p.scroll {
		p.scroll = p.selected
	}
	if p.selected >= p.scroll+height {
		p.scroll = p.selected - height + 1
	}
	for i := p.scroll; i < min(len(rows), p.scroll+height); i++ {
		row := rows[i]
		y := box.y + 1 + i - p.scroll
		if row.header != "" {
			v.write(box.x+2, y, box.w-4, row.header, menuStyle.Foreground(th.accent).Bold(true))
			continue
		}
		style := menuStyle
		if i == p.selected {
			style = activeStyle
			v.screen.FillArea(box.x+1, y, box.w-2, 1, ' ', style)
		}
		mark := row.change.work
		if row.staged {
			mark = row.change.index
		}
		if mark == ' ' || mark == 0 {
			mark = '?'
		}
		if row.change.index == '?' {
			mark = '?'
		}
		v.write(box.x+2, y, 1, string(mark), style.Foreground(gitColor(mark)))
		v.write(box.x+4, y, box.w-6, row.change.rel, style)
	}
	muted := mutedStyle.Background(menuStyle.GetBackground())
	v.write(box.x+2, box.y+box.h-3, box.w-4, "s stage  a stage all  A unstage all  d discard  enter diff", muted)
	v.write(box.x+2, box.y+box.h-2, box.w-4, "c commit  p push  P pull  b branch  R refresh  esc close", muted)
	v.screen.HideCursor()
}

func (v *renderer) drawDiff(e *editor, w, h int) {
	d := e.diff
	if e.panel == nil {
		v.dim(w, h-1)
	}
	box := centerBox(w, h, min(100, w-2), min(30, h-2))
	v.frame(box, d.title)
	height := max(1, box.h-3)
	d.height = height
	d.cursor = max(0, min(d.cursor, len(d.lines)-1))
	if d.cursor < d.scroll {
		d.scroll = d.cursor
	}
	if d.cursor >= d.scroll+height {
		d.scroll = d.cursor - height + 1
	}
	for i := d.scroll; i < min(len(d.lines), d.scroll+height); i++ {
		line := d.lines[i]
		fg := th.fg
		switch {
		case strings.HasPrefix(line, "@@"):
			fg = th.aqua
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index "):
			fg = th.muted
		case strings.HasPrefix(line, "+"):
			fg = th.accent
		case strings.HasPrefix(line, "-"):
			fg = th.red
		}
		y := box.y + 1 + i - d.scroll
		style := menuStyle.Foreground(fg)
		if i == d.cursor {
			style = activeStyle.Foreground(fg)
			v.screen.FillArea(box.x+1, y, box.w-2, 1, ' ', style)
		}
		v.write(box.x+2, y, box.w-4, line, style)
	}
	v.write(box.x+2, box.y+box.h-2, box.w-4, "]c next hunk  [c previous  enter open at hunk  q close", mutedStyle.Background(menuStyle.GetBackground()))
	v.screen.HideCursor()
}
