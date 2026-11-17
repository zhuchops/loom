package main

import (
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
	switch key {
	case "j", "down":
		e.dashboardSelection = min(len(dashboardActions)-1, e.dashboardSelection+1)
	case "k", "up":
		e.dashboardSelection = max(0, e.dashboardSelection-1)
	case "enter":
		e.runCommand(dashboardActions[e.dashboardSelection].command)
	default:
		return false
	}
	return true
}

func dashboardActionsBox(box hitBox, selected int) hitBox {
	rows := min(len(dashboardActions), max(0, box.h-4))
	section := centerBox(box.w, box.h, min(48, box.w-2), min(box.h, rows+4))
	return hitBox{x: box.x + section.x, y: box.y + section.y + 4, w: section.w, h: rows, start: max(0, selected-rows+1)}
}

func (v *renderer) drawDashboard(e *editor, box hitBox, focused bool) {
	actions := dashboardActionsBox(box, e.dashboardSelection)
	if box.h >= 1 {
		titleY := min(actions.y-4, box.y+box.h-1)
		v.write(actions.x, titleY, actions.w, "Loom", headingStyle)
		if box.h >= 2 {
			project := filepath.Base(e.root)
			v.write(actions.x, titleY+1, actions.w, project, mutedStyle)
		}
	}
	for row := range actions.h {
		i := actions.start + row
		item := dashboardActions[i]
		style, mark := menuStyle, "  "
		if focused && i == e.dashboardSelection {
			style, mark = activeStyle, "› "
		}
		y := actions.y + row
		v.screen.FillArea(actions.x, y, actions.w, 1, ' ', style)
		v.writeOption(actions.x+1, y, actions.w-2, option{
			label: mark + item.label, hint: e.shortcut(item.command),
		}, style)
	}
}
