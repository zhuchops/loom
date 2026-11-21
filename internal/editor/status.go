package editor

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (v *renderer) drawStatus(e *editor, w, h int) {
	doc := e
	b := e.b
	name := b.path
	if e.directory != nil {
		doc = e.directory.doc
		b = doc.b
		name = e.directory.dir
	}
	if rel, err := filepath.Rel(e.root, name); name != "" && err == nil && within(e.root, name) {
		name = rel
	}
	if b.dashboard {
		name = "Dashboard"
	} else if name == "" {
		name = "Untitled"
	}
	if e.directory != nil {
		name = "Directory · " + name
	}
	if b.dirty() {
		name += " ●"
	}
	if b.external {
		name += " [disk changed]"
	}
	mode := strings.ToUpper(doc.mode)
	if b.dashboard {
		mode = "DASHBOARD"
	}
	if doc.mode == "select" && b.visualBlock {
		mode = "VISUAL BLOCK"
	} else if doc.mode == "select" && b.visualLine {
		mode = "VISUAL LINE"
	} else if doc.mode == "select" && doc.cfg.Profile == "vim" {
		mode = "VISUAL"
	}
	if e.tree.focused {
		mode = "EXPLORER"
	}
	if e.menu != nil {
		mode = "PROMPT"
	}
	// NvChad-style statusline: a mode block coloured by mode, the editing
	// profile on a lighter block, and a coloured position block on the right.
	modeColor := th.accent
	switch {
	case e.menu != nil || e.tree.focused:
		modeColor = th.yellow
	case doc.mode == "insert":
		modeColor = th.purple
	case doc.mode == "select":
		modeColor = th.aqua
	case strings.HasPrefix(doc.mode, "replace"):
		modeColor = th.orange
	}
	badge := " \uf108 " + mode + " "
	badgeStyle := baseStyle.Background(modeColor).Foreground(th.badge).Bold(true)
	profile := " " + strings.ToLower(doc.cfg.Profile) + " "
	profileStyle := menuStyle.Foreground(modeColor)
	row := b.rowAt(b.cursor)
	position := fmt.Sprintf(" \uf0c9 %d:%d  %d%% ", row+1, b.cursor-b.lines[row]+1, (row+1)*100/len(b.lines))
	positionStyle := baseStyle.Background(modeColor).Foreground(th.badge).Bold(true)
	right := ""
	if w >= 78 {
		ending := "LF"
		if b.newline() == "\r\n" {
			ending = "CRLF"
		}
		lang := strings.ToUpper(e.cfg.languageFor(b.path))
		if lang == "" {
			lang = "TEXT"
		}
		right = " " + lang + "  " + ending + "  UTF-8 "
	}
	if b.dashboard {
		position = ""
	}
	if e.git.branch != "" {
		right = " \U000f062c " + e.git.branch + "  " + right
	}
	folder, folderName := "", ""
	if w >= 78 {
		folder, folderName = " \uf07b ", " "+filepath.Base(e.root)+" "
	}
	v.screen.FillArea(0, h-1, w, 1, ' ', panelStyle)
	v.write(0, h-1, w, badge, badgeStyle)
	v.write(cellWidth(badge), h-1, w, profile, profileStyle)
	rightX := max(cellWidth(badge)+cellWidth(profile), w-cellWidth(right)-cellWidth(folder)-cellWidth(folderName)-cellWidth(position))
	v.write(rightX, h-1, w-rightX, right, mutedStyle)
	x := rightX + cellWidth(right)
	if folder != "" {
		v.write(x, h-1, w-x, folder, baseStyle.Background(th.accent).Foreground(th.badge))
		x += cellWidth(folder)
		v.write(x, h-1, w-x, folderName, menuStyle)
		x += cellWidth(folderName)
	}
	v.write(x, h-1, w-x, position, positionStyle)
	left := cellWidth(badge) + cellWidth(profile) + 1
	keys := keyLabel(doc.prefix)
	if doc.pending.kind != "" {
		keys += " " + doc.pending.kind
	}
	if doc.count > 0 {
		keys = fmt.Sprintf("%d %s", doc.count, keys)
	}
	if doc.recording != "" {
		keys = "REC " + doc.recording + " " + keys
	}
	keys = strings.TrimSpace(keys)
	if keys != "" {
		keys = " " + keys + " … "
		width := min(rightX-left, cellWidth(keys))
		v.write(left, h-1, width, keys, activeStyle)
		left += max(0, width) + 1
	}
	if e.problemsVisible(b) {
		left += v.drawProblems(e, b, left, h-1, rightX-left) + 2
	}
	if crumbs := e.breadcrumbs(b); crumbs != "" && e.cfg.StickyScroll && w >= 60 {
		name += "  ›  " + crumbs
	}
	message := e.message
	if message != "" {
		name = message + "  ·  " + name
	}
	v.write(left, h-1, rightX-left, name, panelStyle)
}

// showProblems keeps the counts visible for any file with a language server, even when both are zero.
func (e *editor) problemsVisible(b *buffer) bool {
	if len(b.diagnostics) > 0 || len(e.problems) > 0 {
		return true
	}
	return len(e.cfg.Languages[e.cfg.languageFor(b.path)].Server) > 0
}

// drawProblems writes "file E0 W0 · project E0 W0" and returns the width it used.
func (v *renderer) drawProblems(e *editor, b *buffer, x, y, limit int) int {
	background := panelStyle.GetBackground()
	muted := mutedStyle.Background(background)
	count := func(n int, fg terminalColor) cellStyle {
		if n == 0 {
			return muted
		}
		return headingStyle.Background(background).Foreground(fg)
	}
	errorColor := th.red
	warningColor := th.yellow
	file := countProblems(b.diagnostics)
	project := e.projectProblems()
	parts := []struct {
		text  string
		style cellStyle
	}{
		{"file ", muted},
		{fmt.Sprintf("E%d", file.errors), count(file.errors, errorColor)},
		{" ", muted},
		{fmt.Sprintf("W%d", file.warnings), count(file.warnings, warningColor)},
		{"  ·  project ", muted},
		{fmt.Sprintf("E%d", project.errors), count(project.errors, errorColor)},
		{" ", muted},
		{fmt.Sprintf("W%d", project.warnings), count(project.warnings, warningColor)},
	}
	start := x
	for _, part := range parts {
		width := cellWidth(part.text)
		if x+width > x+limit {
			break
		}
		v.write(x, y, width, part.text, part.style)
		x += width
	}
	return x - start
}
