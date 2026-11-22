//go:build desktop

package editor

import (
	"image"
	"path/filepath"
	"strings"
)

func (d *desktop) actionIcon(command string) iconID {
	switch command {
	case "files", "project-search":
		return iconSearch
	case "buffers":
		return iconTabs
	case "open-project", "project":
		return glyphFolderOpen
	}
	return iconCommand
}

// paintWelcome is the start page. It lists the same items, in the same order,
// as the keyboard moves through.
func (d *desktop) paintWelcome(e *editor, p paneBox, area rect) {
	u, c := d.u, &d.c
	d.block(area)
	d.fill(area, c.bg)
	if p.leaf != nil {
		d.zone(zone{r: area, click: func() {
			e.tree.focused = false
			e.focusPane(p.leaf)
		}})
	}
	items := e.dashboardItems()
	actions := 0
	for actions < len(items) && items[actions].path == "" && items[actions].dir == "" {
		actions++
	}
	recents := len(items) - actions
	selected := min(e.dashboardSelection, len(items)-1)
	focused := p.b == e.b && !e.tree.focused && !d.modal(e)

	const actionH, recentH = 36, 46
	room := area.Dy() - u(32)
	height := func(hero, footer bool, n int) int {
		h := u(60) + u(28) + actions*u(actionH)
		if hero {
			h += u(82)
		}
		if n > 0 {
			h += u(22+28) + n*u(recentH)
		}
		if footer {
			h += u(52)
		}
		return h
	}
	// On a short window the logo and footer go before any of the items.
	hero, footer, shown := true, true, recents
	if height(hero, footer, shown) > room {
		footer = false
	}
	if height(hero, footer, shown) > room {
		hero = false
	}
	for shown > 0 && height(hero, footer, shown) > room {
		shown--
	}
	first := max(0, selected-actions-shown+1)

	width := min(u(500), area.Dx()-u(32))
	x0 := (area.Min.X + area.Max.X - width) / 2
	y := max(area.Min.Y+u(12), area.Min.Y+(area.Dy()-height(hero, footer, shown))*2/5)
	mid := x0 + width/2

	if hero {
		d.logo(mid, y+u(34), 60)
		y += u(82)
	}
	d.textCenter(mid, y+u(14), "Loom", textHero, c.text)
	sub := "A fast, modal code editor"
	if !e.noProject {
		sub = shortPath(e.root)
	}
	d.textCenter(mid, y+u(42), d.fitLeft(sub, width, textBody), textBody, c.dim)
	y += u(60)

	caption := func(s string) {
		d.text(x0+u(12), y+u(17), s, textCaption, c.faint)
		y += u(28)
	}
	row := func(i, h int) rect {
		r := image.Rect(x0, y+u(1), x0+width, y+h-u(1))
		hot := d.button(r, func() {
			e.tree.focused = false
			if p.leaf != nil {
				e.focusPane(p.leaf)
			}
			e.dashboardSelection = i
			e.dashboardInput("enter")
		})
		switch {
		case focused && i == selected:
			d.round(r, 8, c.selected)
		case hot:
			d.round(r, 8, c.hover)
		}
		y += h
		return r
	}

	caption("START")
	for i, item := range items[:actions] {
		r := row(i, u(actionH))
		cy := (r.Min.Y + r.Max.Y) / 2
		d.icon(d.actionIcon(item.command), r.Min.X+u(22), cy, 16, c.accent)
		_, label, _ := strings.Cut(item.label, "   ")
		d.text(r.Min.X+u(42), cy, label, textBody, c.text)
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
		d.keys(r.Max.X-u(12), cy, hint)
	}

	if shown > 0 {
		y += u(22)
		if items[actions].dir != "" {
			caption("RECENT PROJECTS")
		} else {
			caption("RECENT FILES")
		}
		for i := actions + first; i < actions+first+shown; i++ {
			item := items[i]
			r := row(i, u(recentH))
			cy := (r.Min.Y + r.Max.Y) / 2
			var name, detail string
			if item.dir != "" {
				name, detail = filepath.Base(item.dir), shortPath(filepath.Dir(item.dir))
				d.icon(glyphFolder, r.Min.X+u(22), cy, 16, c.accent)
			} else {
				_, rel, _ := strings.Cut(item.label, "   ")
				name, detail = filepath.Base(rel), filepath.Dir(rel)
				if detail == "." {
					detail = filepath.Base(e.root)
				}
				d.fileIcon(r.Min.X+u(22), cy, item.path)
			}
			tx, tw := r.Min.X+u(42), r.Dx()-u(58)
			d.text(tx, cy-u(8), d.fit(name, tw, textStrong), textStrong, c.text)
			d.text(tx, cy+u(9), d.fitLeft(detail, tw, textSmall), textSmall, c.dim)
		}
	}
	if footer {
		y += u(34)
		d.hints(mid, y, true, "Space", "all shortcuts", ":", "commands", "↑↓", "choose")
	}
}
