package editor

import (
	"os"
	"path/filepath"
	"strings"
)

var dashboardActions = []struct {
	label, command string
}{
	{"Go to File", "files"},
	{"Search Project", "project-search"},
	{"Open Buffers", "buffers"},
	{"Command Palette", "palette"},
}

// dashItem is one selectable row: a command, or a recent file to open.
type dashItem struct {
	label, hint, command, path string
}

func (e *editor) dashboardItems() []dashItem {
	var items []dashItem
	for _, a := range dashboardActions {
		items = append(items, dashItem{label: a.label, hint: e.shortcut(a.command), command: a.command})
	}
	shown := 0
	for _, path := range e.recent {
		if shown == 5 {
			break
		}
		if _, err := os.Stat(path); err != nil || !within(e.root, path) {
			continue
		}
		rel, _ := filepath.Rel(e.root, path)
		items = append(items, dashItem{label: rel, hint: "recent", path: path})
		shown++
	}
	return items
}

var dashboardLogo = []string{
	"██╗      ██████╗  ██████╗ ███╗   ███╗",
	"██║     ██╔═══██╗██╔═══██╗████╗ ████║",
	"██║     ██║   ██║██║   ██║██╔████╔██║",
	"██║     ██║   ██║██║   ██║██║╚██╔╝██║",
	"███████╗╚██████╔╝╚██████╔╝██║ ╚═╝ ██║",
	"╚══════╝ ╚═════╝  ╚═════╝ ╚═╝     ╚═╝",
}

func (e *editor) showDashboard() {
	for _, b := range e.buffers {
		if b.dashboard {
			e.switchBuffer(b)
			return
		}
	}
	b, _ := loadBuffer("")
	b.dashboard = true
	if len(e.buffers) == 1 && e.b.path == "" && !e.b.dirty() && e.layout == e.activePane {
		e.buffers = e.buffers[:0]
	}
	e.buffers = append([]*buffer{b}, e.buffers...)
	e.switchBuffer(b)
}

// Dashboard shares workspace bindings with editor tabs. Text commands resume
// when a file tab is focused.
func (e *editor) runCommand(id string) {
	if e.b.dashboard {
		switch id {
		case "dashboard", "files", "buffers", "palette", "ex", "project-search",
			"explorer", "explorer-popup", "close-explorer", "toggle-sidebar", "directory-jump",
			"messages", "quit", "keybindings", "profile-vim", "recover-buffers",
			"jump-back", "jump-forward", "git-panel", "git-branch", "refresh-git", "problems",
			"save", "normal", "zen", "relative-numbers", "close-tab", "only-pane", "close-split",
			"split", "split-below", "focus-pane", "pane-h", "pane-j", "pane-k", "pane-l":
		default:
			if !strings.HasPrefix(id, "tab-") {
				return
			}
		}
	}
	e.commands[id].run(e)
}

func (e *editor) dashboardInput(key string) bool {
	items := e.dashboardItems()
	e.dashboardSelection = min(e.dashboardSelection, len(items)-1)
	switch key {
	case "j", "down", "ctrl+n":
		e.dashboardSelection = min(len(items)-1, e.dashboardSelection+1)
	case "k", "up", "ctrl+p":
		e.dashboardSelection = max(0, e.dashboardSelection-1)
	case "enter":
		if item := items[e.dashboardSelection]; item.path != "" {
			e.open(item.path)
		} else {
			e.runCommand(item.command)
		}
	default:
		return false
	}
	return true
}

// dashboardHeader is the number of rows above the list: the logo when there
// is room for it, otherwise just a heading.
func dashboardHeader(boxH, items int) int {
	if boxH >= items+12 {
		return len(dashboardLogo) + 4
	}
	return 4
}

func (e *editor) dashboardBox(box hitBox) hitBox {
	count := len(e.dashboardItems())
	header := dashboardHeader(box.h, count)
	rows := min(count, max(0, box.h-header))
	section := centerBox(box.w, box.h, min(56, box.w-2), min(box.h, rows+header))
	selected := min(e.dashboardSelection, count-1)
	return hitBox{x: box.x + section.x, y: box.y + section.y + header, w: section.w, h: rows, start: max(0, selected-rows+1)}
}

func (v *renderer) drawDashboard(e *editor, box hitBox, focused bool) {
	items := e.dashboardItems()
	list := e.dashboardBox(box)
	header := dashboardHeader(box.h, len(items))
	top := list.y - header
	project := filepath.Base(e.root)
	if e.git.branch != "" {
		project += "  \U000f062c " + e.git.branch
	}
	if header > 4 {
		colors := []terminalColor{th.accent, th.accent, th.aqua, th.aqua, th.purple, th.purple}
		for i, line := range dashboardLogo {
			v.write(list.x, top+i, list.w, line, baseStyle.Foreground(colors[i%len(colors)]).Bold(true))
		}
		v.write(list.x, top+len(dashboardLogo)+1, list.w, "terminal editor · v"+version, mutedStyle)
		v.write(list.x, top+len(dashboardLogo)+2, list.w, project, headingStyle)
	} else if box.h >= 1 {
		v.write(list.x, min(top, box.y+box.h-1), list.w, "Loom", headingStyle)
		if box.h >= 2 {
			v.write(list.x, min(top+1, box.y+box.h-1), list.w, project, mutedStyle)
		}
	}
	for row := range list.h {
		i := list.start + row
		item := items[i]
		style, mark := menuStyle, "  "
		if focused && i == e.dashboardSelection {
			style, mark = activeStyle, "› "
		}
		y := list.y + row
		v.screen.FillArea(list.x, y, list.w, 1, ' ', style)
		v.writeOption(list.x+1, y, list.w-2, option{label: mark + item.label, hint: item.hint}, style)
	}
}
