package editor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// Panels: Source Control, diffs, the directory editor, the Welcome page and
// the panes' scrollbars and dividers.

// panelRows is how many rows of the git panel and the diff the web view shows.
type panelRows struct{ git, diff int }

func (b *Bridge) panels(ui map[string]any) {
	e := b.m.editor
	switch {
	case e.notificationDetail != nil || e.menu != nil:
	case e.diff != nil:
		ui["diff"] = b.diffState()
	case e.panel != nil:
		ui["git"] = b.gitState()
	}
	if e.directory != nil {
		dir := e.directory
		ui["directory"] = map[string]any{"path": shortPath(dir.dir), "dirty": dir.doc.b.dirty()}
	}
	ui["panes"] = b.paneState()
}

func (b *Bridge) gitState() map[string]any {
	e := b.m.editor
	p, c := e.panel, &b.frame.colors
	rows := p.entries()
	count := max(1, b.rows2.git)
	p.scroll = max(0, min(p.scroll, len(rows)-count))
	if p.selected < p.scroll {
		p.scroll = p.selected
		if p.scroll == 1 {
			p.scroll = 0 // keep the first heading in sight
		}
	}
	if p.selected >= p.scroll+count {
		p.scroll = p.selected - count + 1
	}
	s := map[string]any{"selected": p.selected, "scroll": p.scroll, "total": len(rows), "staged": p.hasStaged(), "ready": p.top != ""}
	if p.branch != "" {
		s["branch"] = p.branch
		if p.branch == e.git.branch {
			s["branch"] = e.git.branchLabel() // with how far it is from upstream
		}
	}
	var out []any
	for i := p.scroll; i < len(rows) && i < p.scroll+count; i++ {
		row := rows[i]
		if row.header != "" {
			out = append(out, map[string]any{"header": strings.ToUpper(row.header)})
			continue
		}
		mark := row.change.work
		if row.staged {
			mark = row.change.index
		}
		if row.change.index == '?' {
			mark = '?'
		}
		label := string(mark)
		if mark == '?' {
			label = "U"
		}
		r := map[string]any{
			"name": filepath.Base(row.change.rel), "icon": b.fileIcon(row.change.rel),
			"mark": []string{label, hexColor(readable(b.gitInk(mark), c.raised, 3.5))}, "staged": row.staged,
		}
		if dir := filepath.Dir(row.change.rel); dir != "." {
			r["dir"] = dir
		}
		out = append(out, r)
	}
	s["rows"] = out
	return s
}

// diffNumbers gives the old and new line number of every row of a diff.
func diffNumbers(lines []string) [][2]int {
	out := make([][2]int, len(lines))
	before, after := 0, 0
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "@@"):
			if f := strings.Fields(line); len(f) >= 3 {
				first, _, _ := strings.Cut(strings.TrimPrefix(f[1], "-"), ",")
				second, _, _ := strings.Cut(strings.TrimPrefix(f[2], "+"), ",")
				before, _ = strconv.Atoi(first)
				after, _ = strconv.Atoi(second)
			}
		case before == 0 && after == 0, strings.HasPrefix(line, `\`): // no hunk yet, or "No newline at end of file"
		case strings.HasPrefix(line, "+"):
			out[i][1] = after
			after++
		case strings.HasPrefix(line, "-"):
			out[i][0] = before
			before++
		default:
			out[i] = [2]int{before, after}
			before++
			after++
		}
	}
	return out
}

func (b *Bridge) diffState() map[string]any {
	df := b.m.editor.diff
	title := strings.TrimPrefix(df.title, "Diff · ")
	numbers := diffNumbers(df.lines)
	added, removed := 0, 0
	for _, n := range numbers {
		switch {
		case n[0] == 0 && n[1] > 0:
			added++
		case n[1] == 0 && n[0] > 0:
			removed++
		}
	}
	count := max(1, b.rows2.diff)
	df.height = count
	df.cursor = max(0, min(df.cursor, len(df.lines)-1))
	if df.cursor < df.scroll {
		df.scroll = df.cursor
	}
	if df.cursor >= df.scroll+count {
		df.scroll = df.cursor - count + 1
	}
	var lines []any
	for i := df.scroll; i < len(df.lines) && i < df.scroll+count; i++ {
		line, n := strings.ReplaceAll(df.lines[i], "\t", "    "), numbers[i]
		kind, sign := "", ""
		switch {
		case strings.HasPrefix(line, "@@"):
			kind = "hunk"
		case n[0] == 0 && n[1] > 0:
			kind, sign, line = "add", "+", line[1:]
		case n[1] == 0 && n[0] > 0:
			kind, sign, line = "del", "−", line[1:]
		case n[0] > 0:
			line = strings.TrimPrefix(line, " ")
		default:
			kind = "meta"
		}
		if r := []rune(line); len(r) > 240 {
			line = string(r[:240])
		}
		old, cur := "", ""
		if n[0] > 0 {
			old = strconv.Itoa(n[0])
		}
		if n[1] > 0 {
			cur = strconv.Itoa(n[1])
		}
		lines = append(lines, []any{old, cur, sign, line, kind, i == df.cursor})
	}
	return map[string]any{
		"title": title, "icon": b.fileIcon(title), "added": fmt.Sprintf("+%d", added), "removed": fmt.Sprintf("−%d", removed),
		"lines": lines, "scroll": df.scroll, "total": len(df.lines),
	}
}

// paneState is where the panes are on the grid, for scrollbars, dividers and
// the Welcome page.
func (b *Bridge) paneState() []any {
	e, v := b.m.editor, &b.m.view
	var out []any
	for i, p := range v.panes {
		s := map[string]any{"i": i, "box": []int{p.x, p.y, p.w, p.h}}
		switch {
		case p.b.dashboard:
			s["welcome"] = b.welcomeState(p)
		case e.cfg.Scrollbar && len(p.b.lines) > p.h:
			s["scroll"] = []int{p.b.top, p.h, len(p.b.lines)}
		}
		out = append(out, s)
	}
	if e.directory == nil && !e.zen {
		for _, div := range v.paneDividers {
			out = append(out, map[string]any{"divider": []int{div.box.x, div.box.y, div.box.w, div.box.h}, "vertical": div.vertical})
		}
	}
	return out
}

func actionIconName(command string) string {
	switch command {
	case "files", "project-search":
		return "search"
	case "buffers":
		return "tabs"
	case "open-project", "project":
		return "folderOpen"
	}
	return "command"
}

func (b *Bridge) welcomeState(p paneBox) map[string]any {
	e := b.m.editor
	items := e.dashboardItems()
	actions := 0
	for actions < len(items) && items[actions].path == "" && items[actions].dir == "" {
		actions++
	}
	sub := "A fast, modal code editor"
	if !e.noProject {
		sub = shortPath(e.root)
	}
	s := map[string]any{"sub": sub, "selected": min(e.dashboardSelection, len(items)-1),
		"focused": p.b == e.b && !e.tree.focused && e.menu == nil && e.panel == nil && e.diff == nil}
	var start []any
	for _, item := range items[:actions] {
		_, label, _ := strings.Cut(item.label, "   ")
		hint := item.hint
		switch item.command {
		case "files":
			hint = shortcutText("⌘P", hint)
		case "project-search":
			hint = shortcutText("⇧⌘F", hint)
		case "palette":
			hint = shortcutText("⇧⌘P", hint)
		case "open-project":
			hint = shortcutText("⇧⌘O", hint)
		}
		start = append(start, []string{actionIconName(item.command), label, hint})
	}
	s["start"] = start
	var recent []any
	for _, item := range items[actions:] {
		if item.dir != "" {
			s["recentTitle"] = "RECENT PROJECTS"
			recent = append(recent, []any{"dir", filepath.Base(item.dir), shortPath(filepath.Dir(item.dir))})
			continue
		}
		s["recentTitle"] = "RECENT FILES"
		_, rel, _ := strings.Cut(item.label, "   ")
		detail := filepath.Dir(rel)
		if detail == "." {
			detail = filepath.Base(e.root)
		}
		recent = append(recent, []any{b.fileIcon(item.path), filepath.Base(rel), detail})
	}
	s["recent"] = recent
	return s
}

// panelAct handles clicks in the panels.
func (b *Bridge) panelAct(name string, i int, s string) bool {
	e := b.m.editor
	switch name {
	case "panel-close":
		e.panel, e.diff = nil, nil
	case "git-rows":
		b.rows2.git = max(1, i)
	case "git-open", "git-stage":
		p := e.panel
		if p == nil {
			return true
		}
		rows := p.entries()
		if i < 0 || i >= len(rows) || rows[i].header != "" {
			return true
		}
		p.selected = i
		if name == "git-open" {
			e.openDiffFor(rows[i].change.path)
		} else {
			e.sourceControlInput(keyInput{key: "s"})
		}
	case "git-scroll":
		if p := e.panel; p != nil {
			for range absInt(i) {
				p.move(max(-1, min(1, i)))
			}
		}
	case "git-button":
		p := e.panel
		if p == nil || p.top == "" {
			return true
		}
		switch s {
		case "commit":
			e.commitPrompt(p)
		case "push":
			e.gitPush()
		case "pull":
			e.gitRun("pull", []string{"pull", "--ff-only"})
		case "fetch":
			e.gitRun("fetch", []string{"fetch", "--all", "--prune"})
		case "sync":
			e.gitSync()
		case "branch":
			e.openBranchPicker()
		}
	case "diff-rows":
		b.rows2.diff = max(1, i)
	case "diff-scroll":
		if df := e.diff; df != nil {
			count := max(1, b.rows2.diff)
			df.scroll = max(0, min(max(0, len(df.lines)-count), df.scroll+i))
			df.cursor = max(df.scroll, min(df.cursor, df.scroll+count-1))
		}
	case "diff-cursor":
		if df := e.diff; df != nil {
			df.cursor = max(0, min(len(df.lines)-1, i))
		}
	case "directory-close":
		e.closeDirectory(false)
	case "welcome":
		// i is the item, s the pane's index.
		v := &b.m.view
		e.tree.focused = false
		if k, err := strconv.Atoi(s); err == nil && k >= 0 && k < len(v.panes) && v.panes[k].leaf != nil {
			e.focusPane(v.panes[k].leaf)
		}
		e.dashboardSelection = i
		e.dashboardInput("enter")
	case "pane-scroll":
		// Drag on a scrollbar: i is the wanted first line, s the pane's index.
		v := &b.m.view
		k, err := strconv.Atoi(s)
		if err != nil || k < 0 || k >= len(v.panes) {
			return true
		}
		p := v.panes[k]
		want := min(len(p.b.lines)-p.h, max(0, i))
		if delta := want - p.b.top; delta != 0 {
			doc := e
			if e.directory != nil {
				doc = e.directory.doc
			}
			v.paneWheel(doc, p, delta, 0)
		}
	default:
		return false
	}
	return true
}
