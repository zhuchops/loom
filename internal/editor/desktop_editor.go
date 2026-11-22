//go:build desktop

package editor

import (
	"image"
	"image/color"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/io/pointer"
	"gioui.org/op"
)

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
		return " ", baseStyle, 1
	}
	c := s.cells[y*s.w+x]
	return c.s, c.style, c.w
}

// boxRect is where a box of cells is, stretched over the padding at the edges.
func (d *desktop) boxRect(b hitBox, region rect) rect {
	o := d.geo.grid.Min
	r := image.Rect(o.X+d.cellX(b.x), o.Y+b.y*d.cellH, o.X+d.cellX(b.x+b.w), o.Y+(b.y+b.h)*d.cellH)
	if b.x == 0 {
		r.Min.X = region.Min.X
	}
	if b.y == 0 {
		r.Min.Y = region.Min.Y
	}
	if b.x+b.w >= d.cols {
		r.Max.X = region.Max.X
	}
	if b.y+b.h >= d.rows {
		r.Max.Y = region.Max.Y
	}
	return r
}

func (d *desktop) paintEditor(e *editor, region rect, ground color.NRGBA) {
	v, f := &d.m.view, d.m.view.screen
	d.zone(zone{r: region, grid: true, cursor: pointer.CursorText})

	pane := d.activePane()
	d.updateScroll(pane)
	d.updateCursor(f)

	d.clipped(region, func() {
		d.fill(region, ground)
		all := hitBox{0, 0, f.Width(), f.Height(), 0}
		d.paintCells(f, all, 0, region, ground)

		sa := &d.scroll
		if sa.active && pane != nil {
			area := d.boxRect(pane.hitBox, region)
			d.clipped(area, func() {
				d.fill(area, ground)
				p := sa.progress(d.gtx.Now)
				d.paintCells(sa.old, pane.hitBox, -int(p*float64(d.cellH)), region, ground)
				d.paintCells(f, pane.hitBox, int((float64(sa.total)-p)*float64(d.cellH)), region, ground)
			})
			if d.gtx.Now.Sub(sa.start) >= scrollTime {
				sa.active = false
			}
			d.gtx.Execute(op.InvalidateCmd{})
		}
		if pane != nil && !sa.active {
			sa.last = takeSnapshot(f, pane.hitBox)
		}
		d.paintCursor(f, ground)

		for _, p := range v.panes {
			area := d.boxRect(p.hitBox, region)
			if p.b.dashboard {
				d.paintWelcome(e, p, area)
			} else {
				d.paintPaneScrollbar(e, p, area)
			}
		}
		for _, div := range v.paneDividers {
			if e.directory != nil || e.zen {
				break
			}
			d.paintDivider(div, region)
		}
	})
}

func (d *desktop) paintDivider(div paneDivider, region rect) {
	r := d.boxRect(div.box, region)
	cursor := pointer.CursorRowResize
	if div.vertical {
		cursor = pointer.CursorColResize
	}
	d.fill(r, d.c.bg)
	line, w := d.c.line, d.hairline()
	if d.hot(r) || d.m.view.paneDrag != nil && d.m.view.paneDrag.node == div.node {
		line, w = d.c.accent, d.u(2)
	}
	if div.vertical {
		x := (r.Min.X + r.Max.X) / 2
		d.fill(image.Rect(x-w/2, r.Min.Y, x-w/2+w, r.Max.Y), line)
	} else {
		y := (r.Min.Y + r.Max.Y) / 2
		d.fill(image.Rect(r.Min.X, y-w/2, r.Max.X, y-w/2+w), line)
	}
	d.zone(zone{r: r, grid: true, cursor: cursor})
}

func (d *desktop) paintPaneScrollbar(e *editor, p paneBox, area rect) {
	total := len(p.b.lines)
	if total <= p.h || !e.cfg.Scrollbar {
		return
	}
	track := image.Rect(area.Max.X-d.u(10), area.Min.Y, area.Max.X, area.Max.Y)
	d.scrollbar(track, p.b.top, p.h, total)
	move := func() {
		want := (d.my - track.Min.Y) * total / max(1, track.Dy())
		want = min(total-p.h, max(0, want-p.h/2))
		if delta := want - p.b.top; delta != 0 {
			doc := e
			if e.directory != nil {
				doc = e.directory.doc
			}
			d.m.view.paneWheel(doc, p, delta, 0)
		}
	}
	d.zone(zone{r: track, click: move, drag: move, wheel: func(dx, dy int) { d.m.view.paneWheel(e, p, dy, dx) }, step: d.cellH})
}

func (d *desktop) paintCells(src cellSource, box hitBox, dy int, region rect, ground color.NRGBA) {
	o := d.geo.grid.Min
	o.Y += dy
	x1, y1 := box.x+box.w, box.y+box.h
	background := func(c terminalColor) color.NRGBA {
		if c == th.bg {
			return ground
		}
		return d.cellBG(c)
	}
	for y := box.y; y < y1; y++ {
		py := o.Y + y*d.cellH
		for x := box.x; x < x1; {
			_, style, width := src.Get(x, y)
			end := x + max(1, width)
			for end < x1 {
				_, next, w := src.Get(end, y)
				if next.bg != style.bg {
					break
				}
				end += max(1, w)
			}
			if style.bg != th.bg {
				r := image.Rect(o.X+d.cellX(x), py, o.X+d.cellX(end), py+d.cellH)
				if x == 0 {
					r.Min.X = region.Min.X
				}
				if end >= d.cols {
					r.Max.X = region.Max.X
				}
				d.fill(r, background(style.bg))
			}
			x = end
		}
	}
	for y := box.y; y < y1; y++ {
		py := o.Y + y*d.cellH
		for x := box.x; x < x1; {
			s, style, width := src.Get(x, y)
			width = max(1, width)
			start := x
			if s == "" || s == " " {
				x += width
			} else {
				bg := background(style.bg)
				px := o.X + d.cellX(x)
				switch {
				case d.lineChar(px, py, s, style.fg, bg):
					x += width
				case !plainASCII(s):
					d.glyph(px, py, s, d.cellFG(style.fg, bg), style.bold, style.italic)
					x += width
				default:
					var run strings.Builder
					var last byte
					for x < x1 {
						t, next, _ := src.Get(x, y)
						if next.fg != style.fg || next.bg != style.bg || next.bold != style.bold || next.italic != style.italic || next.underline != style.underline || !plainASCII(t) {
							break
						}
						// Spaced-out letters are placed one by one.
						if run.Len() > 0 && (d.look.spacing != 0 || !d.look.ligatures && ligatureBreak(last, t[0])) {
							break
						}
						last = t[0]
						run.WriteString(t)
						x++
					}
					d.glyph(px, py, run.String(), d.cellFG(style.fg, bg), style.bold, style.italic)
				}
			}
			if style.underline != 0 {
				uy := py + d.cellH - d.u(2.5)
				d.fill(image.Rect(o.X+d.cellX(start), uy, o.X+d.cellX(x), uy+d.u(1.2)), nrgba(style.underline, d.c.red))
			}
		}
	}
}

func plainASCII(s string) bool { return len(s) == 1 && s[0] > 0x20 && s[0] < 0x7f }

func (d *desktop) glyph(x, y int, s string, c color.NRGBA, bold, italic bool) {
	st := d.code
	st.strong, st.italic = bold, italic
	x += d.px(d.look.spacing / 2)
	defer op.Offset(image.Pt(x, y+d.lift)).Push(d.gtx.Ops).Pop()
	d.lay(d.gtx, s, st, c)
}

// lineChar draws rules, fold markers and gutter marks as shapes.
func (d *desktop) lineChar(x, y int, s string, fg terminalColor, bg color.NRGBA) bool {
	r, n := utf8.DecodeRuneInString(s)
	if n != len(s) {
		return false
	}
	w, h, line := int(math.Ceil(d.cellW)), d.cellH, d.hairline()
	cx, cy := x+w/2, y+h/2
	c := nrgba(fg, d.c.text)
	switch r {
	case '│':
		d.fill(image.Rect(cx, y, cx+line, y+h), c)
	case '─':
		d.fill(image.Rect(x, cy, x+w, cy+line), c)
	case '╭':
		d.fill(image.Rect(cx, cy, cx+line, y+h), c)
		d.fill(image.Rect(cx, cy, x+w, cy+line), c)
	case '╮':
		d.fill(image.Rect(cx, cy, cx+line, y+h), c)
		d.fill(image.Rect(x, cy, cx+line, cy+line), c)
	case '╰':
		d.fill(image.Rect(cx, y, cx+line, cy+line), c)
		d.fill(image.Rect(cx, cy, x+w, cy+line), c)
	case '╯':
		d.fill(image.Rect(cx, y, cx+line, cy+line), c)
		d.fill(image.Rect(x, cy, cx+line, cy+line), c)
	case '▏': // indent guide
		d.fill(image.Rect(x, y, x+line, y+h), blend(bg, d.c.text, 0.13))
	case '▎': // git change
		d.round(image.Rect(x, y+d.u(1), x+d.u(2.5), y+h-d.u(1)), 1, c)
	case '▾':
		d.icon(iconDown, cx, cy, 10, d.c.faint)
	case '▸':
		d.icon(iconRight, cx, cy, 10, d.c.dim)
	case '●':
		k := d.u(2.8)
		d.round(image.Rect(cx-k, cy-k, cx+k, cy+k), 3, c)
	default:
		return false
	}
	return true
}

func (d *desktop) activePane() *paneBox {
	e, v := d.m.editor, &d.m.view
	if e.directory != nil {
		return nil
	}
	for i := range v.panes {
		p := &v.panes[i]
		if p.leaf == e.activePane && !p.b.dashboard {
			return p
		}
	}
	return nil
}

func (d *desktop) modal(e *editor) bool {
	return d.dialog != nil || d.settings != nil || e.menu != nil || e.notificationDetail != nil || e.directory != nil || e.panel != nil || e.diff != nil
}

// scrollAnim glides the text of the focused pane when its first line changes.
type scrollAnim struct {
	buf     *buffer
	box     hitBox
	lastTop int
	last    *frameCells
	old     *frameCells
	oldTop  int
	total   int
	p0      float64
	start   time.Time
	active  bool
}

const scrollTime = 110 * time.Millisecond

func (a *scrollAnim) progress(now time.Time) float64 {
	t := min(1, float64(now.Sub(a.start))/float64(scrollTime))
	ease := 1 - (1-t)*(1-t)*(1-t)
	return a.p0 + (float64(a.total)-a.p0)*ease
}

func (d *desktop) updateScroll(pane *paneBox) {
	sa, e := &d.scroll, d.m.editor
	if pane == nil || d.modal(e) {
		*sa = scrollAnim{}
		return
	}
	top := pane.b.top
	if sa.buf != pane.b || sa.box != pane.hitBox {
		*sa = scrollAnim{buf: pane.b, box: pane.hitBox, lastTop: top}
		return
	}
	if top == sa.lastTop {
		return
	}
	now := d.gtx.Now
	old, oldTop, p := sa.last, sa.lastTop, 0.0
	if sa.active {
		old, oldTop, p = sa.old, sa.oldTop, sa.progress(now)
	}
	total := top - oldTop
	cfg := &e.cfg.options
	if old != nil && animationsOn && cfg.Animations && cfg.SmoothScroll && total != 0 && total < pane.h && -total < pane.h {
		*sa = scrollAnim{buf: sa.buf, box: sa.box, lastTop: top, last: sa.last, old: old, oldTop: oldTop, total: total, p0: p, start: now, active: true}
	} else {
		sa.active = false
	}
	sa.lastTop = top
}

// cursorAnim slides the cursor between cells instead of jumping.
type cursorAnim struct {
	x, y, fx, fy, tx, ty float32
	start, moved         time.Time
	active, valid        bool
}

const cursorTime = 60 * time.Millisecond

func (d *desktop) updateCursor(f *terminalFrame) {
	c := &d.cur
	if !f.cursorVisible {
		c.valid, c.active = false, false
		return
	}
	o, dy, now := d.geo.grid.Min, 0, d.gtx.Now
	if sa := &d.scroll; sa.active {
		dy = int((float64(sa.total) - sa.progress(now)) * float64(d.cellH))
	}
	tx, ty := float32(o.X+d.cellX(f.cursorX)), float32(o.Y+f.cursorY*d.cellH+dy)
	far := absf(tx-c.tx) > float32(24*d.cellW) || absf(ty-c.ty) > float32(10*d.cellH)
	cfg := &d.m.editor.cfg.options
	glide := animationsOn && cfg.Animations && cfg.CursorGlide
	if tx != c.tx || ty != c.ty {
		c.moved = now // a moving cursor doesn't blink
	}
	if !c.valid || d.scroll.active || d.modal(d.m.editor) || far || !glide {
		*c = cursorAnim{x: tx, y: ty, tx: tx, ty: ty, valid: true, moved: c.moved}
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
			d.gtx.Execute(op.InvalidateCmd{})
		}
	}
}

const blinkTime = 530 * time.Millisecond

func (d *desktop) paintCursor(f *terminalFrame, ground color.NRGBA) {
	if !f.cursorVisible || !d.cur.valid || d.dialog != nil || d.settings != nil {
		return
	}
	cfg := &d.m.editor.cfg.options
	shape := cfg.CursorNormal
	if f.cursorShape == cursorBar {
		shape = cfg.CursorInsert
	}
	if cfg.CursorBlink && d.active {
		if d.cur.moved.IsZero() {
			d.cur.moved = d.gtx.Now
		}
		steps := d.gtx.Now.Sub(d.cur.moved) / blinkTime
		d.gtx.Execute(op.InvalidateCmd{At: d.cur.moved.Add((steps + 1) * blinkTime)})
		if steps%2 == 1 {
			return
		}
	}
	x, y := int(d.cur.x), int(d.cur.y)
	r := image.Rect(x, y, x+int(math.Round(d.cellW)), y+d.cellH)
	switch {
	case shape == "bar":
		r.Max.X = r.Min.X + max(1, d.px(2))
		d.fill(r, d.c.accent)
	case shape == "underline":
		r.Min.Y = r.Max.Y - max(1, d.px(2.5))
		d.fill(r, d.c.accent)
	case !d.active:
		d.border(r, 1, d.c.accent, d.u(1))
	case d.cur.active:
		d.fill(r, fade(d.c.accent, 170))
	default:
		d.round(r, 1.5, d.c.accent)
		if s, style, _ := f.Get(f.cursorX, f.cursorY); s != "" && s != " " && s != "▏" {
			d.glyph(x, y, s, readable(ground, d.c.accent, 4.5), style.bold, style.italic)
		}
	}
}
