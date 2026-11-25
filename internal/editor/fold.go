package editor

// Folds are indentation based: a line opens a fold when the lines below it
// that are indented deeper form a block. This works in every language without
// a language server.

type foldState struct {
	closed  map[int]bool // fold start row -> closed
	lines   int          // line count when a fold was last closed
	version int          // buffer version ends was computed for
	ends    map[int]int  // fold start row -> last row of the block
}

// indentOf returns the visual width of a row's leading whitespace and whether
// the row is blank. Blank rows never end a fold.
func (b *buffer) indentOf(row int) (int, bool) {
	width := 0
	for i := b.lines[row]; i < b.lineEnd(row); i++ {
		switch b.text[i] {
		case ' ':
			width++
		case '\t':
			width += 4
		default:
			return width, false
		}
	}
	return width, true
}

// foldEnds maps each fold start row to its last row. It is cached per version.
func (b *buffer) foldEnds() map[int]int {
	f := &b.folds
	if f.ends != nil && f.version == b.version {
		return f.ends
	}
	ends := map[int]int{}
	for row := range b.lines {
		width, blank := b.indentOf(row)
		if blank {
			continue
		}
		end := row
		for k := row + 1; k < len(b.lines); k++ {
			kw, kb := b.indentOf(k)
			if kb {
				continue
			}
			if kw <= width {
				// A closing bracket at the same indent belongs to the block it closes.
				if kw == width && b.startsWithCloser(k) {
					end = k
				}
				break
			}
			end = k
		}
		if end > row {
			ends[row] = end
		}
	}
	f.ends, f.version = ends, b.version
	return ends
}

// syncFolds drops closed folds when lines were added or removed, because the
// stored row numbers would no longer match the text.
func (b *buffer) syncFolds() {
	if len(b.folds.closed) > 0 && b.folds.lines != len(b.lines) {
		b.folds.closed = nil
	}
}

// hiddenRows marks rows inside closed folds. It returns nil when nothing is folded.
func (b *buffer) hiddenRows() []bool {
	b.syncFolds()
	if len(b.folds.closed) == 0 {
		return nil
	}
	ends := b.foldEnds()
	hidden := make([]bool, len(b.lines))
	for start := range b.folds.closed {
		if end, ok := ends[start]; ok {
			for r := start + 1; r <= end; r++ {
				hidden[r] = true
			}
		}
	}
	return hidden
}

// closedFoldSize reports how many rows a closed fold at row hides.
func (b *buffer) closedFoldSize(row int) (int, bool) {
	if !b.folds.closed[row] {
		return 0, false
	}
	end, ok := b.foldEnds()[row]
	return end - row, ok
}

// visibleStep returns the nearest visible row in direction dir, skipping rows
// hidden by closed folds. It returns row when there is nowhere to go.
func (b *buffer) visibleStep(row, dir int) int {
	hidden := b.hiddenRows()
	target := row + dir
	for target >= 0 && target < len(b.lines) && hidden != nil && hidden[target] {
		target += dir
	}
	if target < 0 || target >= len(b.lines) {
		return row
	}
	return target
}

// foldContaining finds the innermost fold that contains row and passes want.
func (b *buffer) foldContaining(row int, want func(start int) bool) (int, bool) {
	ends := b.foldEnds()
	for s := row; s >= 0; s-- {
		if end, ok := ends[s]; ok && end >= row && want(s) {
			return s, true
		}
	}
	return 0, false
}

// revealCursorRow moves a cursor that landed inside a closed fold, for example
// after a search, onto the fold's first line.
func (b *buffer) revealCursorRow() {
	hidden := b.hiddenRows()
	row := b.rowAt(b.cursor)
	if hidden == nil || !hidden[row] {
		return
	}
	ends := b.foldEnds()
	best := -1
	for s := range b.folds.closed {
		if end, ok := ends[s]; ok && s < row && row <= end && (best < 0 || s < best) {
			best = s
		}
	}
	if best >= 0 {
		b.cursor = motion(b, "first-nonblank", b.lines[best])
	}
}

func (e *editor) foldCommands() {
	e.add("fold-toggle", "Toggle Fold", func(e *editor) {
		b := e.b
		row := b.rowAt(b.cursor)
		start, ok := b.foldContaining(row, func(int) bool { return true })
		if !ok {
			e.notify("No fold here")
			return
		}
		if b.folds.closed[start] {
			delete(b.folds.closed, start)
		} else {
			e.closeFold(start)
		}
	})
	e.add("fold-close", "Close Fold", func(e *editor) {
		b := e.b
		start, ok := b.foldContaining(b.rowAt(b.cursor), func(s int) bool { return !b.folds.closed[s] })
		if !ok {
			e.notify("No open fold here")
			return
		}
		e.closeFold(start)
	})
	e.add("fold-open", "Open Fold", func(e *editor) {
		b := e.b
		start, ok := b.foldContaining(b.rowAt(b.cursor), func(s int) bool { return b.folds.closed[s] })
		if !ok {
			e.notify("No closed fold here")
			return
		}
		delete(b.folds.closed, start)
	})
	e.add("fold-close-all", "Close All Folds", func(e *editor) {
		b := e.b
		if b.folds.closed == nil {
			b.folds.closed = map[int]bool{}
		}
		for start := range b.foldEnds() {
			b.folds.closed[start] = true
		}
		b.folds.lines = len(b.lines)
		b.revealCursorRow()
	})
	e.add("fold-open-all", "Open All Folds", func(e *editor) { e.b.folds.closed = nil })
}

func (e *editor) closeFold(start int) { e.b.closeFold(start) }

func (b *buffer) closeFold(start int) {
	if b.folds.closed == nil {
		b.folds.closed = map[int]bool{}
	}
	b.folds.closed[start] = true
	b.folds.lines = len(b.lines)
	b.revealCursorRow()
}

// toggleFoldAt opens or closes the fold that starts at row, as a click on its
// button in the gutter does. It reports whether there is one.
func (b *buffer) toggleFoldAt(row int) bool {
	if _, ok := b.foldEnds()[row]; !ok {
		return false
	}
	b.syncFolds()
	if b.folds.closed[row] {
		delete(b.folds.closed, row)
	} else {
		b.closeFold(row)
	}
	return true
}

// foldClick handles a press in a pane's gutter: on a fold button it toggles
// the fold and focuses the pane.
func (e *editor) foldClick(pane paneBox, x, y int) bool {
	if len(pane.folds) == 0 || x != pane.foldX {
		return false
	}
	for _, m := range pane.folds {
		if m.y != y {
			continue
		}
		if pane.leaf != nil {
			e.focusPane(pane.leaf)
		}
		e.tree.focused = false
		return pane.b.toggleFoldAt(m.row)
	}
	return false
}

// visibleOffset counts the screen rows between top and row, skipping hidden rows.
func visibleOffset(hidden []bool, top, row int) int {
	n := 0
	for i := top; i < row; i++ {
		if hidden == nil || !hidden[i] {
			n++
		}
	}
	return n
}

func (b *buffer) startsWithCloser(row int) bool {
	for i := b.lines[row]; i < b.lineEnd(row); i++ {
		switch b.text[i] {
		case ' ', '\t':
			continue
		case '}', ')', ']':
			return true
		default:
			return false
		}
	}
	return false
}

// rowAtScreen returns the buffer row shown n screen rows below top, skipping
// rows hidden by closed folds.
func (b *buffer) rowAtScreen(top, n int) int {
	hidden := b.hiddenRows()
	row := top
	for row < len(b.lines)-1 {
		if hidden == nil || !hidden[row] {
			if n == 0 {
				return row
			}
			n--
		}
		row++
	}
	return row
}

// wantEnd is the remembered column after $: always the end of the line.
const wantEnd = 1 << 30
