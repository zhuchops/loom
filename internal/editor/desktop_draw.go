//go:build desktop

package editor

import (
	"image"
	"image/color"
	"os"
	"strings"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

type rect = image.Rectangle

type textStyle struct {
	size   float32
	strong bool
	mono   bool
	italic bool
	// code size is the editor's own, not scaled with the interface.
	code bool
	// family previews an installed font.
	family font.Typeface
}

var (
	textBody    = textStyle{size: 13}
	textStrong  = textStyle{size: 13, strong: true}
	textSmall   = textStyle{size: 11.5}
	textCaption = textStyle{size: 10.5, strong: true}
	textTitle   = textStyle{size: 14.5, strong: true}
	textInput   = textStyle{size: 15.5}
	textHero    = textStyle{size: 30, strong: true}
	textCode    = textStyle{size: 12, mono: true}

	textPath     = textStyle{size: 14.5}
	textPathFile = textStyle{size: 14.5, strong: true}
)

type measured struct {
	style textStyle
	text  string
}

func readFaces(path string) []font.FontFace {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	faces, err := opentype.ParseCollection(withoutStrikes(raw))
	if err != nil {
		return nil
	}
	return faces
}

func (d *desktop) face(st textStyle) font.Font {
	if st.family != "" {
		return font.Font{Typeface: st.family}
	}
	if !st.mono {
		if st.strong {
			return d.uiStrong
		}
		return d.uiFont
	}
	f := d.codeFont
	if st.strong {
		f.Weight = font.Bold
	}
	if st.italic && d.hasItalic {
		f.Style = font.Italic
	}
	return f
}

// u converts density-independent pixels to device pixels.
func (d *desktop) u(n float32) int { return int(n*d.scale + 0.5) }

// px is u for what the interface size must not scale: code, and the system's
// own window buttons.
func (d *desktop) px(n float32) int { return int(n*d.gtx.Metric.PxPerDp + 0.5) }

func (d *desktop) hairline() int { return max(1, int(d.gtx.Metric.PxPerDp/2+0.5)) }

func (d *desktop) lay(gtx layout.Context, s string, st textStyle, c color.NRGBA) image.Point {
	gtx.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
	rec := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	ink := rec.Stop()
	size := st.size
	if !st.code {
		size *= d.look.uiPts / defaultUISize
	}
	return widget.Label{MaxLines: 1}.Layout(gtx, d.shaper, d.face(st), unit.Sp(size), s, ink).Size
}

func (d *desktop) measureText(s string, st textStyle) image.Point {
	if s == "" {
		return image.Point{}
	}
	key := measured{st, s}
	if size, ok := d.sizes[key]; ok {
		return size
	}
	if len(d.sizes) > 20000 {
		clear(d.sizes)
	}
	gtx := d.gtx
	gtx.Ops = &d.scratch
	d.scratch.Reset()
	size := d.lay(gtx, s, st, black)
	d.sizes[key] = size
	return size
}

func (d *desktop) width(s string, st textStyle) int { return d.measureText(s, st).X }

// text draws one line starting at x and centred on cy, and returns its width.
func (d *desktop) text(x, cy int, s string, st textStyle, c color.NRGBA) int {
	if s == "" {
		return 0
	}
	size := d.measureText(s, st)
	defer op.Offset(image.Pt(x, cy-size.Y/2)).Push(d.gtx.Ops).Pop()
	d.lay(d.gtx, s, st, c)
	return size.X
}

func (d *desktop) textRight(right, cy int, s string, st textStyle, c color.NRGBA) int {
	w := d.width(s, st)
	d.text(right-w, cy, s, st, c)
	return w
}

func (d *desktop) textCenter(cx, cy int, s string, st textStyle, c color.NRGBA) {
	d.text(cx-d.width(s, st)/2, cy, s, st, c)
}

// fit shortens s with an ellipsis so it is at most w pixels wide.
func (d *desktop) fit(s string, w int, st textStyle) string {
	if w <= 0 {
		return ""
	}
	if len(s) > 400 {
		s = string([]rune(s)[:min(400, utf8.RuneCountInString(s))])
	}
	if d.width(s, st) <= w {
		return s
	}
	r := []rune(s)
	lo, hi := 0, len(r)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if d.width(string(r[:mid])+"…", st) <= w {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		return ""
	}
	return strings.TrimRight(string(r[:lo]), " ") + "…"
}

// fitLeft keeps the end of s, for paths and for text being typed.
func (d *desktop) fitLeft(s string, w int, st textStyle) string {
	if d.width(s, st) <= w {
		return s
	}
	r := []rune(s)
	lo, hi := 0, len(r)
	for lo < hi {
		mid := (lo + hi) / 2
		if d.width("…"+string(r[mid:]), st) <= w {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return "…" + string(r[lo:])
}

// wrap breaks s into lines that are at most w pixels wide.
func (d *desktop) wrap(s string, w int, st textStyle) []string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), "\t", "    ")
	key := wrapped{st, s, w}
	if lines, ok := d.wraps[key]; ok {
		return lines
	}
	if len(d.wraps) > 500 {
		clear(d.wraps)
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if len(line) > 4000 {
			line = strings.ToValidUTF8(line[:4000], "") + "…"
		}
		for d.width(line, st) > w {
			r := []rune(line)
			lo, hi := 1, min(len(r), 600)
			for lo < hi {
				mid := (lo + hi + 1) / 2
				if d.width(string(r[:mid]), st) <= w {
					lo = mid
				} else {
					hi = mid - 1
				}
			}
			if lo >= len(r) {
				break
			}
			head := string(r[:lo])
			if at := strings.LastIndexByte(head, ' '); at > 0 && strings.TrimSpace(head[:at]) != "" {
				head = head[:at]
			}
			out = append(out, head)
			line = strings.TrimLeft(line[len(head):], " ")
		}
		out = append(out, line)
	}
	d.wraps[key] = out
	return out
}

type wrapped struct {
	style textStyle
	text  string
	width int
}

func (d *desktop) fill(r rect, c color.NRGBA) {
	if !r.Empty() {
		paint.FillShape(d.gtx.Ops, c, clip.Rect(r).Op())
	}
}

func (d *desktop) round(r rect, radius float32, c color.NRGBA) {
	if r.Empty() {
		return
	}
	n := min(d.u(radius), r.Dx()/2, r.Dy()/2)
	paint.FillShape(d.gtx.Ops, c, clip.UniformRRect(r, n).Op(d.gtx.Ops))
}

func (d *desktop) border(r rect, radius float32, c color.NRGBA, width int) {
	if r.Empty() {
		return
	}
	n := min(d.u(radius), r.Dx()/2, r.Dy()/2)
	shape := clip.UniformRRect(r, n)
	paint.FillShape(d.gtx.Ops, c, clip.Stroke{Path: shape.Path(d.gtx.Ops), Width: float32(width)}.Op())
}

// clipped draws inside r. What can't be seen there can't be clicked either.
func (d *desktop) clipped(r rect, draw func()) {
	defer clip.Rect(r).Push(d.gtx.Ops).Pop()
	outer := d.clip
	d.clip = r
	if !outer.Empty() {
		d.clip = r.Intersect(outer)
	}
	defer func() { d.clip = outer }()
	draw()
}

// card is the surface every floating element sits on.
func (d *desktop) card(r rect) {
	for i := range 7 {
		grow := d.u(float32(2 + 3*i))
		d.round(r.Inset(-grow).Add(image.Pt(0, d.u(7))), float32(12+3*i), fade(d.c.shadow, uint8(max(2, int(d.c.shadow.A)-2*i))))
	}
	d.round(r, 10, d.c.raised)
	d.border(r, 10, d.c.strongLine, d.hairline())
}

func (d *desktop) lineH(x0, x1, y int) {
	d.fill(image.Rect(x0, y, x1, y+d.hairline()), d.c.line)
}

func (d *desktop) lineV(x, y0, y1 int) {
	d.fill(image.Rect(x, y0, x+d.hairline(), y1), d.c.line)
}

func (d *desktop) caret(x, cy, h int) {
	d.fill(image.Rect(x, cy-h/2, x+d.u(1.5), cy+h/2), d.c.accent)
}

func (d *desktop) scrollbar(track rect, top, shown, total int) {
	if total <= shown || total == 0 || shown <= 0 {
		return
	}
	thumb := max(d.u(24), track.Dy()*shown/total)
	y := track.Min.Y + (track.Dy()-thumb)*min(top, total-shown)/(total-shown)
	d.round(image.Rect(track.Max.X-d.u(7), y+d.u(2), track.Max.X-d.u(3), y+thumb-d.u(2)), 2, d.c.thumb)
}

// pen draws in a 16 by 16 box, the space icons are designed in.
type pen struct {
	d    *desktop
	x, y float32
	k    float32
	c    color.NRGBA
	w    float32
}

func (d *desktop) pen(cx, cy int, size float32, c color.NRGBA) pen {
	k := size * d.scale / 16
	return pen{d: d, x: float32(cx) - 8*k, y: float32(cy) - 8*k, k: k, c: c, w: 1.35}
}

func (p pen) at(x, y float32) f32.Point { return f32.Pt(p.x+x*p.k, p.y+y*p.k) }

// line joins the x, y pairs in pts.
func (p pen) line(pts ...float32) {
	var path clip.Path
	path.Begin(p.d.gtx.Ops)
	path.MoveTo(p.at(pts[0], pts[1]))
	for i := 2; i+1 < len(pts); i += 2 {
		path.LineTo(p.at(pts[i], pts[i+1]))
	}
	paint.FillShape(p.d.gtx.Ops, p.c, clip.Stroke{Path: path.End(), Width: p.w * p.k}.Op())
}

func (p pen) area(pts ...float32) {
	var path clip.Path
	path.Begin(p.d.gtx.Ops)
	path.MoveTo(p.at(pts[0], pts[1]))
	for i := 2; i+1 < len(pts); i += 2 {
		path.LineTo(p.at(pts[i], pts[i+1]))
	}
	path.Close()
	paint.FillShape(p.d.gtx.Ops, p.c, clip.Outline{Path: path.End()}.Op())
}

func (p pen) circle(x, y, r float32) {
	const k = 0.5523
	var path clip.Path
	path.Begin(p.d.gtx.Ops)
	path.MoveTo(p.at(x+r, y))
	path.CubeTo(p.at(x+r, y+k*r), p.at(x+k*r, y+r), p.at(x, y+r))
	path.CubeTo(p.at(x-k*r, y+r), p.at(x-r, y+k*r), p.at(x-r, y))
	path.CubeTo(p.at(x-r, y-k*r), p.at(x-k*r, y-r), p.at(x, y-r))
	path.CubeTo(p.at(x+k*r, y-r), p.at(x+r, y-k*r), p.at(x+r, y))
	paint.FillShape(p.d.gtx.Ops, p.c, clip.Stroke{Path: path.End(), Width: p.w * p.k}.Op())
}

// Key symbols are drawn, because few fonts have all of them at one size.
func keySymbol(r rune) bool { return strings.ContainsRune("⌘⇧↵↑↓←→", r) }

func (d *desktop) keyTextWidth(s string, st textStyle) int {
	if !strings.ContainsAny(s, "⌘⇧↵↑↓←→") {
		return d.width(s, st)
	}
	w := 0
	for _, r := range s {
		if keySymbol(r) {
			w += d.u(11.5)
		} else {
			w += d.width(string(r), st)
		}
	}
	return w
}

func (d *desktop) keyText(x, cy int, s string, st textStyle, c color.NRGBA) {
	if !strings.ContainsAny(s, "⌘⇧↵↑↓←→") {
		d.text(x, cy, s, st, c)
		return
	}
	for _, r := range s {
		if !keySymbol(r) {
			x += d.text(x, cy, string(r), st, c)
			continue
		}
		p := d.pen(x+d.u(5.5), cy, 11.5, c)
		switch r {
		case '⌘':
			p.line(5.5, 5.5, 10.5, 5.5, 10.5, 10.5, 5.5, 10.5, 5.5, 5.5, 6.5, 5.5)
			p.circle(3.9, 3.9, 1.65)
			p.circle(12.1, 3.9, 1.65)
			p.circle(3.9, 12.1, 1.65)
			p.circle(12.1, 12.1, 1.65)
		case '⇧':
			p.line(8, 2.5, 13.5, 8, 10.5, 8, 10.5, 13, 5.5, 13, 5.5, 8, 2.5, 8, 8, 2.5, 8.6, 3.1)
		case '↵':
			p.line(13, 3.5, 13, 9.5, 3, 9.5)
			p.line(6, 6.5, 3, 9.5, 6, 12.5)
		case '↑':
			p.line(8, 13, 8, 3)
			p.line(4.5, 6.5, 8, 3, 11.5, 6.5)
		case '↓':
			p.line(8, 3, 8, 13)
			p.line(4.5, 9.5, 8, 13, 11.5, 9.5)
		case '←':
			p.line(13, 8, 3, 8)
			p.line(6.5, 4.5, 3, 8, 6.5, 11.5)
		case '→':
			p.line(3, 8, 13, 8)
			p.line(9.5, 4.5, 13, 8, 9.5, 11.5)
		}
		x += d.u(11.5)
	}
}
