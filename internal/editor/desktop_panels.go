//go:build desktop

package editor

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"strings"

	"gioui.org/layout"
)

// blend mixes b over a by t (0..1).
func blend(a, b color.NRGBA, t float32) color.NRGBA {
	mix := func(x, y uint8) uint8 { return uint8(float32(x)*(1-t) + float32(y)*t) }
	return color.NRGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), 255}
}

// chip draws a small keycap-style label and returns its width.
func (d *desktop) chip(gtx layout.Context, x, cy int, s string, fg, bg color.NRGBA) int {
	w := d.uiWidth(gtx, s, 11, false)
	pad := d.dp(6)
	d.round(gtx, image.Rect(x, cy-d.dp(9), x+w+2*pad, cy+d.dp(9)), d.dp(5), bg, false)
	d.ui(gtx, x+pad, cy, s, fg, 11, false)
	return w + 2*pad
}

// hintChips draws a shortcut such as "Space q  :close" as keycaps, right-aligned
// at right, and returns the x where it starts.
func (d *desktop) hintChips(gtx layout.Context, right, cy int, hint string, selected bool) int {
	bg := blend(d.color(th.menu, th.bg), d.color(th.fg, th.fg), 0.10)
	if selected {
		bg = blend(d.color(th.menu, th.bg), d.color(th.fg, th.fg), 0.16)
	}
	fg := d.color(th.muted, th.fg)
	groups := strings.Split(hint, "  ")
	x := right
	for i := len(groups) - 1; i >= 0; i-- {
		tokens := strings.Fields(groups[i])
		for j := len(tokens) - 1; j >= 0; j-- {
			w := d.uiWidth(gtx, tokens[j], 11, false) + 2*d.dp(6)
			x -= w
			d.chip(gtx, x, cy, tokens[j], fg, bg)
			x -= d.dp(4)
		}
		x -= d.dp(8)
	}
	return x
}

// hintLine draws "key description" pairs separated by two spaces, left-aligned.
func (d *desktop) hintLine(gtx layout.Context, x, cy int, hint string) {
	bg := blend(d.color(th.menu, th.bg), d.color(th.fg, th.fg), 0.10)
	muted := d.color(th.muted, th.fg)
	for _, group := range strings.Split(hint, "  ") {
		key, desc, _ := strings.Cut(strings.TrimSpace(group), " ")
		if key == "" {
			continue
		}
		x += d.chip(gtx, x, cy, key, d.color(th.fg, th.fg), bg) + d.dp(5)
		x += d.ui(gtx, x, cy, desc, muted, 11, false) + d.dp(14)
	}
}

// treeRows draws the explorer rows between px0 and px1. firstY is the grid row
// of the first visible entry and count how many rows fit.
func (d *desktop) treeRows(gtx layout.Context, e *editor, px0, px1, firstY, count, scroll int) {
	t := &e.tree
	muted := d.color(th.muted, th.fg)
	base := d.color(th.panel, th.bg)
	if t.popup {
		base = d.color(th.menu, th.bg)
	}
	for i := scroll; i < len(t.rows) && i-scroll < count; i++ {
		y := firstY + i - scroll
		row := t.rows[i]
		py := d.rowY(y, d.rows)
		cy := py + d.cellH/2
		hover := d.mx >= px0 && d.mx < px1 && d.my >= py && d.my < py+d.cellH
		inset := d.dp(6)
		switch {
		case i == t.selected && t.focused:
			d.round(gtx, image.Rect(px0+inset, py+1, px1-inset, py+d.cellH-1), d.dp(6), blend(base, d.color(th.accent, th.fg), 0.18), false)
		case hover:
			d.round(gtx, image.Rect(px0+inset, py+1, px1-inset, py+d.cellH-1), d.dp(6), blend(base, d.color(th.fg, th.fg), 0.06), false)
		}
		x := px0 + d.dp(14) + row.depth*d.dp(14)
		fg := d.color(th.fg, th.fg)
		if row.path == e.b.path {
			fg = d.color(th.accent, th.fg)
		}
		if row.dir {
			d.chevron(gtx, x+d.dp(5), cy, t.expanded[row.path] || t.filter != "", muted)
		}
		x += d.dp(16)
		iconColor := muted
		if row.dir {
			iconColor = d.color(th.accent, th.fg)
		}
		d.fileIcon(gtx, x, cy, row.path, row.dir, iconColor)
		x += d.dp(22)
		d.ui(gtx, x, cy, d.fit(gtx, filepath.Base(row.path), px1-x-d.dp(26), 13, false), fg, 13, false)
		if mark := e.git.files[row.path]; mark != 0 {
			d.ui(gtx, px1-d.dp(20), cy, string(mark), d.color(gitColor(mark), th.fg), 11, true)
		}
	}
}

// cardInterior clears everything inside a card's border.
func (d *desktop) cardInterior(gtx layout.Context, fr hitBox) image.Rectangle {
	r := d.columnRect(fr.x+1, fr.x+fr.w-1, fr.y, fr.y+fr.h-1)
	d.fill(gtx, r, d.color(th.menu, th.bg))
	return r
}

func (d *desktop) cardTitle(gtx layout.Context, fr hitBox, r image.Rectangle, title string) {
	d.ui(gtx, r.Min.X+d.dp(14), d.rowY(fr.y, d.rows)+d.cellH/2+d.dp(2), title, d.color(th.muted, th.fg), 12, true)
}

// paintTreePopup draws the explorer opened as a popup (Space e).
func (d *desktop) paintTreePopup(gtx layout.Context, e *editor) {
	v := &d.m.view
	fr := v.treeFrame
	if fr.w == 0 || e.menu != nil {
		return
	}
	r := d.cardInterior(gtx, fr)
	label := e.tree.status()
	d.cardTitle(gtx, fr, r, strings.ToUpper(strings.SplitN(label, " ·", 2)[0]))
	if _, rest, ok := strings.Cut(label, "·"); ok {
		rest = strings.TrimSpace(rest)
		d.ui(gtx, r.Max.X-d.dp(14)-d.uiWidth(gtx, rest, 11, false), d.rowY(fr.y, d.rows)+d.cellH/2+d.dp(2), rest, d.color(th.muted, th.fg), 11, false)
	}
	tb := v.treeBox
	d.treeRows(gtx, e, r.Min.X, r.Max.X, tb.y, tb.h, tb.start)
	d.hintLine(gtx, r.Min.X+d.dp(14), d.rowY(fr.y+fr.h-3, d.rows)+d.cellH/2, "j/k Move  h/l Fold  Enter Open  / Filter  s Sort")
	d.hintLine(gtx, r.Min.X+d.dp(14), d.rowY(fr.y+fr.h-2, d.rows)+d.cellH/2, "a New  r Rename  m Move  x Cut  p Paste  d Trash")
}

// paintSourceControl draws the Git panel with status badges and keycaps.
func (d *desktop) paintSourceControl(gtx layout.Context, e *editor) {
	v := &d.m.view
	p := e.panel
	fr := v.panelFrame
	if p == nil || fr.w == 0 || e.menu != nil {
		return
	}
	r := d.cardInterior(gtx, fr)
	muted, fg := d.color(th.muted, th.fg), d.color(th.fg, th.fg)
	titleY := d.rowY(fr.y, d.rows) + d.cellH/2 + d.dp(2)
	x := r.Min.X + d.dp(14)
	x += d.ui(gtx, x, titleY, "SOURCE CONTROL", muted, 12, true) + d.dp(10)
	if p.branch != "" {
		d.chip(gtx, x, titleY, p.branch, d.color(th.accent, th.fg), blend(d.color(th.menu, th.bg), d.color(th.accent, th.fg), 0.14))
	}
	rows := p.entries()
	height := max(1, fr.h-5)
	if len(rows) == 0 {
		d.ui(gtx, r.Min.X+d.dp(14), d.rowY(fr.y+1, d.rows)+d.cellH/2, "Working tree clean", muted, 13, false)
	}
	for i := p.scroll; i < len(rows) && i < p.scroll+height; i++ {
		row := rows[i]
		y := fr.y + 1 + i - p.scroll
		py := d.rowY(y, d.rows)
		cy := py + d.cellH/2
		if row.header != "" {
			d.ui(gtx, r.Min.X+d.dp(14), cy, strings.ToUpper(row.header), muted, 11, true)
			continue
		}
		if i == p.selected {
			d.round(gtx, image.Rect(r.Min.X+d.dp(6), py+1, r.Max.X-d.dp(6), py+d.cellH-1), d.dp(6), blend(d.color(th.menu, th.bg), d.color(th.accent, th.fg), 0.18), false)
		}
		mark := row.change.work
		if row.staged {
			mark = row.change.index
		}
		if mark == ' ' || mark == 0 || row.change.index == '?' {
			mark = '?'
		}
		badge := d.color(gitColor(byte(mark)), th.fg)
		bx := r.Min.X + d.dp(16)
		d.round(gtx, image.Rect(bx, cy-d.dp(8), bx+d.dp(16), cy+d.dp(8)), d.dp(4), blend(d.color(th.menu, th.bg), badge, 0.22), false)
		d.ui(gtx, bx+d.dp(4), cy, string(mark), badge, 11, true)
		name := filepath.Base(row.change.rel)
		nx := bx + d.dp(26)
		nx += d.ui(gtx, nx, cy, name, fg, 13, false) + d.dp(8)
		if dir := filepath.Dir(row.change.rel); dir != "." {
			d.ui(gtx, nx, cy, d.fit(gtx, dir, r.Max.X-nx-d.dp(14), 12, false), muted, 12, false)
		}
	}
	d.hintLine(gtx, r.Min.X+d.dp(14), d.rowY(fr.y+fr.h-3, d.rows)+d.cellH/2, "s Stage  a Stage all  A Unstage all  d Discard  Enter Diff")
	d.hintLine(gtx, r.Min.X+d.dp(14), d.rowY(fr.y+fr.h-2, d.rows)+d.cellH/2, "c Commit  p Push  P Pull  b Branch  R Refresh  Esc Close")
}

// paintDiff tints added and removed lines and shows the key hints as keycaps.
func (d *desktop) paintDiff(gtx layout.Context, e *editor) {
	v := &d.m.view
	df := e.diff
	fr := v.diffFrame
	if df == nil || fr.w == 0 || e.menu != nil {
		return
	}
	green := color.NRGBA{R: 0x98, G: 0xc3, B: 0x79}
	red := d.color(th.red, th.fg)
	for i := df.scroll; i < len(df.lines) && i < df.scroll+df.height; i++ {
		line := df.lines[i]
		y := fr.y + 1 + i - df.scroll
		py := d.rowY(y, d.rows)
		rect := image.Rect((fr.x+1)*d.cellW, py, (fr.x+fr.w-1)*d.cellW, py+d.cellH)
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			d.fill(gtx, rect, color.NRGBA{R: green.R, G: green.G, B: green.B, A: 34})
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			d.fill(gtx, rect, color.NRGBA{R: red.R, G: red.G, B: red.B, A: 34})
		case strings.HasPrefix(line, "@@"):
			d.fill(gtx, rect, color.NRGBA{R: 0x56, G: 0xb6, B: 0xc2, A: 24})
		}
		if i == df.cursor {
			d.fill(gtx, image.Rect(rect.Min.X, py, rect.Min.X+d.dp(3), py+d.cellH), d.color(th.accent, th.fg))
		}
	}
	rowY := d.rowY(fr.y+fr.h-2, d.rows)
	r := d.columnRect(fr.x+1, fr.x+fr.w-1, fr.y+fr.h-2, fr.y+fr.h-1)
	d.fill(gtx, r, d.color(th.menu, th.bg))
	d.hintLine(gtx, r.Min.X+d.dp(14), rowY+d.cellH/2, fmt.Sprint("]c Next hunk  [c Previous  Enter Open  q Close"))
}

// paintDirectoryTitle restyles the title of the directory editor popup.
func (d *desktop) paintDirectoryTitle(gtx layout.Context, e *editor) {
	box := d.m.view.directoryBox
	if e.directory == nil || box.w == 0 || e.menu != nil {
		return
	}
	fr := hitBox{box.x - 1, box.y - 1, box.w + 2, box.h + 2, 0}
	r := d.columnRect(fr.x+1, fr.x+fr.w-1, fr.y, fr.y+1)
	d.fill(gtx, r, d.color(th.menu, th.bg))
	title := "DIRECTORY  " + e.directory.dir
	if e.directory.doc.b.dirty() {
		title += "  \u25cf"
	}
	d.ui(gtx, r.Min.X+d.dp(14), d.rowY(fr.y, d.rows)+d.cellH/2+d.dp(2), d.fit(gtx, title, r.Dx()-d.dp(28), 12, true), d.color(th.muted, th.fg), 12, true)
}
