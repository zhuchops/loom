package editor

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/zhuchops/loom/internal/markdown"
)

// Internal nodes partition their rectangle. Leaves reference shared buffers;
// cursor and scroll state belong to the leaf, while text and undo belong to the buffer.
type pane struct {
	first, second     *pane
	vertical          bool // true places children left/right; false places them top/bottom.
	b                 *buffer
	cursor, top, left int
	dashboardPrevious *pane // File view covered by the dashboard in this leaf.

	// A preview leaf renders b as Markdown instead of editing it. It never
	// takes focus, and redraws from the buffer so it tracks every keystroke.
	ratio              int // share of the first child in permille; 0 means half
	preview            bool
	md                 []markdown.Line
	mdVersion, mdWidth int
	mdTop, mdRow       int
	mdManual           bool
}

func (e *editor) editableLeaves() []*pane {
	var out []*pane
	for _, p := range e.layout.leaves() {
		if !p.preview {
			out = append(out, p)
		}
	}
	return out
}

// dropPreviews removes preview leaves of b, or of every buffer when b is nil.
func (e *editor) dropPreviews(b *buffer) {
	for _, p := range e.layout.leaves() {
		if p.preview && (b == nil || p.b == b) && len(e.layout.leaves()) > 1 {
			e.layout = prunePane(e.layout, p)
		}
	}
}

func (e *editor) toggleMarkdownPreview() {
	if e.directory != nil || e.b.dashboard || e.b.tags != nil {
		return
	}
	if e.cfg.languageFor(e.b.path) != "markdown" {
		e.notify("Preview works on Markdown files")
		return
	}
	for _, p := range e.layout.leaves() {
		if p.preview && p.b == e.b {
			e.dropPreviews(e.b)
			return
		}
	}
	if len(e.layout.leaves()) >= 32 {
		e.notify("Pane limit reached")
		return
	}
	e.rememberPane()
	p := e.activePane
	first := *p
	*p = pane{first: &first, second: &pane{b: e.b, preview: true, mdRow: -1}, vertical: true}
	e.activePane = &first
}

func (p *pane) leaves() []*pane {
	if p == nil {
		return nil
	}
	if p.b != nil {
		return []*pane{p}
	}
	return append(p.first.leaves(), p.second.leaves()...)
}

func (e *editor) rememberPane() {
	p := e.activePane
	if p != nil && p.b == e.b {
		p.cursor, p.top, p.left = e.b.cursor, e.b.top, e.b.left
	}
}

func (e *editor) focusPane(p *pane) {
	if p == nil || p == e.activePane || p.preview {
		return
	}
	e.normal()
	e.rememberPane()
	e.activePane, e.b = p, p.b
	e.b.cursor, e.b.top, e.b.left = min(p.cursor, len(e.b.text)), p.top, p.left
	e.afterBufferSwitch()
}

func (e *editor) splitPane(vertical bool, path string) {
	if e.directory != nil {
		e.notify("Close the directory editor before splitting")
		return
	}
	if len(e.layout.leaves()) >= 32 {
		e.notify("Pane limit reached")
		return
	}
	var b *buffer
	if path != "" {
		var err error
		b, err = loadBuffer(path)
		if err != nil {
			e.notify(err.Error())
			return
		}
	}
	e.normal()
	e.rememberPane()
	p := e.activePane
	if p == nil {
		return
	}
	first := *p
	second := *p
	second.dashboardPrevious = nil
	*p = pane{first: &first, second: &second, vertical: vertical}
	e.activePane = &second
	if b != nil {
		e.openLoaded(b)
	}
}

func prunePane(node, target *pane) *pane {
	if node == target {
		return nil
	}
	if node == nil || node.b != nil {
		return node
	}
	left, right := prunePane(node.first, target), prunePane(node.second, target)
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	node.first, node.second = left, right
	return node
}

func (e *editor) closePane(only bool) {
	p := e.activePane
	if p == nil {
		return
	}
	if only {
		e.layout = p
		return
	}
	if e.layout == p {
		e.closeBuffer(e.b)
		return
	}
	leaves := e.editableLeaves()
	if len(leaves) < 2 {
		// Only previews would be left, so close the buffer and its previews.
		e.dropPreviews(nil)
		e.closeBuffer(e.b)
		return
	}
	at := slices.Index(leaves, p)
	next := leaves[max(0, at-1)]
	if at == 0 {
		next = leaves[1]
	}
	e.layout = prunePane(e.layout, p)
	e.focusPane(next)
}

type panePlacement struct {
	p   *pane
	box hitBox
}
type paneDivider struct {
	box      hitBox
	vertical bool
	node     *pane  // the split this divider belongs to
	whole    hitBox // the area the split divides
}

const (
	minPaneWidth  = 8
	minPaneHeight = 3
	resizeStep    = 50 // permille moved by one keypress
)

// splitSize is the width or height given to the first child of a split.
func splitSize(total, ratio int, vertical bool) int {
	if ratio <= 0 {
		ratio = 500
	}
	minimum := minPaneHeight
	if vertical {
		minimum = minPaneWidth
	}
	room := total - 1
	if room < 2*minimum {
		return room / 2
	}
	return max(minimum, min(room-minimum, room*ratio/1000))
}

// ratioFor converts a first-child size back to permille for a split of total cells.
func ratioFor(size, total int) int {
	return max(1, min(999, size*1000/max(1, total-1)))
}

// Layout and keyboard focus use the same tree geometry; tiny terminals show
// the active leaf without discarding any hidden panes.
func placePanes(p *pane, box hitBox, active *pane, leaves *[]panePlacement, dividers *[]paneDivider) {
	if p == nil {
		return
	}
	if p.b != nil {
		*leaves = append(*leaves, panePlacement{p, box})
		return
	}
	if p.vertical && box.w < 17 || !p.vertical && box.h < 5 {
		leaf := p.leaves()[0]
		if slices.Contains(p.leaves(), active) {
			leaf = active
		}
		*leaves = append(*leaves, panePlacement{leaf, box})
		return
	}
	a, b := box, box
	edge := box
	if p.vertical {
		a.w = splitSize(box.w, p.ratio, true)
		b.x, b.w = box.x+a.w+1, box.w-a.w-1
		edge.x, edge.w = box.x+a.w, 1
	} else {
		a.h = splitSize(box.h, p.ratio, false)
		b.y, b.h = box.y+a.h+1, box.h-a.h-1
		edge.y, edge.h = box.y+a.h, 1
	}
	*dividers = append(*dividers, paneDivider{edge, p.vertical, p, box})
	placePanes(p.first, a, active, leaves, dividers)
	placePanes(p.second, b, active, leaves, dividers)
}

// ancestors returns the splits from the root down to the one holding target.
func ancestors(node, target *pane, path []*pane) []*pane {
	if node == nil || node == target {
		return path
	}
	if node.b != nil {
		return nil
	}
	path = append(path, node)
	if found := ancestors(node.first, target, path); found != nil || node.first == target {
		return found
	}
	return ancestors(node.second, target, path)
}

// resizePane grows (positive) or shrinks the active pane along one axis.
func (e *editor) resizePane(vertical bool, steps int) {
	p := e.activePane
	if p == nil || p.b == nil {
		return
	}
	path := ancestors(e.layout, p, nil)
	for i := len(path) - 1; i >= 0; i-- {
		node := path[i]
		if node.vertical != vertical {
			continue
		}
		ratio := node.ratio
		if ratio == 0 {
			ratio = 500
		}
		if slices.Contains(node.first.leaves(), p) {
			ratio += steps * resizeStep
		} else {
			ratio -= steps * resizeStep
		}
		node.ratio = max(50, min(950, ratio))
		return
	}
	e.notify("No split to resize in that direction")
}

func (e *editor) equalizePanes() {
	var walk func(*pane)
	walk = func(p *pane) {
		if p == nil || p.b != nil {
			return
		}
		p.ratio = 0
		walk(p.first)
		walk(p.second)
	}
	walk(e.layout)
}

func (e *editor) focusDirection(dir string) {
	var placements []panePlacement
	var edges []paneDivider
	placePanes(e.layout, hitBox{w: 4096, h: 4096}, e.activePane, &placements, &edges)
	var current hitBox
	for _, p := range placements {
		if p.p == e.activePane {
			current = p.box
			break
		}
	}
	var target *pane
	bestGap, bestOffset := 1<<30, 1<<30
	for _, candidate := range placements {
		if candidate.p.preview {
			continue
		}
		n := candidate.box
		gap, offset := 0, 0
		switch dir {
		case "h", "l":
			gap = n.x - (current.x + current.w)
			if dir == "h" {
				gap = current.x - (n.x + n.w)
			}
			if n.y >= current.y+current.h || current.y >= n.y+n.h {
				continue
			}
			offset = absInt(2*(n.y-current.y) + n.h - current.h)
		case "j", "k":
			gap = n.y - (current.y + current.h)
			if dir == "k" {
				gap = current.y - (n.y + n.h)
			}
			if n.x >= current.x+current.w || current.x >= n.x+n.w {
				continue
			}
			offset = absInt(2*(n.x-current.x) + n.w - current.w)
		}
		if gap >= 0 && (gap < bestGap || gap == bestGap && offset < bestOffset) {
			target, bestGap, bestOffset = candidate.p, gap, offset
		}
	}
	if target != nil {
		e.focusPane(target)
	}
}

func absInt(n int) int { return max(n, -n) }

func (e *editor) cyclePane() {
	leaves := e.editableLeaves()
	if len(leaves) > 1 {
		e.focusPane(leaves[(slices.Index(leaves, e.activePane)+1)%len(leaves)])
	}
}

func (e *editor) tabNumber(n int) {
	if n < 1 || n > len(e.buffers) {
		e.notify(fmt.Sprintf("Tab must be between 1 and %d", len(e.buffers)))
		return
	}
	e.switchBuffer(e.buffers[n-1])
}

func (e *editor) cycleTab(delta int) {
	n := len(e.buffers)
	if n == 0 {
		return
	}
	at := slices.Index(e.buffers, e.b)
	e.tabNumber((at+delta%n+n)%n + 1)
}

func (e *editor) moveTab(index int) {
	at := slices.Index(e.buffers, e.b)
	if at < 0 || index < 0 || index >= len(e.buffers) || at == index {
		return
	}
	if at < index {
		copy(e.buffers[at:index], e.buffers[at+1:index+1])
	} else {
		copy(e.buffers[index+1:at+1], e.buffers[index:at])
	}
	e.buffers[index] = e.b
}

func (e *editor) layoutCommands() {
	e.add("split", "Split Right", func(e *editor) { e.splitPane(true, "") })
	e.add("split-below", "Split Below", func(e *editor) { e.splitPane(false, "") })
	e.add("focus-pane", "Cycle Panes", func(e *editor) { e.cyclePane() })
	e.add("close-split", "Close Pane", func(e *editor) { e.closePane(false) })
	e.add("only-pane", "Keep Only Active Pane", func(e *editor) { e.closePane(true) })
	for i, key := range []string{"h", "j", "k", "l"} {
		title := []string{"Focus Pane Left", "Focus Pane Below", "Focus Pane Above", "Focus Pane Right"}[i]
		e.add("pane-"+key, title, func(e *editor) { e.focusDirection(key) })
	}
	e.add("markdown-preview", "Toggle Markdown Preview", func(e *editor) { e.toggleMarkdownPreview() })
	e.add("pane-wider", "Make Pane Wider", func(e *editor) { e.resizePane(true, e.takeCount()) })
	e.add("pane-narrower", "Make Pane Narrower", func(e *editor) { e.resizePane(true, -e.takeCount()) })
	e.add("pane-taller", "Make Pane Taller", func(e *editor) { e.resizePane(false, e.takeCount()) })
	e.add("pane-shorter", "Make Pane Shorter", func(e *editor) { e.resizePane(false, -e.takeCount()) })
	e.add("pane-equalize", "Equalize Pane Sizes", func(e *editor) { e.equalizePanes() })
	e.add("zen", "Toggle Zen Mode", func(e *editor) { e.zen = !e.zen; e.tree.focused = false; e.tree.popup = false })
	e.add("relative-numbers", "Toggle Relative Line Numbers", func(e *editor) { e.relativeNumbers = !e.relativeNumbers })
	e.add("close-explorer", "Close Explorer", func(e *editor) { e.hideExplorer() })
	e.add("tab-next", "Next Tab", func(e *editor) {
		if e.count > 0 {
			e.tabNumber(e.takeCount())
		} else {
			e.cycleTab(1)
		}
	})
	e.add("tab-prev", "Previous Tab", func(e *editor) { e.cycleTab(-e.takeCount()) })
	e.add("tab-first", "First Tab", func(e *editor) { e.tabNumber(1) })
	e.add("tab-last", "Last Tab", func(e *editor) { e.tabNumber(len(e.buffers)) })
	e.add("tab-left", "Move Tab Left", func(e *editor) { e.moveTab(slices.Index(e.buffers, e.b) - e.takeCount()) })
	e.add("tab-right", "Move Tab Right", func(e *editor) { e.moveTab(slices.Index(e.buffers, e.b) + e.takeCount()) })
	for n := 1; n <= 9; n++ {
		label := strconv.Itoa(n)
		e.add("tab-"+label, "Go to Tab "+label, func(e *editor) { e.tabNumber(n) })
	}
}

func (e *editor) hideExplorer() {
	if e.directory != nil {
		e.closeDirectory(false)
		return
	}
	e.tree.sidebar, e.tree.focused, e.tree.popup = false, false, false
}

// Draw a leaf with its private view, then restore the buffer projection used
// by editing commands. This keeps duplicate views of the same file independent.
func (e *editor) drawLayout(v *renderer, box hitBox) {
	var leaves []panePlacement
	var dividers []paneDivider
	placePanes(e.layout, box, e.activePane, &leaves, &dividers)
	v.paneDividers = append(v.paneDividers[:0], dividers...)
	for _, d := range dividers {
		mark := '─'
		if d.vertical {
			mark = '│'
		}
		v.screen.FillArea(d.box.x, d.box.y, d.box.w, d.box.h, mark, borderStyle)
	}
	for _, item := range leaves {
		p := item.p
		b := p.b
		if p.preview {
			v.drawPreview(e, p, item.box)
			continue
		}
		focused := p == e.activePane
		cursor, top, left := b.cursor, b.top, b.left
		if !focused {
			b.cursor, b.top, b.left = min(p.cursor, len(b.text)), p.top, p.left
		}
		n := len(v.panes)
		v.drawPane(e, b, item.box, focused)
		if len(v.panes) > n {
			v.panes[n].leaf = p
		}
		p.cursor, p.top, p.left = b.cursor, b.top, b.left
		if !focused {
			b.cursor, b.top, b.left = cursor, top, left
		}
	}
}

func (v *renderer) paneWheel(e *editor, p paneBox, vertical, horizontal int) {
	b := p.b
	if p.leaf != nil && p.leaf != e.activePane {
		p.leaf.top = max(0, min(max(0, len(b.lines)-p.h), p.leaf.top+vertical))
		p.leaf.left = max(0, p.leaf.left+horizontal)
		return
	}
	e.scrollLock = true
	b.top = max(0, min(max(0, len(b.lines)-p.h), b.top+vertical))
	b.left = max(0, b.left+horizontal)
}

// The session stores only file-backed leaves. Restore prunes missing paths
// and accepts sessions from the old two-pane layout through workspace.go.
type savedPane struct {
	First, Second     *savedPane
	Path              string
	Vertical, Active  bool
	Cursor, Top, Left int
	Ratio             int `json:",omitempty"`
}

func savePaneLayout(p, active *pane) *savedPane {
	if p == nil {
		return nil
	}
	if p.b != nil {
		if p.preview {
			return nil
		}
		if p.b.dashboard {
			if p.dashboardPrevious == nil {
				return nil
			}
			return savePaneLayout(p.dashboardPrevious, p.dashboardPrevious)
		}
		if p.b.path == "" {
			return nil
		}
		return &savedPane{Path: p.b.path, Active: p == active, Cursor: p.cursor, Top: p.top, Left: p.left}
	}
	return &savedPane{First: savePaneLayout(p.first, active), Second: savePaneLayout(p.second, active), Vertical: p.vertical, Ratio: p.ratio}
}

func restorePaneLayout(saved *savedPane, buffers map[string]*buffer, depth int) (root, active *pane) {
	if saved == nil || depth > 32 {
		return nil, nil
	}
	if saved.Path != "" {
		b := buffers[saved.Path]
		if b == nil {
			return nil, nil
		}
		leaf := &pane{b: b, cursor: max(0, min(saved.Cursor, len(b.text))), top: max(0, min(saved.Top, len(b.lines)-1)), left: max(0, saved.Left)}
		if saved.Active {
			active = leaf
		}
		return leaf, active
	}
	a, activeA := restorePaneLayout(saved.First, buffers, depth+1)
	b, activeB := restorePaneLayout(saved.Second, buffers, depth+1)
	if a == nil {
		return b, activeB
	}
	if b == nil {
		return a, activeA
	}
	if activeA == nil {
		activeA = activeB
	}
	return &pane{first: a, second: b, vertical: saved.Vertical, ratio: max(0, min(999, saved.Ratio))}, activeA
}
