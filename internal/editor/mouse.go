package editor

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

type pointerButton uint8

const (
	pointerNone pointerButton = iota
	mouseRight
	mouseMiddle
	wheelUp
	wheelDown
	wheelLeft
	wheelRight
)

type pointerEvent struct {
	x, y            int
	button          pointerButton
	leftDown, shift bool
}

// wheelGesture is how long the wheel must be quiet before a new swipe may pick a new axis.
const wheelGesture = 150 * time.Millisecond

func (v *renderer) mouse(e *editor, event pointerEvent) {
	x, y := event.x, event.y
	button := event.button
	down := event.leftDown
	press := down && !v.leftDown
	v.leftDown = down
	wheel := 0
	if button == wheelUp {
		wheel = -3
	}
	if button == wheelDown {
		wheel = 3
	}
	if isWheel := wheel != 0 || button == wheelLeft || button == wheelRight; isWheel {
		// A touchpad swipe is never perfectly straight. Keep to the axis the
		// swipe started on until the wheel pauses.
		axis := 1
		if wheel == 0 {
			axis = 2
		}
		now := time.Now()
		if now.Sub(v.wheelAt) > wheelGesture {
			v.wheelAxis = axis
		}
		v.wheelAt = now
		if axis != v.wheelAxis {
			return
		}
	}
	if v.mouseDiagnosticHover(e, event, wheel) {
		return
	}
	if !down {
		if v.tabMoving {
			v.dropTab(e)
		}
		v.tabDrag, v.tabMoving, v.drop = nil, false, dropZone{}
		v.paneDrag = nil
		v.resize, v.dragging, v.drag = false, false, nil
	}
	if e.notificationDetail != nil {
		e.notificationDetail.move(wheel)
		return
	}
	if e.menu != nil {
		m := e.menu
		if wheel != 0 {
			if v.previewBox.contains(x, y) {
				m.previewScroll += wheel
				return
			}
			m.selected = max(0, min(len(m.matches)-1, m.selected+wheel))
			return
		}
		if press && v.menuBox.contains(x, y) {
			m.selected = v.menuBox.start + y - v.menuBox.y
			if m.selected < len(m.matches) && !m.confirmation {
				if run := m.matches[m.selected].run; run != nil {
					e.menu = nil
					run()
				}
			}
		}
		if m.details && !down && button == pointerNone && v.menuBox.contains(x, y) {
			m.selected = max(0, min(len(m.matches)-1, v.menuBox.start+y-v.menuBox.y))
			m.hovering = true
		} else if !v.previewBox.contains(x, y) {
			m.hovering = false
		}
		return
	}
	if y == 0 && len(v.tabs) > 0 && x >= v.tabs[0].x && (wheel != 0 || button == wheelLeft || button == wheelRight) {
		step := 1
		if wheel < 0 || button == wheelLeft {
			step = -1
		}
		if !v.tabManual {
			v.tabStart = max(0, slices.Index(e.buffers, v.tabs[0].b))
		}
		v.tabStart += step
		v.tabManual = true
		return
	}
	if _, h := v.screen.Size(); !v.native && press && y == h-1 {
		e.openNotifications()
		return
	}
	if e.diff != nil || e.panel != nil {
		return
	}
	if e.bar != nil && e.directory == nil {
		for _, h := range v.barHits {
			if h.contains(x, y) {
				if press {
					e.barAction(h.action)
				}
				return
			}
		}
	}
	if v.completeBox.contains(x, y) && len(e.completion.items) > 0 {
		if wheel != 0 {
			e.completion.selected = max(0, min(len(e.completion.items)-1, e.completion.selected+wheel))
			return
		}
		if press {
			e.completion.selected = v.completeBox.start + y - v.completeBox.y
			e.completionInput("enter")
			e.scheduleLanguage()
			e.scheduleGit()
		}
		return
	}
	if e.directory != nil {
		if !v.directoryBox.contains(x, y) || len(v.panes) == 0 {
			if press {
				e.closeDirectory(false)
			}
			return
		}
		if button == mouseRight || button == mouseMiddle {
			return
		}
		doc := e.directory.doc
		panes := v.panes
		v.panes = v.panes[len(panes)-1:]
		tree, divider := v.treeBox, v.divider
		v.treeBox, v.divider = hitBox{}, -1
		v.leftDown = !press && down
		v.mouse(doc, event)
		v.panes, v.treeBox, v.divider = panes, tree, divider
		return
	}
	if v.paneDrag != nil && down {
		d := v.paneDrag
		if d.vertical {
			d.node.setRatio(ratioFor(x-d.whole.x, d.whole.w), false)
		} else {
			d.node.setRatio(ratioFor(y-d.whole.y, d.whole.h), false)
		}
		return
	}
	if press && v.tabDrag == nil {
		for i := range v.paneDividers {
			if d := v.paneDividers[i]; d.box.contains(x, y) {
				now := time.Now()
				if now.Sub(v.lastClick) < 350*time.Millisecond && x == v.lastX && y == v.lastY {
					d.node.setRatio(0, e.cfg.Animations) // double-click evens the split out
				} else {
					v.paneDrag = &d
				}
				v.lastClick, v.lastX, v.lastY = now, x, y
				return
			}
		}
	}
	if v.tabDrag != nil && down && !press {
		v.dragTab(e, x, y)
		return
	}
	if v.resize {
		e.tree.width = max(16, x)
		return
	}
	if press && !e.tree.popup && x == v.divider {
		v.resize = true
		return
	}
	if v.treeBox.contains(x, y) {
		if wheel != 0 {
			e.tree.manualScroll = true
			e.tree.scroll = max(0, min(max(0, len(e.tree.rows)-v.treeBox.h), e.tree.scroll+wheel))
			return
		}
		at := v.treeBox.start + y - v.treeBox.y
		if at < len(e.tree.rows) && (press || button == mouseRight) {
			e.tree.selected, e.tree.focused, e.tree.manualScroll = at, true, false
			if button == mouseRight {
				e.treeContext()
				return
			}
			e.treeInput("enter")
		}
		return
	}
	if e.tree.popup {
		if press {
			e.tree.popup, e.tree.focused = false, false
		}
		return
	}
	for _, tab := range v.tabs {
		if !tab.contains(x, y) {
			continue
		}
		if (press && x == tab.close) || button == mouseMiddle {
			e.closeBuffer(tab.b)
		} else if press {
			v.tabPrev = e.b
			e.switchBuffer(tab.b)
			v.tabDrag, v.tabStartX, v.tabStartY = tab.b, x, y
		}
		return
	}
	for _, pv := range v.previews {
		if pv.contains(x, y) {
			if wheel != 0 {
				v.previewWheel(pv.p, wheel)
			}
			return
		}
	}
	for _, pane := range v.panes {
		if !pane.contains(x, y) && !(v.dragging && v.drag == pane.b && (pane.leaf == nil || pane.leaf == e.activePane)) {
			continue
		}
		b := pane.b
		if b.dashboard {
			if !v.native && press && pane.contains(x, y) {
				e.focusPane(pane.leaf)
				if at := e.dashboardBox(pane.hitBox); at.contains(x, y) {
					e.dashboardSelection = at.start + y - at.y
					e.dashboardInput("enter")
				}
			}
			return
		}
		if wheel != 0 {
			v.paneWheel(e, pane, wheel, 0)
			return
		}
		if button == wheelLeft || button == wheelRight {
			delta := 4
			if button == wheelLeft {
				delta = -4
			}
			v.paneWheel(e, pane, 0, delta)
			return
		}
		if press || v.dragging && down {
			if pane.leaf != nil {
				e.focusPane(pane.leaf)
			}
			e.tree.focused = false
			e.invalidateCompletion()
			e.scrollLock = false
			if v.dragging && !press {
				v.edgeScroll(pane, x, y)
			}
			e.useTabWidth(b)
			row := b.rowAtScreen(b.top, max(0, min(pane.h-1, y-pane.y)))
			at := cellOffset(b, row, x-pane.x-pane.gutter+b.left)
			if press {
				now := time.Now()
				double := now.Sub(v.lastClick) < 350*time.Millisecond && x == v.lastX && y == v.lastY
				v.lastClick, v.lastX, v.lastY = now, x, y
				v.drag, v.dragStart, v.dragging = b, at, true
				b.visualLine = false
				if event.shift {
					v.dragStart = b.cursor
					e.mode, b.anchor = "select", b.cursor
				} else {
					b.anchor = -1
					if e.mode == "select" {
						e.mode = "normal"
					}
				}
				b.cursor = at
				if double {
					if s, ok := textObject(b, 'w', false); ok {
						e.mode, b.anchor, b.cursor = "select", s.start, s.end-1
						v.dragStart = s.start
					}
				}
			} else if v.drag == b && at != v.dragStart {
				e.mode, b.anchor, b.cursor = "select", v.dragStart, at
			}
			e.clampModalCursor()
			return
		}
		if button == mouseMiddle {
			if pane.leaf != nil {
				e.focusPane(pane.leaf)
			}
			e.paste(false)
			return
		}
		if button == mouseRight {
			if pane.leaf != nil {
				e.focusPane(pane.leaf)
			}
			if b.anchor < 0 {
				row := b.rowAtScreen(b.top, max(0, y-pane.y))
				b.cursor = cellOffset(b, row, x-pane.x-pane.gutter+b.left)
				e.clampModalCursor()
			}
			e.editorContext()
			return
		}
	}
}

func cellOffset(b *buffer, row, col int) int {
	start, end := b.lines[row], b.lineEnd(row)
	at, column := start, 0
	g := uniseg.NewGraphemes(string(b.text[start:end]))
	for g.Next() {
		width := max(1, g.Width())
		if g.Str() == "\t" {
			width = tabWidth - column%tabWidth
		}
		if col < column+width {
			return at
		}
		column += width
		at += utf8.RuneCountInString(g.Str())
	}
	return at
}

func (e *editor) treeContext() {
	m := &menu{title: "Explorer Actions"}
	for _, action := range []struct{ label, key string }{{"Open", "enter"}, {"Filter", "/"}, {"Sort", "s"}, {"New File", "a"}, {"New Folder", "A"}, {"Rename", "r"}, {"Move", "m"}, {"Cut", "x"}, {"Paste", "p"}, {"Move to Trash", "d"}} {
		m.options = append(m.options, option{action.label, formatKey(action.key), func() { e.treeInput(action.key) }, nil})
	}
	e.openMenu(m)
}

func (e *editor) editorContext() {
	m := &menu{title: "Editor Actions"}
	for _, id := range []string{"definition", "references", "hover", "save", "split", "close-tab"} {
		cmd := e.commands[id]
		m.options = append(m.options, option{cmd.title, e.shortcut(id), func() { cmd.run(e) }, nil})
	}
	for _, op := range []string{"Copy", "Cut", "Paste"} {
		m.options = append(m.options, option{op, "", func() {
			if op == "Paste" {
				e.paste(false)
				return
			}
			action := "yank"
			if op == "Cut" {
				action = "delete"
			}
			e.operate(action, e.b.selection(), e.b.visualLine)
			if action == "delete" {
				e.scheduleLanguage()
			}
			e.notify(strings.ToLower(op) + " selection")
		}, nil})
	}
	e.openMenu(m)
}

// edgeScroll moves the view while a selection is dragged to or past the edge of
// the pane, so there is more text to reach. The further out the pointer is, the
// faster it goes.
func (v *renderer) edgeScroll(pane paneBox, x, y int) {
	b := pane.b
	band := 1
	if pane.h < 4 {
		band = 0
	}
	if above := pane.y + band - y; above > 0 {
		b.top = max(0, b.top-min(above, 6))
	} else if below := y - (pane.y + pane.h - 1 - band); below > 0 {
		b.top = min(len(b.lines)-1, b.top+min(below, 6))
	}
	left, right := pane.x+pane.gutter, pane.x+pane.w-1
	if out := left + 1 - x; out > 0 && b.left > 0 {
		b.left = max(0, b.left-min(out, 8))
	} else if out := x - (right - 1); out > 0 {
		row := b.rowAtScreen(b.top, max(0, min(pane.h-1, y-pane.y)))
		if cellOffset(b, row, right-left+b.left) < b.lineEnd(row) {
			b.left += min(out, 8)
		}
	}
}
