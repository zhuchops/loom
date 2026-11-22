//go:build desktop

package editor

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"strings"
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

func (d *desktop) paint(gtx layout.Context, size image.Point) {
	f := d.m.view.screen
	e := d.m.editor
	rows := d.rows
	d.fill(gtx, image.Rectangle{Max: size}, d.color(th.bg, th.bg))
	for y := 0; y < f.Height() && y < rows; y++ {
		py, ph := d.rowY(y, rows), d.rowH(y, rows)
		for x := 0; x < f.Width(); {
			_, style, width := f.Get(x, y)
			width = max(1, width)
			end := x + width
			for end < f.Width() {
				_, next, w := f.Get(end, y)
				if next.bg != style.bg {
					break
				}
				end += max(1, w)
			}
			if style.bg != th.bg {
				d.fill(gtx, image.Rect(x*d.cellW, py, end*d.cellW, py+ph), d.color(style.bg, th.bg))
			}
			x = end
		}
	}
	cursorX, cursorY := -1, -1
	if f.cursorVisible {
		cursorX, cursorY = f.cursorX, f.cursorY
		py := d.rowY(cursorY, rows) + (d.rowH(cursorY, rows)-d.cellH)/2
		r := image.Rect(cursorX*d.cellW, py, (cursorX+1)*d.cellW, py+d.cellH)
		if f.cursorShape == cursorBar {
			r.Max.X = r.Min.X + max(2, d.cellW/6)
		}
		d.fill(gtx, r, d.color(th.fg, th.fg))
	}
	for y := 0; y < f.Height() && y < rows; y++ {
		py := d.rowY(y, rows) + (d.rowH(y, rows)-d.cellH)/2
		for x := 0; x < f.Width(); {
			s, style, width := f.Get(x, y)
			width = max(1, width)
			if s == "" || s == " " {
				x += width
				continue
			}
			fg := style.fg
			if x == cursorX && y == cursorY && f.cursorShape != cursorBar {
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
			for x < f.Width() {
				t, st, w := f.Get(x, y)
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
	// A hairline above the status bar.
	d.fill(gtx, image.Rect(0, d.height-d.bottom, size.X, d.height-d.bottom+1), d.color(th.border, th.border))
	d.paintStatus(gtx, e)
	if d.modal(e) {
		return
	}
	if !e.zen {
		d.paintSidebar(gtx, e)
		d.paintTabs(gtx, e)
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
	t := &e.tree
	for i := t.scroll; i < len(t.rows); i++ {
		y := 1 + i - t.scroll
		if y > rows-2 {
			break
		}
		row := t.rows[i]
		py := d.rowY(y, rows)
		cy := py + d.cellH/2
		hover := d.mx < px && d.my >= py && d.my < py+d.cellH
		switch {
		case i == t.selected && t.focused:
			d.round(gtx, image.Rect(d.dp(6), py+1, px-d.dp(6), py+d.cellH-1), d.dp(6), d.color(th.active, th.bg), false)
		case hover:
			d.round(gtx, image.Rect(d.dp(6), py+1, px-d.dp(6), py+d.cellH-1), d.dp(6), d.color(th.line, th.bg), false)
		}
		x := d.dp(14) + row.depth*d.dp(14)
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
		d.ui(gtx, x, cy, d.fit(gtx, filepath.Base(row.path), px-x-d.dp(26), 13, false), fg, 13, false)
		if mark := e.git.files[row.path]; mark != 0 {
			d.ui(gtx, px-d.dp(20), cy, string(mark), d.color(gitColor(mark), th.fg), 11, true)
		}
	}
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
