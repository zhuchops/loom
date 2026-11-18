package editor

import "slices"

// dropZone is where a dragged tab would land: the whole pane, or one half of
// it when the pointer is near an edge, which splits the pane that way.
type dropZone struct {
	box  hitBox
	leaf *pane
	zone string // center, left, right, top, bottom
}

func dropZoneAt(p paneBox, x, y int) dropZone {
	rx, ry := (x-p.x)*100/max(1, p.w), (y-p.y)*100/max(1, p.h)
	box := p.hitBox
	switch {
	case rx < 25:
		return dropZone{hitBox{p.x, p.y, p.w / 2, p.h, 0}, p.leaf, "left"}
	case rx >= 75:
		return dropZone{hitBox{p.x + p.w - p.w/2, p.y, p.w / 2, p.h, 0}, p.leaf, "right"}
	case ry < 25:
		return dropZone{hitBox{p.x, p.y, p.w, p.h / 2, 0}, p.leaf, "top"}
	case ry >= 75:
		return dropZone{hitBox{p.x, p.y + p.h - p.h/2, p.w, p.h / 2, 0}, p.leaf, "bottom"}
	}
	return dropZone{box, p.leaf, "center"}
}

// dragTab tracks a tab being dragged. Over the tab bar it reorders tabs; over
// a pane it picks a drop zone.
func (v *renderer) dragTab(e *editor, x, y int) {
	if !v.tabMoving && absInt(x-v.tabStartX)+absInt(y-v.tabStartY) < 3 {
		return
	}
	v.tabMoving = true
	v.drop = dropZone{}
	if y == 0 {
		for _, tab := range v.tabs {
			if tab.contains(x, y) && tab.b != v.tabDrag && e.b == v.tabDrag {
				if at := slices.Index(e.buffers, tab.b); at >= 0 {
					e.moveTab(at)
				}
			}
		}
		return
	}
	for _, pane := range v.panes {
		if pane.leaf != nil && pane.contains(x, y) && !pane.b.dashboard {
			v.drop = dropZoneAt(pane, x, y)
			return
		}
	}
}

// dropTab opens the dragged tab where it was released.
func (v *renderer) dropTab(e *editor) {
	d, b := v.drop, v.tabDrag
	if d.leaf == nil || b == nil || !slices.Contains(e.buffers, b) || e.directory != nil {
		return
	}
	e.focusPane(d.leaf)
	if e.activePane != d.leaf {
		return
	}
	if d.zone == "center" {
		e.switchBuffer(b)
		return
	}
	e.splitPane(d.zone == "left" || d.zone == "right", "")
	if d.leaf.b != nil {
		return // The split did not happen.
	}
	if d.zone == "left" || d.zone == "top" {
		d.leaf.first, d.leaf.second = d.leaf.second, d.leaf.first
	}
	e.switchBuffer(b)
	// The tab moved out of the old pane, so that pane goes back to what it showed.
	old := d.leaf.first
	if old == e.activePane {
		old = d.leaf.second
	}
	if prev := v.tabPrev; old.b == b && prev != nil && prev != b && slices.Contains(e.buffers, prev) {
		old.b, old.cursor, old.top, old.left = prev, min(prev.cursor, len(prev.text)), prev.top, prev.left
	}
}

func (v *renderer) drawDrop() {
	box := v.drop.box
	for y := box.y; y < box.y+box.h; y++ {
		for x := box.x; x < box.x+box.w; {
			s, style, width := v.screen.Get(x, y)
			if s == "" {
				s = " "
			}
			v.screen.Put(x, y, s, max(1, width), style.Background(th.sel))
			x += max(1, width)
		}
	}
	label := map[string]string{"center": "Open here", "left": "Split left", "right": "Split right", "top": "Split up", "bottom": "Split down"}[v.drop.zone]
	text := " " + label + " "
	v.write(box.x+max(0, (box.w-len(text))/2), box.y+box.h/2, box.w, text, menuStyle.Bold(true))
}
