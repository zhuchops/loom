//go:build desktop

package editor

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// The editor lays itself out on a character grid. The desktop build keeps
// that grid as the layout engine (so every key, click and command behaves
// as before) but gives the tab bar and status bar their own pixel heights
// and paints the chrome as real UI: sans-serif text, rounded shapes, hover
// states and one-pixel lines.

func (d *desktop) rowY(y, rows int) int {
	switch {
	case y >= rows-1:
		return d.height - d.bottom
	case d.top == 0:
		return y * d.cellH
	case y == 0:
		return 0
	}
	return d.top + (y-1)*d.cellH
}

func (d *desktop) rowH(y, rows int) int {
	switch {
	case y >= rows-1:
		return d.bottom
	case d.top != 0 && y == 0:
		return d.top
	}
	return d.cellH
}

// cellRow maps a pixel row to the editor's grid row.
func (d *desktop) cellRow(py int) int {
	rows := d.rows
	switch {
	case py >= d.height-d.bottom:
		return rows - 1
	case d.top == 0:
		return min(rows-2, max(0, py/d.cellH))
	case py < d.top:
		return 0
	}
	return min(rows-2, 1+(py-d.top)/d.cellH)
}

func (d *desktop) fill(gtx layout.Context, r image.Rectangle, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
}

func (d *desktop) round(gtx layout.Context, r image.Rectangle, radius int, c color.NRGBA, topOnly bool) {
	rr := clip.RRect{Rect: r, NE: radius, NW: radius}
	if !topOnly {
		rr.SE, rr.SW = radius, radius
	}
	paint.FillShape(gtx.Ops, c, rr.Op(gtx.Ops))
}

func (d *desktop) stroke(gtx layout.Context, c color.NRGBA, width float32, build func(p *clip.Path)) {
	var p clip.Path
	p.Begin(gtx.Ops)
	build(&p)
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: width}.Op())
}

// label lays out one line of text and returns the macro that draws it and its size.
func (d *desktop) label(gtx layout.Context, f fontChoice, s string, c color.NRGBA, sp float32) (op.CallOp, image.Point) {
	gtx.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
	color := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	call := color.Stop()
	rec := op.Record(gtx.Ops)
	face := d.uiFont
	switch f {
	case uiBold:
		face = d.uiBold
	case iconFont:
		face = d.fontIcon
	}
	dims := widget.Label{MaxLines: 1}.Layout(gtx, d.shaper, face, unit.Sp(sp), s, call)
	return rec.Stop(), dims.Size
}

type fontChoice int

const (
	uiRegular fontChoice = iota
	uiBold
	iconFont
)

// ui draws a line of interface text starting at x and centred on cy.
func (d *desktop) ui(gtx layout.Context, x, cy int, s string, c color.NRGBA, sp float32, bold bool) int {
	choice := uiRegular
	if bold {
		choice = uiBold
	}
	draw, size := d.label(gtx, choice, s, c, sp)
	defer op.Offset(image.Pt(x, cy-size.Y/2)).Push(gtx.Ops).Pop()
	draw.Add(gtx.Ops)
	return size.X
}

func (d *desktop) uiWidth(gtx layout.Context, s string, sp float32, bold bool) int {
	choice := uiRegular
	if bold {
		choice = uiBold
	}
	_, size := d.label(gtx, choice, s, color.NRGBA{}, sp)
	return size.X
}

// fit shortens s with an ellipsis so it is at most width pixels wide.
func (d *desktop) fit(gtx layout.Context, s string, width int, sp float32, bold bool) string {
	if d.uiWidth(gtx, s, sp, bold) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 1 {
		r = r[:len(r)-1]
		if t := string(r) + "…"; d.uiWidth(gtx, t, sp, bold) <= width {
			return t
		}
	}
	return "…"
}

func (d *desktop) icon(gtx layout.Context, x, cy int, s string, c color.NRGBA, sp float32) {
	draw, size := d.label(gtx, iconFont, s, c, sp)
	defer op.Offset(image.Pt(x, cy-size.Y/2)).Push(gtx.Ops).Pop()
	draw.Add(gtx.Ops)
}

func (d *desktop) modal(e *editor) bool {
	return e.menu != nil || e.notificationDetail != nil || e.directory != nil || e.tree.popup || e.panel != nil || e.diff != nil
}

// cellSource is anything the grid can be painted from: the live frame, or a
// snapshot of part of an earlier one.
type cellSource interface {
	Get(x, y int) (string, cellStyle, int)
}

type snapCell struct {
	s     string
	style cellStyle
	w     int
}

type frameCells struct {
	x0, y0, w, h int
	cells        []snapCell
}

func takeSnapshot(f *terminalFrame, r hitBox) *frameCells {
	s := &frameCells{x0: r.x, y0: r.y, w: r.w, h: r.h, cells: make([]snapCell, r.w*r.h)}
	for y := range r.h {
		for x := range r.w {
			c, st, w := f.Get(r.x+x, r.y+y)
			s.cells[y*r.w+x] = snapCell{c, st, w}
		}
	}
	return s
}

func (s *frameCells) Get(x, y int) (string, cellStyle, int) {
	x, y = x-s.x0, y-s.y0
	if x < 0 || y < 0 || x >= s.w || y >= s.h {
		return " ", cellStyle{}, 1
	}
	c := s.cells[y*s.w+x]
	return c.s, c.style, c.w
}

// scrollAnim glides the text of the focused pane when its first line changes.
type scrollAnim struct {
	buf     *buffer
	rect    hitBox
	lastTop int
	last    *frameCells
	old     *frameCells
	oldTop  int
	total   int
	p0      float64
	start   time.Time
	active  bool
}

const scrollTime = 140 * time.Millisecond

func (a *scrollAnim) progress(now time.Time) float64 {
	t := min(1, float64(now.Sub(a.start))/float64(scrollTime))
	ease := 1 - (1-t)*(1-t)*(1-t)
	return a.p0 + (float64(a.total)-a.p0)*ease
}

func (d *desktop) activePane() *paneBox {
	e := d.m.editor
	for i := range d.m.view.panes {
		p := &d.m.view.panes[i]
		if p.leaf == e.activePane && p.b != nil && !p.b.dashboard {
			return p
		}
	}
	return nil
}

func (d *desktop) updateScroll(now time.Time, pane *paneBox) {
	sa := &d.scroll
	if pane == nil || d.modal(d.m.editor) {
		*sa = scrollAnim{}
		return
	}
	top := pane.b.top
	if sa.buf != pane.b || sa.rect != pane.hitBox {
		*sa = scrollAnim{buf: pane.b, rect: pane.hitBox, lastTop: top}
		return
	}
	if top == sa.lastTop {
		return
	}
	old, oldTop, p := sa.last, sa.lastTop, 0.0
	if sa.active {
		old, oldTop, p = sa.old, sa.oldTop, sa.progress(now)
	}
	total := top - oldTop
	if old != nil && animationsOn && total != 0 && total < pane.h && -total < pane.h && len(d.m.view.cards) == 0 {
		*sa = scrollAnim{buf: sa.buf, rect: sa.rect, lastTop: top, last: sa.last, old: old, oldTop: oldTop, total: total, p0: p, start: now, active: true}
	} else {
		sa.active = false
	}
	sa.lastTop = top
}

// cursorAnim slides the cursor between cells instead of jumping.
type cursorAnim struct {
	x, y, fx, fy, tx, ty float32
	start                time.Time
	active, valid        bool
}

const cursorTime = 60 * time.Millisecond

func (d *desktop) updateCursor(now time.Time, f *terminalFrame) {
	c := &d.cur
	if !f.cursorVisible || f.cursorY >= d.rows {
		c.valid, c.active = false, false
		return
	}
	dy := 0
	sa := &d.scroll
	if sa.active {
		dy = int((float64(sa.total) - sa.progress(now)) * float64(d.cellH))
	}
	tx := float32(f.cursorX * d.cellW)
	ty := float32(d.rowY(f.cursorY, d.rows) + (d.rowH(f.cursorY, d.rows)-d.cellH)/2 + dy)
	far := absf(tx-c.tx) > float32(24*d.cellW) || absf(ty-c.ty) > float32(10*d.cellH)
	if !c.valid || sa.active || d.modal(d.m.editor) || far || !animationsOn {
		*c = cursorAnim{x: tx, y: ty, tx: tx, ty: ty, valid: true}
		return
	}
	if tx != c.tx || ty != c.ty {
		c.fx, c.fy, c.start, c.active = c.x, c.y, now, true
		c.tx, c.ty = tx, ty
	}
	if c.active {
		t := min(1, float32(now.Sub(c.start))/float32(cursorTime))
		ease := 1 - (1-t)*(1-t)
		c.x, c.y = c.fx+(c.tx-c.fx)*ease, c.fy+(c.ty-c.fy)*ease
		if t >= 1 {
			c.active, c.x, c.y = false, c.tx, c.ty
		} else {
			d.w.Invalidate()
		}
	}
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

type cellRange struct{ x0, y0, x1, y1 int }

func (d *desktop) onRing(x, y int) (ring, corner bool) {
	for _, c := range d.m.view.cards {
		if x < c.x || x >= c.x+c.w || y < c.y || y >= c.y+c.h {
			continue
		}
		left, right, top, bottom := x == c.x, x == c.x+c.w-1, y == c.y, y == c.y+c.h-1
		if left || right || top || bottom {
			return true, (left || right) && (top || bottom)
		}
	}
	return false, false
}

func (d *desktop) inCard(x, y int) bool {
	for _, c := range d.m.view.cards {
		if x >= c.x && x < c.x+c.w && y >= c.y && y < c.y+c.h {
			return true
		}
	}
	return false
}

// chromeCell reports cells that the sidebar and tab bar paint natively, so the
// grid must leave them alone (unless a popup sits on top of them).
func (d *desktop) chromeCell(x, y int) bool {
	e := d.m.editor
	if e.zen || d.inCard(x, y) {
		return false
	}
	v := &d.m.view
	if ds := v.dash; ds.box.w > 0 && x >= ds.box.x && x < ds.box.x+ds.box.w && y >= ds.box.y && y < ds.box.y+ds.box.h {
		return true // the start screen is painted natively, underneath any popup
	}
	if v.divider > 0 && x <= v.divider && y < d.rows-1 {
		return true
	}
	return y == 0 && x > v.divider
}

func isFrameChar(s string) bool {
	switch s {
	case "│", "─", "╭", "╮", "╰", "╯":
		return true
	}
	return false
}

func (d *desktop) cardRect(c hitBox) image.Rectangle {
	rows := d.rows
	y0 := d.rowY(min(c.y, rows-1), rows)
	y1 := d.rowY(min(c.y+c.h-1, rows-1), rows) + d.cellH
	return image.Rect(c.x*d.cellW, y0, (c.x+c.w)*d.cellW, y1)
}

// paintCards draws the shadow and body of every framed popup behind the grid.
func (d *desktop) paintCards(gtx layout.Context) {
	for _, c := range d.m.view.cards {
		r := d.cardRect(c)
		for i, alpha := range []uint8{14, 10, 7, 4} {
			grow := d.dp(2 + 3*i)
			shadow := image.Rect(r.Min.X-grow, r.Min.Y-grow+d.dp(5), r.Max.X+grow, r.Max.Y+grow+d.dp(5))
			d.round(gtx, shadow, d.dp(8)+grow, color.NRGBA{A: alpha}, false)
		}
		d.round(gtx, r, d.dp(8), d.color(th.menu, th.bg), false)
	}
}

func (d *desktop) paintCardBorders(gtx layout.Context) {
	for _, c := range d.m.view.cards {
		rr := clip.UniformRRect(d.cardRect(c).Inset(0), d.dp(8))
		paint.FillShape(gtx.Ops, d.color(th.border, th.border), clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(max(1, d.dp(1)))}.Op())
	}
}

func (d *desktop) paintCells(gtx layout.Context, src cellSource, r cellRange, dy int, main bool) {
	rows := d.rows
	skipBg := func(x, y int) bool {
		if !main {
			return false
		}
		if d.chromeCell(x, y) {
			return true
		}
		_, corner := d.onRing(x, y)
		return corner
	}
	for y := r.y0; y < r.y1 && y < rows; y++ {
		py, ph := d.rowY(y, rows)+dy, d.rowH(y, rows)
		for x := r.x0; x < r.x1; {
			_, style, width := src.Get(x, y)
			width = max(1, width)
			if skipBg(x, y) {
				x += width
				continue
			}
			end := x + width
			for end < r.x1 {
				_, next, w := src.Get(end, y)
				if next.bg != style.bg || skipBg(end, y) {
					break
				}
				end += max(1, w)
			}
			if style.bg != th.bg {
				rect := image.Rect(x*d.cellW, py, end*d.cellW, py+ph)
				if main && style.bg == th.sel {
					d.round(gtx, rect, d.dp(3), d.color(style.bg, th.bg), false)
				} else {
					d.fill(gtx, rect, d.color(style.bg, th.bg))
				}
			}
			x = end
		}
	}
	f, _ := src.(*terminalFrame)
	cursorX, cursorY := -1, -1
	if main && f != nil && f.cursorVisible {
		cursorX, cursorY = f.cursorX, f.cursorY
		if cursorX >= r.x0 && cursorX < r.x1 && cursorY >= r.y0 && cursorY < r.y1 && d.cur.valid {
			rect := image.Rect(int(d.cur.x), int(d.cur.y), int(d.cur.x)+d.cellW, int(d.cur.y)+d.cellH)
			col := d.color(th.fg, th.fg)
			if f.cursorShape == cursorBar {
				rect.Max.X = rect.Min.X + max(2, d.cellW/6)
			} else if d.cur.active {
				col.A = 150
			}
			d.fill(gtx, rect, col)
		}
	}
	for y := r.y0; y < r.y1 && y < rows; y++ {
		py := d.rowY(y, rows) + dy + (d.rowH(y, rows)-d.cellH)/2
		for x := r.x0; x < r.x1; {
			s, style, width := src.Get(x, y)
			width = max(1, width)
			if s == "" || s == " " {
				x += width
				continue
			}
			if main && d.chromeCell(x, y) {
				x += width
				continue
			}
			if main {
				if ring, _ := d.onRing(x, y); ring && isFrameChar(s) {
					x += width
					continue
				}
			}
			fg := style.fg
			if x == cursorX && y == cursorY && f != nil && f.cursorShape != cursorBar && !d.cur.active {
				fg = style.bg
				if fg&defaultFlag != 0 {
					fg = th.bg
				}
			}
			col := d.color(fg, th.fg)
			if d.lineChar(gtx, x*d.cellW, py, s, col) {
				x += width
				continue
			}
			ascii := len(s) == 1 && s[0] > 0x20 && s[0] < 0x7f
			if !ascii {
				d.glyph(gtx, x*d.cellW, py, s, col, style.bold)
				if style.underline != 0 {
					d.fill(gtx, image.Rect(x*d.cellW, py+d.cellH-2, (x+width)*d.cellW, py+d.cellH-1), d.color(style.underline, th.fg))
				}
				x += width
				continue
			}
			var run strings.Builder
			start := x
			for x < r.x1 {
				t, st, w := src.Get(x, y)
				onCursor := x == cursorX && y == cursorY
				if st.fg != style.fg || st.bold != style.bold || w > 1 || len(t) != 1 || t[0] < 0x20 || t[0] >= 0x7f || onCursor && start != x {
					break
				}
				run.WriteString(t)
				x++
				if onCursor {
					break
				}
			}
			d.glyph(gtx, start*d.cellW, py, run.String(), col, style.bold)
			if style.underline != 0 {
				d.fill(gtx, image.Rect(start*d.cellW, py+d.cellH-2, x*d.cellW, py+d.cellH-1), d.color(style.underline, th.fg))
			}
		}
	}
}

func (d *desktop) paint(gtx layout.Context, size image.Point) {
	f := d.m.view.screen
	e := d.m.editor
	rows := d.rows
	full := cellRange{0, 0, f.Width(), min(f.Height(), rows)}
	d.fill(gtx, image.Rectangle{Max: size}, d.color(th.bg, th.bg))
	pane := d.activePane()
	d.updateScroll(gtx.Now, pane)
	d.updateCursor(gtx.Now, f)
	if !e.zen {
		d.paintSidebar(gtx, e)
		d.paintTabs(gtx, e)
		if d.modal(e) {
			// Match the grid's own dimming behind popups.
			v := &d.m.view
			dim := color.NRGBA{A: 115}
			if v.divider > 0 {
				d.fill(gtx, image.Rect(0, 0, v.divider*d.cellW+1, d.height-d.bottom), dim)
			}
			d.fill(gtx, image.Rect(max(0, v.divider*d.cellW+1), 0, d.width, d.top), dim)
		}
	}
	d.paintDashboard(gtx, e)
	if ds := d.m.view.dash; ds.box.w > 0 && d.modal(e) {
		d.fill(gtx, d.columnRect(ds.box.x, ds.box.x+ds.box.w, ds.box.y, ds.box.y+ds.box.h), color.NRGBA{A: 115})
	}
	d.paintCards(gtx)
	d.paintCells(gtx, f, full, 0, true)
	sa := &d.scroll
	if sa.active {
		r := cellRange{pane.x, pane.y, pane.x + pane.w, pane.y + pane.h}
		area := image.Rect(pane.x*d.cellW, d.rowY(pane.y, rows), (pane.x+pane.w)*d.cellW, d.rowY(pane.y+pane.h-1, rows)+d.cellH)
		clipArea := clip.Rect(area).Push(gtx.Ops)
		d.fill(gtx, area, d.color(th.bg, th.bg))
		p := sa.progress(gtx.Now)
		d.paintCells(gtx, sa.old, r, -int(p*float64(d.cellH)), false)
		d.paintCells(gtx, f, r, int((float64(sa.total)-p)*float64(d.cellH)), true)
		clipArea.Pop()
		if gtx.Now.Sub(sa.start) >= scrollTime {
			sa.active = false
		}
		d.w.Invalidate()
	}
	if pane != nil && !sa.active {
		sa.last = takeSnapshot(f, pane.hitBox)
	}
	// A hairline above the status bar.
	d.fill(gtx, image.Rect(0, d.height-d.bottom, size.X, d.height-d.bottom+1), d.color(th.border, th.border))
	d.paintTitles(gtx)
	d.paintMenu(gtx, e)
	d.paintCompletion(gtx, e)
	if v := &d.m.view; e.notificationDetail != nil && len(v.cards) > 0 {
		d.paintProse(gtx, f, v.cards[len(v.cards)-1])
	} else if v.diagnosticBox.w > 0 {
		d.paintProse(gtx, f, v.diagnosticBox)
	}
	d.paintWhichKey(gtx, e)
	d.paintFindBar(gtx, e)
	d.paintTreePopup(gtx, e)
	d.paintDirectoryTitle(gtx, e)
	d.paintSourceControl(gtx, e)
	d.paintDiff(gtx, e)
	d.paintCardBorders(gtx)
	d.paintStatus(gtx, e)
}

func (d *desktop) columnRect(x0, x1, y0, y1 int) image.Rectangle {
	rows := d.rows
	return image.Rect(x0*d.cellW, d.rowY(y0, rows), x1*d.cellW, d.rowY(y1-1, rows)+d.cellH)
}

func (d *desktop) magnifier(gtx layout.Context, x, cy int, c color.NRGBA) {
	k := d.scale
	cx, cyf := float32(x)+5*k, float32(cy)-1*k
	d.stroke(gtx, c, 1.4*k, func(p *clip.Path) {
		const n = 16
		for i := 0; i <= n; i++ {
			a := float64(i) / n * 2 * 3.14159265
			pt := f32.Pt(cx+5*k*float32(math.Cos(a)), cyf+5*k*float32(math.Sin(a)))
			if i == 0 {
				p.MoveTo(pt)
			} else {
				p.LineTo(pt)
			}
		}
		p.MoveTo(f32.Pt(cx+3.6*k, cyf+3.6*k))
		p.LineTo(f32.Pt(cx+7.5*k, cyf+7.5*k))
	})
}

// matchRunes returns which runes of label a fuzzy query matches, in order.
func matchRunes(label, query string) map[int]bool {
	query = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(query), ":")))
	out := map[int]bool{}
	if query == "" {
		return out
	}
	q := []rune(query)
	k := 0
	for i, r := range []rune(strings.ToLower(label)) {
		if k < len(q) && r == q[k] {
			out[i] = true
			k++
		}
	}
	if k < len(q) {
		return map[int]bool{}
	}
	return out
}

// highlighted draws label with the matched letters in the accent colour.
func (d *desktop) highlighted(gtx layout.Context, x, cy int, label string, hit map[int]bool, fg, accent color.NRGBA, sp float32) {
	runes := []rune(label)
	for i := 0; i < len(runes); {
		j := i
		for j < len(runes) && hit[j] == hit[i] {
			j++
		}
		seg := string(runes[i:j])
		if hit[i] {
			x += d.ui(gtx, x, cy, seg, accent, sp, true)
		} else {
			x += d.ui(gtx, x, cy, seg, fg, sp, false)
		}
		i = j
	}
}

// paintMenu draws the command palette and other pickers: an input row and a
// list with rounded selection, in the interface font.
func (d *desktop) paintMenu(gtx layout.Context, e *editor) {
	v := &d.m.view
	m := e.menu
	if m == nil || e.notificationDetail != nil || m.confirmation || v.menuFrame.w == 0 {
		return
	}
	fr, list := v.menuFrame, v.menuBox
	x0, x1 := list.x, list.x+list.w
	inputRow := fr.y + 1
	card := d.color(th.menu, th.bg)
	block := d.columnRect(x0, x1, inputRow, max(list.y+list.h, inputRow+1))
	d.fill(gtx, block, card)
	// The input sits on its own darker band, separated from the list by a rule.
	inputBand := d.columnRect(x0, x1, inputRow, inputRow+1)
	d.fill(gtx, inputBand, blend(card, d.color(th.bg, th.bg), 0.5))
	muted, fg := d.color(th.muted, th.fg), d.color(th.fg, th.fg)
	accent := d.color(th.accent, th.fg)
	px0, px1 := block.Min.X, block.Max.X
	titleCY := d.rowY(fr.y, d.rows) + d.cellH/2 + d.dp(2)
	if m.title != "" {
		title := d.columnRect(fr.x+1, fr.x+fr.w-1, fr.y, fr.y+1)
		d.fill(gtx, title, card)
		d.ui(gtx, title.Min.X+d.dp(14), titleCY, strings.ToUpper(m.title), muted, 11, true)
		if n := len(m.matches); n > 0 {
			count := fmt.Sprintf("%d", n)
			if len(m.options) != n {
				count = fmt.Sprintf("%d of %d", n, len(m.options))
			}
			d.ui(gtx, title.Max.X-d.dp(14)-d.uiWidth(gtx, count, 11, false), titleCY, count, muted, 11, false)
		}
	}
	cy := d.rowY(inputRow, d.rows) + d.cellH/2
	d.magnifier(gtx, px0+d.dp(14), cy, muted)
	tx := px0 + d.dp(40)
	caret := func(x int) {
		d.fill(gtx, image.Rect(x, cy-d.dp(9), x+max(2, d.dp(1)), cy+d.dp(9)), accent)
	}
	if m.input == "" {
		caret(tx)
		hint := "Search…"
		if strings.Contains(strings.ToLower(m.title), "command") {
			hint = "Type a command…"
		}
		d.ui(gtx, tx+d.dp(6), cy, hint, muted, 16, false)
	} else {
		w := d.ui(gtx, tx, cy, d.fit(gtx, m.input, px1-tx-d.dp(60), 16, false), fg, 16, false)
		caret(tx + w + d.dp(1))
	}
	d.chip(gtx, px1-d.dp(14)-d.uiWidth(gtx, "esc", 11, false)-2*d.dp(6), cy, "esc", muted, blend(card, d.color(th.fg, th.fg), 0.08))
	rule := d.rowY(inputRow, d.rows) + d.cellH
	d.fill(gtx, image.Rect(px0, rule-max(2, d.dp(2)), px1, rule), blend(card, d.color(th.fg, th.fg), 0.26))
	if len(m.matches) == 0 && m.empty != "" {
		d.ui(gtx, px0+d.dp(16), d.rowY(list.y, d.rows)+d.cellH/2, m.empty, muted, 13, false)
	}
	for i := 0; i < list.h && list.start+i < len(m.matches); i++ {
		o := m.matches[list.start+i]
		py := d.rowY(list.y+i, d.rows)
		cy := py + d.cellH/2
		selected := list.start+i == m.selected
		hover := d.mx >= px0 && d.mx < px1 && d.my >= py && d.my < py+d.cellH
		switch {
		case selected:
			d.round(gtx, image.Rect(px0+d.dp(8), py+d.dp(1), px1-d.dp(8), py+d.cellH-d.dp(1)), d.dp(7), blend(card, accent, 0.20), false)
			d.fill(gtx, image.Rect(px0+d.dp(10), py+d.dp(8), px0+d.dp(10)+max(2, d.dp(2)), py+d.cellH-d.dp(8)), accent)
		case hover:
			d.round(gtx, image.Rect(px0+d.dp(8), py+d.dp(1), px1-d.dp(8), py+d.cellH-d.dp(1)), d.dp(7), blend(card, d.color(th.fg, th.fg), 0.06), false)
		}
		right := px1 - d.dp(16)
		switch {
		case o.hint == "":
		case !looksLikeShortcut(o.hint):
			// A code snippet or detail: shown only if the label leaves room for it.
			room := right - px0 - d.dp(20) - d.uiWidth(gtx, o.label, 14, false) - d.dp(24)
			if room > d.dp(90) {
				text := d.fit(gtx, strings.TrimSpace(o.hint), room, 12, false)
				hw := d.uiWidth(gtx, text, 12, false)
				d.ui(gtx, right-hw, cy, text, muted, 12, false)
				right -= hw + d.dp(14)
			}
		default:
			left := d.hintChips(gtx, right, cy, o.hint, selected)
			if left > px0+(px1-px0)/2 {
				right = left
			} else {
				right = px0 + (px1-px0)/2
			}
		}
		label := d.fit(gtx, o.label, right-px0-d.dp(20), 14, false)
		d.highlighted(gtx, px0+d.dp(20), cy, label, matchRunes(label, m.input), fg, accent, 14)
	}
	if pb := v.previewBox; pb.h > 0 && pb.y == fr.y+1 && pb.x > list.x+list.w {
		// The rule between the list and the preview runs the full height of the card.
		edge := (pb.x-2)*d.cellW + d.cellW/2
		top := d.rowY(fr.y, d.rows) + d.cellH
		bottom := d.cardRect(fr).Max.Y - max(1, d.dp(1))
		d.fill(gtx, image.Rect(edge, top, edge+max(1, d.dp(1)), bottom), blend(card, d.color(th.fg, th.fg), 0.22))
	}
	if len(m.matches) > list.h && list.h > 0 {
		trackTop := d.rowY(list.y, d.rows) + d.dp(6)
		trackH := list.h*d.cellH - d.dp(12)
		thumb := max(d.dp(18), trackH*list.h/len(m.matches))
		off := (trackH - thumb) * list.start / max(1, len(m.matches)-list.h)
		x := px1 - d.dp(5)
		d.round(gtx, image.Rect(x, trackTop+off, x+d.dp(3), trackTop+off+thumb), d.dp(2), blend(card, d.color(th.fg, th.fg), 0.18), false)
	}
}

// paintCompletion draws the completion list the same way.
func (d *desktop) paintCompletion(gtx layout.Context, e *editor) {
	v := &d.m.view
	cb, c := v.completeBox, e.completion
	if e.menu != nil || cb.h == 0 || len(c.items) == 0 {
		return
	}
	framed := false
	for _, card := range v.cards {
		framed = framed || card.x == cb.x-1 && card.y == cb.y-1
	}
	if !framed {
		return
	}
	block := d.columnRect(cb.x, cb.x+cb.w, cb.y, cb.y+cb.h)
	d.fill(gtx, block, d.color(th.menu, th.bg))
	muted, fg := d.color(th.muted, th.fg), d.color(th.fg, th.fg)
	for i := 0; i < cb.h && cb.start+i < len(c.items); i++ {
		item := c.items[cb.start+i]
		py := d.rowY(cb.y+i, d.rows)
		cy := py + d.cellH/2
		if cb.start+i == c.selected {
			d.round(gtx, image.Rect(block.Min.X+d.dp(4), py+d.dp(1), block.Max.X-d.dp(4), py+d.cellH-d.dp(1)), d.dp(6), d.color(th.active, th.bg), false)
		}
		right := block.Max.X - d.dp(10)
		if item.Detail != "" {
			detail := d.fit(gtx, item.Detail, (block.Max.X-block.Min.X)/2, 12, false)
			dw := d.uiWidth(gtx, detail, 12, false)
			d.ui(gtx, right-dw, cy, detail, muted, 12, false)
			right -= dw + d.dp(12)
		}
		d.ui(gtx, block.Min.X+d.dp(10), cy, d.fit(gtx, item.Label, right-block.Min.X-d.dp(10), 13, false), fg, 13, false)
	}
}

// lineChar draws box-drawing and partial block characters as real shapes so
// borders are crisp one-pixel lines with rounded corners.
func (d *desktop) lineChar(gtx layout.Context, x, y int, s string, c color.NRGBA) bool {
	r, n := utf8.DecodeRuneInString(s)
	if n != len(s) {
		return false
	}
	w, h := float32(d.cellW), float32(d.cellH)
	fx, fy := float32(x), float32(y)
	cx, cy := fx+w/2, fy+h/2
	rad := w * 0.9
	switch r {
	case '│':
		d.fill(gtx, image.Rect(int(cx), y, int(cx)+1, y+d.cellH), c)
	case '─':
		d.fill(gtx, image.Rect(x, int(cy), x+d.cellW, int(cy)+1), c)
	case '╭':
		d.stroke(gtx, c, 1, func(p *clip.Path) {
			p.MoveTo(f32.Pt(cx, fy+h))
			p.LineTo(f32.Pt(cx, cy+rad))
			p.QuadTo(f32.Pt(cx, cy), f32.Pt(cx+rad, cy))
			p.LineTo(f32.Pt(fx+w, cy))
		})
	case '╮':
		d.stroke(gtx, c, 1, func(p *clip.Path) {
			p.MoveTo(f32.Pt(fx, cy))
			p.LineTo(f32.Pt(cx-rad, cy))
			p.QuadTo(f32.Pt(cx, cy), f32.Pt(cx, cy+rad))
			p.LineTo(f32.Pt(cx, fy+h))
		})
	case '╰':
		d.stroke(gtx, c, 1, func(p *clip.Path) {
			p.MoveTo(f32.Pt(cx, fy))
			p.LineTo(f32.Pt(cx, cy-rad))
			p.QuadTo(f32.Pt(cx, cy), f32.Pt(cx+rad, cy))
			p.LineTo(f32.Pt(fx+w, cy))
		})
	case '╯':
		d.stroke(gtx, c, 1, func(p *clip.Path) {
			p.MoveTo(f32.Pt(fx, cy))
			p.LineTo(f32.Pt(cx-rad, cy))
			p.QuadTo(f32.Pt(cx, cy), f32.Pt(cx, cy-rad))
			p.LineTo(f32.Pt(cx, fy))
		})
	case '▏', '▎':
		d.fill(gtx, image.Rect(x, y, x+max(1, d.cellW/8), y+d.cellH), c)
	case '▕':
		d.fill(gtx, image.Rect(x+d.cellW-1, y, x+d.cellW, y+d.cellH), c)
	default:
		return false
	}
	return true
}

func (d *desktop) paintSidebar(gtx layout.Context, e *editor) {
	v := &d.m.view
	if v.divider <= 0 {
		return
	}
	rows := d.rows
	px := v.divider * d.cellW
	d.fill(gtx, image.Rect(0, 0, px, d.height-d.bottom), d.color(th.panel, th.bg))
	d.fill(gtx, image.Rect(px, 0, px+1, d.height-d.bottom), d.color(th.border, th.border))
	muted := d.color(th.muted, th.fg)
	// Section header, like the Explorer title in VS Code.
	d.ui(gtx, d.dp(16), d.top/2, "EXPLORER", muted, 11, true)
	label := e.tree.status()
	if i := strings.Index(label, "·"); i >= 0 {
		label = strings.TrimSpace(label[i+len("·"):])
	}
	d.ui(gtx, px-d.dp(16)-d.uiWidth(gtx, label, 11, false), d.top/2, label, muted, 11, false)
	d.treeRows(gtx, e, 0, px, 1, rows-2, e.tree.scroll)
}

func (d *desktop) chevron(gtx layout.Context, cx, cy int, open bool, c color.NRGBA) {
	x, y, k := float32(cx), float32(cy), d.scale
	d.stroke(gtx, c, 1.4*k, func(p *clip.Path) {
		if open {
			p.MoveTo(f32.Pt(x-3*k, y-1.5*k))
			p.LineTo(f32.Pt(x, y+1.5*k))
			p.LineTo(f32.Pt(x+3*k, y-1.5*k))
		} else {
			p.MoveTo(f32.Pt(x-1.5*k, y-3*k))
			p.LineTo(f32.Pt(x+1.5*k, y))
			p.LineTo(f32.Pt(x-1.5*k, y+3*k))
		}
	})
}

func (d *desktop) paintTabs(gtx layout.Context, e *editor) {
	v := &d.m.view
	left := 0
	if v.divider > 0 {
		left = v.divider*d.cellW + 1
	}
	d.fill(gtx, image.Rect(left, 0, d.width, d.top), d.color(th.panel, th.bg))
	d.fill(gtx, image.Rect(left, d.top-1, d.width, d.top), d.color(th.border, th.border))
	for _, tb := range v.tabs {
		x0, x1 := tb.x*d.cellW+d.dp(2), (tb.x+tb.w)*d.cellW-d.dp(2)
		active := tb.b == e.b
		hover := d.mx >= x0 && d.mx < x1 && d.my < d.top
		top := d.dp(6)
		switch {
		case active:
			d.round(gtx, image.Rect(x0, top, x1, d.top), d.dp(8), d.color(th.bg, th.bg), true)
			d.fill(gtx, image.Rect(x0+d.dp(8), top, x1-d.dp(8), top+d.dp(2)), d.color(th.accent, th.accent))
		case hover:
			d.round(gtx, image.Rect(x0, top+d.dp(2), x1, d.top-1), d.dp(8), d.color(th.menu, th.bg), true)
		}
		cy := (top + d.top) / 2
		name := filepath.Base(tb.b.path)
		switch {
		case tb.b.dashboard:
			name = "Dashboard"
		case tb.b.path == "":
			name = "Untitled"
		}
		fg := d.color(th.muted, th.fg)
		if active {
			fg = d.color(th.fg, th.fg)
		}
		x := x0 + d.dp(12)
		d.fileIcon(gtx, x, cy, tb.b.path, false, fg)
		x += d.dp(22)
		closeX := tb.close*d.cellW + d.cellW/2
		d.ui(gtx, x, cy, d.fit(gtx, name, closeX-x-d.dp(10), 13, active), fg, 13, active)
		overClose := hover && d.mx >= closeX-d.dp(9) && d.mx < closeX+d.dp(9)
		switch {
		case tb.b.dirty() && !overClose:
			d.round(gtx, image.Rect(closeX-d.dp(3), cy-d.dp(3), closeX+d.dp(3), cy+d.dp(3)), d.dp(3), d.color(th.yellow, th.fg), false)
		case active || hover:
			if overClose {
				d.round(gtx, image.Rect(closeX-d.dp(9), cy-d.dp(9), closeX+d.dp(9), cy+d.dp(9)), d.dp(5), d.color(th.active, th.bg), false)
			}
			cx, cyf, a := float32(closeX), float32(cy), d.scale*3.5
			d.stroke(gtx, fg, 1.3*d.scale, func(p *clip.Path) {
				p.MoveTo(f32.Pt(cx-a, cyf-a))
				p.LineTo(f32.Pt(cx+a, cyf+a))
				p.MoveTo(f32.Pt(cx+a, cyf-a))
				p.LineTo(f32.Pt(cx-a, cyf+a))
			})
		}
	}
}

// fileIcon draws a small vector icon: a folder, or a page tinted by language.
func (d *desktop) fileIcon(gtx layout.Context, x, cy int, path string, dir bool, fallback color.NRGBA) {
	u := d.dp
	if dir {
		c := d.color(th.accent, th.fg)
		d.round(gtx, image.Rect(x, cy-u(5), x+u(7), cy-u(2)), u(2), c, true)
		d.round(gtx, image.Rect(x, cy-u(3), x+u(15), cy+u(6)), u(2), c, false)
		return
	}
	c := fallback
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		c = d.color(th.aqua, th.fg)
	case ".js", ".ts", ".jsx", ".tsx", ".json":
		c = d.color(th.yellow, th.fg)
	case ".md", ".txt":
		c = d.color(th.accent, th.fg)
	case ".py", ".rb", ".sh":
		c = d.color(th.purple, th.fg)
	case ".rs", ".c", ".h", ".cpp", ".java":
		c = d.color(th.orange, th.fg)
	case ".mod", ".sum", ".toml", ".yaml", ".yml":
		c = d.color(th.red, th.fg)
	}
	d.round(gtx, image.Rect(x+u(2), cy-u(7), x+u(13), cy+u(7)), u(2), c, false)
	line := d.color(th.panel, th.bg)
	for i := range 3 {
		y := cy - u(3) + i*u(3)
		d.fill(gtx, image.Rect(x+u(4), y, x+u(11), y+max(1, u(1))), line)
	}
}

func (d *desktop) paintStatus(gtx layout.Context, e *editor) {
	st := d.m.view.status
	if st.mode == "" {
		return
	}
	u := d.dp
	y0 := d.height - d.bottom
	cy := y0 + d.bottom/2
	d.fill(gtx, image.Rect(0, y0, d.width, d.height), d.color(th.panel, th.bg))
	d.fill(gtx, image.Rect(0, y0, d.width, y0+1), d.color(th.border, th.border))
	muted, fg := d.color(th.muted, th.fg), d.color(th.fg, th.fg)
	const sp = 12
	x := u(8)
	// Mode pill.
	w := d.uiWidth(gtx, st.mode, sp, true)
	d.round(gtx, image.Rect(x, cy-u(9), x+w+u(18), cy+u(9)), u(5), d.color(st.modeColor, th.accent), false)
	d.ui(gtx, x+u(9), cy, st.mode, d.color(th.badge, th.bg), sp, true)
	x += w + u(28)
	if !st.dashboard {
		x += d.ui(gtx, x, cy, st.profile, muted, sp, false) + u(16)
	}
	if st.keys != "" {
		k := st.keys + " …"
		kw := d.uiWidth(gtx, k, sp, true)
		d.round(gtx, image.Rect(x-u(6), cy-u(9), x+kw+u(6), cy+u(9)), u(5), d.color(th.active, th.bg), false)
		d.ui(gtx, x, cy, k, fg, sp, true)
		x += kw + u(18)
	}
	// Right side, laid out from the edge inwards.
	r := d.width - u(12)
	put := func(s string, c color.NRGBA, bold bool) {
		r -= d.uiWidth(gtx, s, sp, bold)
		d.ui(gtx, r, cy, s, c, sp, bold)
		r -= u(16)
	}
	if !st.dashboard {
		put(fmt.Sprintf("Ln %d, Col %d", st.line, st.column), fg, false)
		put(fmt.Sprintf("%d%%", st.percent), muted, false)
		put("UTF-8", muted, false)
		put(st.ending, muted, false)
		put(st.language, muted, false)
	}
	if st.branch != "" {
		put(st.branch, muted, false)
		// A small branch mark: two dots joined by a line.
		bx := r + u(16) - u(14)
		d.fill(gtx, image.Rect(bx+u(3), cy-u(5), bx+u(4), cy+u(5)), muted)
		d.round(gtx, image.Rect(bx, cy-u(6), bx+u(7), cy+u(1)), u(3), muted, false)
		d.round(gtx, image.Rect(bx+u(1), cy-u(5), bx+u(6), cy), u(2), d.color(th.panel, th.bg), false)
		r -= u(12)
	}
	if st.folder != "" {
		put(st.folder, fg, true)
	}
	if st.problems && r-x > u(120) {
		count := func(n int, c terminalColor, mark string) {
			col := muted
			if n > 0 {
				col = d.color(c, th.fg)
			}
			x += d.ui(gtx, x, cy, fmt.Sprintf("%s %d", mark, n), col, sp, n > 0) + u(10)
		}
		count(st.fileErrors+st.projectErrors, th.red, "\u2715")
		count(st.fileWarnings+st.projectWarnings, th.yellow, "\u25b3")
		x += u(6)
	}
	text := st.file
	if st.crumbs != "" {
		text += "  \u203a  " + st.crumbs
	}
	if st.message != "" {
		text = st.message
	}
	if st.dashboard {
		text = ""
	}
	d.ui(gtx, x, cy, d.fit(gtx, text, max(0, r-x), sp, false), fg, sp, false)
}

// dp converts density-independent pixels to device pixels.
func (d *desktop) dp(n int) int { return int(float32(n)*d.scale + 0.5) }
