//go:build desktop

package editor

import (
	"image"
	"image/color"
	"path/filepath"
	"strings"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// paintTitles redraws every popup title in the interface font.
func (d *desktop) paintTitles(gtx layout.Context) {
	card := d.color(th.menu, th.bg)
	for _, t := range d.m.view.titles {
		if t.text == "" || t.box.w < 6 {
			continue
		}
		r := d.columnRect(t.box.x+1, t.box.x+t.box.w-1, t.box.y, t.box.y+1)
		d.fill(gtx, r, card)
		d.ui(gtx, r.Min.X+d.dp(14), d.rowY(t.box.y, d.rows)+d.cellH/2+d.dp(2),
			d.fit(gtx, strings.ToUpper(t.text), r.Dx()-d.dp(28), 11, true), d.color(th.muted, th.fg), 11, true)
	}
}

// paintProse redraws the text lines of a card in the interface font. It is
// for popups that hold sentences (hover documentation, notifications), not code.
func (d *desktop) paintProse(gtx layout.Context, src cellSource, fr hitBox) {
	card := d.color(th.menu, th.bg)
	muted := d.color(th.muted, th.fg)
	for y := fr.y + 1; y < fr.y+fr.h-1 && y < d.rows-1; y++ {
		var line strings.Builder
		var first cellStyle
		lead, seen := 0, false
		for x := fr.x + 1; x < fr.x+fr.w-1; x++ {
			s, st, w := src.Get(x, y)
			if s == "" {
				if w == 0 {
					continue
				}
				s = " "
			}
			if !seen {
				if s == " " {
					lead++
					continue
				}
				first, seen = st, true
			}
			line.WriteString(s)
		}
		row := d.columnRect(fr.x+1, fr.x+fr.w-1, y, y+1)
		d.fill(gtx, row, card)
		text := strings.TrimRight(line.String(), " ")
		if text == "" {
			continue
		}
		size, bold, col := float32(13), first.bold, d.color(first.fg, th.fg)
		if first.fg == th.muted {
			size, col = 12, muted
		}
		d.ui(gtx, row.Min.X+d.dp(14)+lead*d.cellW, row.Min.Y+d.cellH/2, d.fit(gtx, text, row.Dx()-d.dp(28)-lead*d.cellW, size, bold), col, size, bold)
	}
}

func (d *desktop) paintWhichKey(gtx layout.Context, e *editor) {
	v := &d.m.view
	if len(v.whichItems) == 0 {
		return
	}
	first := v.whichItems[0]
	var fr hitBox
	for _, c := range v.cards {
		if first.x > c.x && first.x < c.x+c.w && first.y > c.y && first.y < c.y+c.h-1 {
			fr = c
		}
	}
	if fr.w == 0 {
		return
	}
	card := d.color(th.menu, th.bg)
	d.fill(gtx, d.columnRect(fr.x+1, fr.x+fr.w-1, fr.y+1, fr.y+fr.h-1), card)
	bg := blend(card, d.color(th.fg, th.fg), 0.10)
	for _, it := range v.whichItems {
		py := d.rowY(it.y, d.rows)
		cy := py + d.cellH/2
		x := it.x * d.cellW
		w := d.chip(gtx, x, cy, it.key, d.color(th.accent, th.fg), bg)
		d.ui(gtx, x+max(w, it.keyWidth*d.cellW)+d.dp(10), cy, it.title, d.color(th.fg, th.fg), 13, false)
	}
}

func (d *desktop) paintFindBar(gtx layout.Context, e *editor) {
	f := d.m.view.find
	if !f.on {
		return
	}
	card := d.color(th.menu, th.bg)
	accent, fg, muted := d.color(th.accent, th.fg), d.color(th.fg, th.fg), d.color(th.muted, th.fg)
	box := f.box
	d.fill(gtx, d.columnRect(box.x+1, box.x+box.w-1, box.y+1, box.y+box.h-1), card)
	// Chevron that shows or hides the replace row.
	cx := f.chevron.x*d.cellW + d.cellW
	cy := d.rowY(f.chevron.y, d.rows) + d.cellH/2
	d.chevron(gtx, cx, cy, f.replace, muted)
	for _, fld := range f.fields {
		py := d.rowY(fld.y, d.rows)
		r := image.Rect(fld.x*d.cellW+d.dp(2), py+d.dp(4), (fld.x+fld.w)*d.cellW-d.dp(2), py+d.cellH-d.dp(4))
		d.round(gtx, r, d.dp(6), d.color(th.bg, th.bg), false)
		if fld.active {
			rr := clip.UniformRRect(r, d.dp(6))
			paintStroke(gtx, rr, accent, float32(max(1, d.dp(1))))
		}
		text := fld.text
		tx := r.Min.X + d.dp(10)
		mid := py + d.cellH/2
		if text == "" {
			d.ui(gtx, tx, mid, fld.placeholder, muted, 13, false)
		} else {
			w := d.ui(gtx, tx, mid, d.fit(gtx, tailString(text, 64), r.Dx()-d.dp(24), 13, false), fg, 13, false)
			tx += w
		}
		if fld.active {
			if text == "" {
				tx = r.Min.X + d.dp(10)
			}
			d.fill(gtx, image.Rect(tx, mid-d.dp(8), tx+max(1, d.dp(1)), mid+d.dp(8)), accent)
		}
	}
	for _, t := range f.toggles {
		py := d.rowY(t.y, d.rows)
		r := image.Rect(t.x*d.cellW+d.dp(2), py+d.dp(5), (t.x+4)*d.cellW-d.dp(2), py+d.cellH-d.dp(5))
		col := muted
		if t.on {
			d.round(gtx, r, d.dp(5), blend(card, accent, 0.25), false)
			col = accent
		}
		label := strings.TrimSpace(t.label)
		w := d.uiWidth(gtx, label, 11, true)
		d.ui(gtx, r.Min.X+(r.Dx()-w)/2, py+d.cellH/2, label, col, 11, true)
	}
	if f.count != "" {
		col := muted
		if f.countTone == "red" {
			col = d.color(th.red, th.fg)
		}
		d.ui(gtx, f.countX*d.cellW, d.rowY(box.y+1, d.rows)+d.cellH/2, f.count, col, 12, false)
	}
	for _, a := range f.arrows {
		x := float32(a.x*d.cellW + d.cellW*3/2)
		y := float32(d.rowY(a.y, d.rows) + d.cellH/2)
		k := d.scale
		d.stroke(gtx, muted, 1.4*k, func(p *clip.Path) {
			switch a.label {
			case "↑":
				p.MoveTo(f32.Pt(x-3.5*k, y+1.5*k))
				p.LineTo(f32.Pt(x, y-2*k))
				p.LineTo(f32.Pt(x+3.5*k, y+1.5*k))
			case "↓":
				p.MoveTo(f32.Pt(x-3.5*k, y-1.5*k))
				p.LineTo(f32.Pt(x, y+2*k))
				p.LineTo(f32.Pt(x+3.5*k, y-1.5*k))
			default:
				p.MoveTo(f32.Pt(x-3.5*k, y-3.5*k))
				p.LineTo(f32.Pt(x+3.5*k, y+3.5*k))
				p.MoveTo(f32.Pt(x+3.5*k, y-3.5*k))
				p.LineTo(f32.Pt(x-3.5*k, y+3.5*k))
			}
		})
	}
	for _, b := range f.buttons {
		label := strings.TrimSpace(b.label)
		py := d.rowY(b.y, d.rows)
		w := len(b.label) * d.cellW
		r := image.Rect(b.x*d.cellW+d.dp(2), py+d.dp(5), b.x*d.cellW+w-d.dp(2), py+d.cellH-d.dp(5))
		d.round(gtx, r, d.dp(5), blend(card, accent, 0.20), false)
		tw := d.uiWidth(gtx, label, 12, true)
		d.ui(gtx, r.Min.X+(r.Dx()-tw)/2, py+d.cellH/2, label, accent, 12, true)
	}
}

func tailString(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[len(r)-n:])
	}
	return s
}

func paintStroke(gtx layout.Context, rr clip.RRect, c color.NRGBA, width float32) {
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rr.Path(gtx.Ops), Width: width}.Op())
}

// weaveLogo draws the Loom mark: warp and weft threads over and under.
func (d *desktop) weaveLogo(gtx layout.Context, cx, top, size int) {
	const n = 5
	pitch := size / n
	width := pitch * 62 / 100
	warp := []color.NRGBA{{0x61, 0xaf, 0xef, 255}, {0x56, 0xb6, 0xc2, 255}}
	weft := []color.NRGBA{{0xc8, 0x82, 0xe7, 255}, {0xe0, 0x6c, 0x75, 255}}
	x0 := cx - size/2
	pick := func(pair []color.NRGBA, i int) color.NRGBA { return blend(pair[0], pair[1], float32(i)/float32(n-1)) }
	bar := func(r image.Rectangle, c color.NRGBA, rounded bool) {
		if rounded {
			d.round(gtx, r, min(r.Dx(), r.Dy())/2, c, false)
		} else {
			d.fill(gtx, r, c)
		}
	}
	for j := 0; j < n; j++ {
		cy := top + pitch*j + pitch/2
		bar(image.Rect(x0, cy-width/2, x0+size, cy+width/2), pick(weft, j), true)
	}
	for i := 0; i < n; i++ {
		cxi := x0 + pitch*i + pitch/2
		bar(image.Rect(cxi-width/2, top, cxi+width/2, top+size), pick(warp, i), true)
		for j := 0; j < n; j++ {
			if (i+j)%2 == 0 {
				continue
			}
			cy := top + pitch*j + pitch/2
			bar(image.Rect(cxi-width/2, cy-width/2, cxi+width/2, cy+width/2), pick(weft, j), false)
		}
	}
}

// actionIcon draws a small vector icon for a dashboard command.
func (d *desktop) actionIcon(gtx layout.Context, command string, x, cy int, c color.NRGBA) {
	u := d.dp
	switch command {
	case "files", "project-search":
		d.magnifier(gtx, x, cy, c)
	case "buffers":
		d.round(gtx, image.Rect(x+u(3), cy-u(7), x+u(14), cy+u(2)), u(2), blend(c, d.color(th.bg, th.bg), 0.5), false)
		d.round(gtx, image.Rect(x, cy-u(4), x+u(11), cy+u(5)), u(2), c, false)
	case "open-project", "project":
		d.round(gtx, image.Rect(x, cy-u(7), x+u(7), cy-u(2)), u(2), c, false)
		d.round(gtx, image.Rect(x, cy-u(4), x+u(15), cy+u(6)), u(2), c, false)
	default:
		d.round(gtx, image.Rect(x, cy-u(7), x+u(15), cy+u(7)), u(3), c, false)
		d.stroke(gtx, d.color(th.bg, th.bg), 1.5*d.scale, func(p *clip.Path) {
			fx, fy, k := float32(x), float32(cy), d.scale
			p.MoveTo(f32.Pt(fx+4*k, fy-2.5*k))
			p.LineTo(f32.Pt(fx+7*k, fy))
			p.LineTo(f32.Pt(fx+4*k, fy+2.5*k))
			p.MoveTo(f32.Pt(fx+8.5*k, fy+3*k))
			p.LineTo(f32.Pt(fx+11.5*k, fy+3*k))
		})
	}
}

// paintDashboard replaces the block-letter start screen with a logo, a
// welcome block and a list of actions and recent files.
func (d *desktop) paintDashboard(gtx layout.Context, e *editor) {
	v := &d.m.view
	ds := v.dash
	if ds.box.w == 0 || len(ds.items) == 0 {
		return
	}
	bg := d.color(th.bg, th.bg)
	d.fill(gtx, d.columnRect(ds.box.x, ds.box.x+ds.box.w, ds.box.y, ds.box.y+ds.box.h), bg)
	muted, fg, accent := d.color(th.muted, th.fg), d.color(th.fg, th.fg), d.color(th.accent, th.fg)
	list := ds.list
	cx := (list.x*d.cellW + (list.x+list.w)*d.cellW) / 2
	top := d.rowY(max(ds.box.y, list.y-ds.header), d.rows)
	y := top + d.dp(10)
	if ds.logo {
		size := d.dp(52)
		d.weaveLogo(gtx, cx, y, size)
		y += size + d.dp(14)
	}
	title := "Loom"
	tw := d.uiWidth(gtx, title, 28, true)
	d.ui(gtx, cx-tw/2, y+d.dp(16), title, fg, 28, true)
	y += d.dp(38)
	sub := "A fast modal code editor · v" + version
	sw := d.uiWidth(gtx, sub, 13, false)
	d.ui(gtx, cx-sw/2, y+d.dp(8), sub, muted, 13, false)
	y += d.dp(22)
	if ds.logo {
		label := ds.project
		pw := d.uiWidth(gtx, label, 12, true) + d.dp(34)
		if ds.branch != "" {
			pw += d.uiWidth(gtx, ds.branch, 12, false) + d.dp(18)
		}
		x := cx - pw/2
		d.fileIcon(gtx, x, y+d.dp(9), ".", true, accent)
		x += d.dp(22)
		x += d.ui(gtx, x, y+d.dp(9), label, fg, 12, true) + d.dp(12)
		if ds.branch != "" {
			d.chip(gtx, x, y+d.dp(9), ds.branch, accent, blend(bg, accent, 0.14))
		}
	}
	for i := 0; i < list.h && list.start+i < len(ds.items); i++ {
		item := ds.items[list.start+i]
		py := d.rowY(list.y+i, d.rows)
		cy := py + d.cellH/2
		selected := ds.focused && list.start+i == ds.selected
		hover := d.mx >= list.x*d.cellW && d.mx < (list.x+list.w)*d.cellW && d.my >= py && d.my < py+d.cellH
		r := image.Rect(list.x*d.cellW, py+d.dp(2), (list.x+list.w)*d.cellW, py+d.cellH-d.dp(2))
		switch {
		case selected:
			d.round(gtx, r, d.dp(8), blend(bg, accent, 0.18), false)
			d.fill(gtx, image.Rect(r.Min.X+d.dp(2), r.Min.Y+d.dp(8), r.Min.X+d.dp(2)+max(2, d.dp(2)), r.Max.Y-d.dp(8)), accent)
		case hover:
			d.round(gtx, r, d.dp(8), blend(bg, d.color(th.fg, th.fg), 0.06), false)
		case item.path == "":
			d.round(gtx, r, d.dp(8), blend(bg, d.color(th.fg, th.fg), 0.035), false)
		}
		if item.path != "" && i > 0 && ds.items[list.start+i-1].path == "" {
			d.fill(gtx, image.Rect(r.Min.X+d.dp(8), py-d.dp(1), r.Max.X-d.dp(8), py), d.color(th.border, th.border))
		}
		ix := r.Min.X + d.dp(16)
		right := r.Max.X - d.dp(14)
		if item.path == "" {
			d.actionIcon(gtx, item.command, ix, cy, accent)
			_, label, _ := strings.Cut(item.label, "   ")
			if label == "" {
				label = item.label
			}
			d.ui(gtx, ix+d.dp(30), cy, label, fg, 14, false)
			if item.hint != "" {
				d.hintChips(gtx, right, cy, item.hint, selected)
			}
			continue
		}
		d.fileIcon(gtx, ix, cy, item.path, false, muted)
		_, rel, _ := strings.Cut(item.label, "   ")
		if rel == "" {
			rel = item.label
		}
		name := filepath.Base(rel)
		nx := ix + d.dp(26)
		nx += d.ui(gtx, nx, cy, name, fg, 13, false) + d.dp(8)
		if dir := filepath.Dir(rel); dir != "." {
			d.ui(gtx, nx, cy, d.fit(gtx, dir, right-nx-d.dp(60), 12, false), muted, 12, false)
		}
		d.ui(gtx, right-d.uiWidth(gtx, "recent", 11, false), cy, "recent", muted, 11, false)
	}
}
