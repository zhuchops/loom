package editor

import "strings"

// blockInsert replicates text typed on the first row of a block onto the others
// when Insert mode ends (Vim's visual-block I, A and c).
type blockInsert struct {
	rows        []int
	col         int
	pad         bool
	offset, row int
}

func (e *editor) registerBlockCommands() {
	e.add("select-block", "Visual Block Mode", func(e *editor) {
		b := e.b
		if e.mode == "select" && b.visualBlock {
			e.normal()
			return
		}
		if e.mode != "select" || b.anchor < 0 {
			b.anchor = b.cursor
		}
		e.mode, b.visualLine, b.visualBlock = "select", false, true
	})
	e.add("block-insert", "Insert at Start of Selection", func(e *editor) { e.selectionInsert(false) })
	e.add("block-append", "Append at End of Selection", func(e *editor) { e.selectionInsert(true) })
}

func (b *buffer) columnAt(offset int) int {
	return cellWidth(string(b.text[b.lines[b.rowAt(offset)]:offset]))
}

// blockColumns is the half-open display-column range of the block.
func (b *buffer) blockColumns() (c1, c2 int) {
	a, c := b.columnAt(b.anchor), b.columnAt(b.cursor)
	return min(a, c), max(a, c) + 1
}

// blockSpans is the block's slice of each row it covers.
func (b *buffer) blockSpans() []span {
	c1, c2 := b.blockColumns()
	first, last := b.rowAt(min(b.anchor, b.cursor)), b.rowAt(max(b.anchor, b.cursor))
	spans := make([]span, 0, last-first+1)
	for row := first; row <= last; row++ {
		spans = append(spans, span{cellOffset(b, row, c1), cellOffset(b, row, c2)})
	}
	return spans
}

func (e *editor) blockOperate(op string) {
	b := e.b
	spans := b.blockSpans()
	c1, _ := b.blockColumns()
	firstRow := b.rowAt(min(b.anchor, b.cursor))
	parts := make([]string, len(spans))
	rows := make([]int, len(spans))
	for i, s := range spans {
		parts[i] = string(b.text[s.start:s.end])
		rows[i] = firstRow + i
	}
	e.yanking = op == "yank"
	e.setRegister(strings.Join(parts, b.newline()), false, nil)
	first := spans[0].start
	if op != "yank" {
		var edits []edit
		for _, s := range spans {
			if s.end > s.start {
				edits = append(edits, edit{s, ""})
			}
		}
		if len(edits) > 0 {
			b.replaceAll(edits, first)
		}
	}
	e.normal()
	b.cursor = first
	if op == "change" {
		e.mode = "insert"
		e.blockInsert = &blockInsert{rows: rows, col: c1, offset: first, row: firstRow}
		return
	}
	e.clampModalCursor()
}

// selectionInsert starts Insert at the start (I) or end (A) of the selection.
// For a block, what is typed is copied to every row of it.
func (e *editor) selectionInsert(appendAfter bool) {
	b := e.b
	if b.anchor < 0 {
		return
	}
	if !b.visualBlock {
		s := b.selection()
		at := s.start
		if appendAfter {
			at = s.end
		}
		e.normal()
		e.mode, b.cursor = "insert", min(at, len(b.text))
		return
	}
	c1, c2 := b.blockColumns()
	firstRow, lastRow := b.rowAt(min(b.anchor, b.cursor)), b.rowAt(max(b.anchor, b.cursor))
	rows := make([]int, 0, lastRow-firstRow+1)
	for row := firstRow; row <= lastRow; row++ {
		rows = append(rows, row)
	}
	col := c1
	if appendAfter {
		col = c2
		if width := cellWidth(string(b.text[b.lines[firstRow]:b.lineEnd(firstRow)])); width < col {
			end := b.lineEnd(firstRow)
			b.replaceAll([]edit{{span{end, end}, strings.Repeat(" ", col-width)}}, end)
		}
	}
	at := cellOffset(b, firstRow, col)
	e.normal()
	e.mode, b.cursor = "insert", at
	e.blockInsert = &blockInsert{rows: rows, col: col, pad: appendAfter, offset: at, row: firstRow}
}

// finishBlockInsert copies the text typed on the block's first row onto the rest.
func (e *editor) finishBlockInsert() {
	bi := e.blockInsert
	e.blockInsert = nil
	b := e.b
	if bi == nil || b.cursor <= bi.offset || b.cursor > len(b.text) {
		return
	}
	typed := b.text[bi.offset:b.cursor]
	if strings.ContainsRune(string(typed), '\n') || b.rowAt(bi.offset) != bi.row {
		return
	}
	var edits []edit
	for _, row := range bi.rows {
		if row == bi.row || row >= len(b.lines) {
			continue
		}
		end := b.lineEnd(row)
		width := cellWidth(string(b.text[b.lines[row]:end]))
		switch {
		case width < bi.col && bi.pad:
			edits = append(edits, edit{span{end, end}, strings.Repeat(" ", bi.col-width) + string(typed)})
		case width < bi.col:
		default:
			at := cellOffset(b, row, bi.col)
			edits = append(edits, edit{span{at, at}, string(typed)})
		}
	}
	if len(edits) > 0 {
		cursor := b.cursor
		b.replaceAll(edits, cursor)
	}
}
