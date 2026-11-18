package editor

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

var panelStyle cellStyle
var lineStyle terminalColor
var borderStyle cellStyle

type hitBox struct{ x, y, w, h, start int }

func (b hitBox) contains(x, y int) bool { return x >= b.x && x < b.x+b.w && y >= b.y && y < b.y+b.h }

type paneBox struct {
	hitBox
	b      *buffer
	gutter int
	leaf   *pane
}
type previewBox struct {
	hitBox
	p *pane
}
type tabBox struct {
	hitBox
	b     *buffer
	close int
}
type renderer struct {
	diagnosticHover                             diagnosticHover
	diagnosticBox                               hitBox
	screen                                      *terminalFrame
	treeBox, menuBox, completeBox, directoryBox hitBox
	previewBox                                  hitBox
	panes                                       []paneBox
	previews                                    []previewBox
	barHits                                     []barHit
	paneDividers                                []paneDivider
	paneDrag                                    *paneDivider // Divider being dragged, with the area it splits.
	tabDrag, tabPrev                            *buffer      // Tab being dragged, and the tab that was active before the press.
	tabStartX, tabStartY                        int
	tabMoving                                   bool
	drop                                        dropZone
	tabs                                        []tabBox
	divider                                     int
	leftDown, resize, dragging                  bool
	drag                                        *buffer
	dragStart                                   int
	lastClick                                   time.Time
	lastX, lastY                                int
}

func (v *renderer) draw(e *editor) {
	w, h := v.screen.Size()
	v.screen.Fill(' ', baseStyle)
	v.screen.HideCursor()
	v.treeBox, v.menuBox, v.completeBox, v.directoryBox = hitBox{}, hitBox{}, hitBox{}, hitBox{}
	v.previewBox = hitBox{}
	v.tabs = v.tabs[:0]
	v.panes = v.panes[:0]
	v.paneDividers = v.paneDividers[:0]
	v.previews = v.previews[:0]
	v.divider = -1
	if w < 4 || h < 5 {
		return
	}
	left := 0
	if e.tree.sidebar && !e.zen && w >= 48 {
		left = min(max(16, e.tree.width), w-24)
		v.drawTree(e, hitBox{0, 0, left, h - 1, 0})
		v.divider = left
		v.screen.FillArea(left, 0, 1, h-1, '│', borderStyle)
		left++
	}
	editorBox := hitBox{left, 1, w - left, h - 2, 0}
	if e.zen {
		editorBox.w = min(112, w)
		editorBox.x = (w - editorBox.w) / 2
		editorBox.y, editorBox.h = 0, h-1
		v.drawPane(e, e.b, editorBox, true)
		if len(v.panes) > 0 {
			v.panes[len(v.panes)-1].leaf = e.activePane
		}
	} else {
		v.drawTabs(e, left, w-left)
		e.drawLayout(v, editorBox)
	}
	v.drawStatus(e, w, h)
	if v.tabMoving && v.drop.leaf != nil {
		v.drawDrop()
	}
	v.barHits = v.barHits[:0]
	if e.bar != nil && e.directory == nil {
		v.drawFindBar(e)
	}
	if e.directory != nil {
		v.dim(w, h-1)
		box := centerBox(w, h, min(78, w-2), min(27, h-2))
		title := "Directory · " + e.directory.dir
		if e.directory.doc.b.dirty() {
			title += " ●"
		}
		v.frame(box, title)
		v.directoryBox = hitBox{box.x + 1, box.y + 1, box.w - 2, box.h - 2, 0}
		v.drawPane(e.directory.doc, e.directory.doc.b, v.directoryBox, true)
	} else {
		v.directoryBox = hitBox{}
		if e.tree.popup || e.tree.focused && left == 0 {
			v.dim(w, h-1)
			box := centerBox(w, h, min(68, w-2), min(24, h-2))
			v.frame(box, "Explorer")
			v.drawTree(e, hitBox{box.x + 1, box.y + 1, box.w - 2, box.h - 2, 0})
			v.screen.HideCursor()
		}
		if e.tree.focused {
			v.screen.HideCursor()
		}
	}
	if e.panel != nil {
		v.drawSourceControl(e, w, h)
	}
	if e.diff != nil {
		v.drawDiff(e, w, h)
	}
	if e.notificationDetail != nil {
		v.dim(w, h-1)
		v.drawNotification(e.notificationDetail, w, h)
	} else if e.menu != nil {
		v.dim(w, h-1)
		v.drawMenu(e.menu, w, h)
	} else if v.screen.cursorVisible && e.mode == "insert" && (len(e.completion.items) > 0 || e.signature.label != "") {
		top := 1 // Keep the tab bar and status line visible.
		if e.zen {
			top = 0
		}
		hasSignature := e.signature.label != ""
		if len(e.completion.items) > 0 {
			v.drawCompletion(e.completion, top, h-1, hasSignature)
		}
		if hasSignature {
			v.drawSignature(e.signature, top, h-1)
		}
	} else {
		doc := e
		if e.directory != nil {
			doc = e.directory.doc
		}
		if doc.prefix != "" {
			v.drawWhichKey(doc, w, h)
		}
	}
	v.drawDiagnosticHover(e)
}

func (v *renderer) drawTabs(e *editor, left, width int) {
	v.screen.FillArea(left, 0, width, 1, ' ', panelStyle)
	if len(e.buffers) == 0 {
		return
	}
	tabWidth := max(10, min(24, width/min(5, len(e.buffers))))
	count := max(1, width/tabWidth)
	active := slices.Index(e.buffers, e.b)
	start := max(0, min(active-count+1, len(e.buffers)-count))
	for i := start; i < min(len(e.buffers), start+count); i++ {
		b := e.buffers[i]
		x := left + (i-start)*tabWidth
		size := min(tabWidth, width-(x-left))
		style := panelStyle
		if b == e.b {
			style = baseStyle.Bold(true)
		} else if slices.ContainsFunc(e.layout.leaves(), func(p *pane) bool { return p.b == b }) {
			style = menuStyle
		}
		v.screen.FillArea(x, 0, size, 1, ' ', style)
		name := filepath.Base(b.path)
		if b.dashboard {
			name = "Dashboard"
		} else if b.path == "" {
			name = "Untitled"
		}
		v.write(x+1, 0, size-4, fmt.Sprintf("%d %s", i+1, name), style)
		marker := "×"
		if b.dirty() {
			marker = "●"
		}
		v.write(x+size-2, 0, 1, marker, style)
		v.tabs = append(v.tabs, tabBox{hitBox{x, 0, size, 1, 0}, b, x + size - 2})
	}
}

func (v *renderer) drawTree(e *editor, box hitBox) {
	t := &e.tree
	style := panelStyle
	if t.popup {
		style = menuStyle
	}
	v.screen.FillArea(box.x, box.y, box.w, box.h, ' ', style)
	v.write(box.x+1, box.y, box.w-2, t.status(), headingStyle.Background(styleBackground(t.popup)))
	footer := 1
	if t.popup {
		footer = 2
	}
	height := max(1, box.h-1-footer)
	maxScroll := max(0, len(t.rows)-height)
	t.scroll = min(t.scroll, maxScroll)
	if !t.manualScroll && (t.focused || t.reveal) && (!t.popup || box.x != 0) {
		t.scroll = max(0, max(t.selected-height+1, min(t.selected, t.scroll)))
	}
	if !t.popup || box.x != 0 {
		t.reveal = false
	}
	for i := t.scroll; i < min(len(t.rows), t.scroll+height); i++ {
		row := t.rows[i]
		y := box.y + 1 + i - t.scroll
		s := style
		if row.path == e.b.path {
			s = s.Foreground(th.accent)
		}
		if i == t.selected && t.focused {
			s = activeStyle
			v.screen.FillArea(box.x, y, box.w, 1, ' ', s)
		}
		chevron := " "
		if row.dir {
			chevron = "›"
			if t.expanded[row.path] || t.filter != "" {
				chevron = "⌄"
			}
		}
		prefix := chevron + " " + treeIcon(row.path, row.dir, t.expanded[row.path]) + " "
		indent := min(row.depth*2, max(0, box.w-7))
		name := filepath.Base(row.path)
		v.write(box.x+1+indent, y, box.w-4-indent, prefix+name, s)
		if mark := e.git.files[row.path]; mark != 0 {
			label := string(mark)
			v.write(box.x+box.w-2, y, 1, label, s.Foreground(gitColor(mark)))
		}
	}
	if t.popup {
		v.write(box.x+1, box.y+box.h-2, box.w-2, "j/k Move   h/l Fold   Enter Open   / Filter   s Sort", style)
		v.write(box.x+1, box.y+box.h-1, box.w-2, "a/A New   r Rename   m Move   x/p Cut/Paste   d Trash", style)
	}
	v.treeBox = hitBox{box.x, box.y + 1, box.w, height, t.scroll}
}

func styleBackground(popup bool) terminalColor {
	if popup {
		return th.menu
	}
	return th.panel
}

func gitColor(mark byte) terminalColor {
	switch mark {
	case '+', 'A', '?':
		return th.accent
	case '-', 'D':
		return th.red
	}
	return th.yellow
}

func (v *renderer) drawPane(e *editor, b *buffer, box hitBox, focused bool) {
	gutter := min(box.w-1, len(fmt.Sprint(len(b.lines)))+3)
	if box.w < 3 {
		return
	}
	if b.dashboard {
		v.panes = append(v.panes, paneBox{hitBox: box, b: b})
		v.drawDashboard(e, box, focused)
		return
	}
	v.panes = append(v.panes, paneBox{hitBox: box, b: b, gutter: gutter})
	if b.highlightVersion != b.version || b.highlightPath != b.path {
		b.highlightVersion, b.highlightPath = b.version, b.path
		b.lexical = highlightSyntax(b, e.cfg.languageFor(b.path))

	}
	if focused {
		b.revealCursorRow()
	}
	row := b.rowAt(b.cursor)
	col := cellWidth(string(b.text[b.lines[row]:b.cursor]))
	if focused && !e.scrollLock {
		b.top = max(0, max(row-box.h+1, min(row, b.top)))
		b.left = max(0, max(col-(box.w-gutter-1), min(col, b.left)))
	}
	b.top = min(b.top, len(b.lines)-1)
	selected := span{}
	if focused && b.anchor >= 0 {
		selected = b.selection()
	}
	firstSelectedRow, lastSelectedRow := -1, -1
	if focused && b.visualLine && b.anchor >= 0 {
		firstSelectedRow, lastSelectedRow = b.rowAt(min(b.anchor, b.cursor)), b.rowAt(max(b.anchor, b.cursor))
	}
	flash := e.surroundFlash
	flashing := flash.buffer == b && flash.version == b.version && time.Now().Before(flash.until)
	semantic := b.semantic
	if b.semanticVersion != b.version || b.semanticPath != b.path {
		semantic = nil
	}
	first := b.lines[b.top]
	lexIndex, semIndex := seekHighlight(b.lexical, first), seekHighlight(semantic, first)

	last := min(len(b.lines), b.top+box.h)
	problems := diagnosticRows(b, b.top, last)
	var searchSpans *searchResult
	if e.searchVisible && e.query != "" {
		res := e.matches(b)
		searchSpans = &res
	}
	hidden := b.hiddenRows()
	foldable := b.foldEnds()
	// screenY counts screen rows, so rows hidden by closed folds take no space.
	screenY := box.y - 1
	for i := b.top; i < len(b.lines) && screenY+1 < box.y+box.h; i++ {
		if hidden != nil && hidden[i] {
			continue
		}
		screenY++
		y := screenY
		bg := th.bg
		if i == row && focused {
			bg = lineStyle
			v.screen.FillArea(box.x, y, box.w, 1, ' ', baseStyle.Background(bg))
		}
		if i >= firstSelectedRow && i <= lastSelectedRow {
			bg = selectStyle
			v.screen.FillArea(box.x, y, box.w, 1, ' ', baseStyle.Background(bg))
		}
		number := mutedStyle.Background(bg)
		if i == row && focused {
			number = number.Foreground(th.fg)
		}
		lineNumber := i + 1
		if e.relativeNumbers && i != row {
			lineNumber = absInt(i - row)
		}
		v.write(box.x+1, y, gutter-2, fmt.Sprintf("%*d", gutter-2, lineNumber), number)
		// The blank column between the number and the text shows fold state:
		// ▾ for a fold that is open, ▸ for one that is closed.
		if _, ok := foldable[i]; ok && gutter >= 3 {
			glyph := "▾"
			if b.folds.closed[i] {
				glyph = "▸"
			}
			v.write(box.x+gutter-1, y, 1, glyph, number.Foreground(th.accent))
		}
		if severity := diagnosticAt(problems[i], b.lines[i], b.lineEnd(i)+1); severity != 0 {
			v.write(box.x, y, 1, "●", number.Foreground(diagnosticColor(severity)))
		} else if mark := e.git.lines[i]; e.git.path == b.path && e.git.version == b.version && mark != 0 {
			v.write(box.x, y, 1, "▎", number.Foreground(gitColor(mark)))
		}
		column, offset := 0, b.lines[i]
		var matches []span
		if searchSpans != nil {
			matches = searchSpans.matchRange(offset, b.lineEnd(i))
		}
		graphemes := uniseg.NewGraphemes(string(b.text[offset:b.lineEnd(i)]))
		for graphemes.Next() {
			g := graphemes.Str()
			width := graphemes.Width()
			if g == "\t" {
				width = 4 - column%4
				g = strings.Repeat(" ", width)
			}
			if width == 0 {
				width = 1
				g = "�"
			}
			s := baseStyle
			if next, ok := styleAt(b.lexical, &lexIndex, offset); ok {
				s = next
			}
			if next, ok := styleAt(semantic, &semIndex, offset); ok {
				s = next
			}
			s = s.Background(bg)
			for len(matches) > 0 && matches[0].end <= offset {
				matches = matches[1:]
			}
			if len(matches) > 0 && matches[0].start <= offset {
				s = s.Background(th.match)
				if focused && matches[0].start == b.cursor {
					s = s.Background(th.flash)
				}
			}
			next := offset + utf8.RuneCountInString(graphemes.Str())
			if offset < selected.end && next > selected.start {
				s = s.Background(selectStyle)
			}
			if flashing && (offset < flash.inner.start && next > flash.outer.start || offset < flash.outer.end && next > flash.inner.end) {
				s = s.Background(th.flash)
			}
			s = markDiagnostic(s, problems[i], offset, next)
			x := box.x + gutter + column - b.left
			if x >= box.x+gutter && x+width <= box.x+box.w {
				v.write(x, y, width, g, s)
			}
			column += width
			offset = next
			if x >= box.x+box.w {
				break
			}
		}
		if size, ok := b.closedFoldSize(i); ok {
			if x := box.x + gutter + column - b.left + 1; x < box.x+box.w {
				v.write(x, y, box.x+box.w-x, fmt.Sprintf("⋯ %d lines", size), mutedStyle.Background(bg))
			}
		}
		end := b.lineEnd(i)
		if offset == end {
			x := box.x + gutter + column - b.left
			if x >= box.x+gutter && x < box.x+box.w {
				if s := markDiagnostic(baseStyle.Background(bg), problems[i], end, end+1); s.underline != 0 {
					v.screen.Put(x, y, " ", 1, s)
				}
			}
		}
	}
	if !focused || e.tree.focused || e.menu != nil {
		return
	}
	x, y := box.x+gutter+col-b.left, box.y+visibleOffset(hidden, b.top, row)
	if !box.contains(x, y) || x < box.x+gutter {
		return
	}
	cursor := cursorBlock
	if e.mode == "insert" {
		cursor = cursorBar
	}
	v.screen.SetCursorStyle(cursor)
	v.screen.ShowCursor(x, y)
}

func seekHighlight(tokens []highlight, offset int) int {
	at, _ := slices.BinarySearchFunc(tokens, offset, func(h highlight, n int) int {
		if h.end <= n {
			return -1
		}
		return 1
	})
	return at
}

func styleAt(tokens []highlight, i *int, offset int) (cellStyle, bool) {
	for *i < len(tokens) && tokens[*i].end <= offset {
		(*i)++
	}
	if *i < len(tokens) && tokens[*i].start <= offset {
		return tokens[*i].style, true
	}
	return baseStyle, false
}
