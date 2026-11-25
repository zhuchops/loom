// Package brand draws the Loom mark: two warp and two weft threads woven into
// an L. The app icon sets it on a night sky, with a crescent moon resting in
// the crook of the L (Loom, lune). The app icon and the logo inside the app
// both come from here.
package brand

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"sync"
)

type vec struct{ x, y float64 }

type rgb struct{ r, g, b float64 }

func hex(v uint32) rgb {
	return rgb{float64(v >> 16 & 0xff), float64(v >> 8 & 0xff), float64(v & 0xff)}
}

func (a rgb) mix(b rgb, t float64) rgb {
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

func clamp(v float64) float64 { return min(max(v, 0), 1) }

// thread is a capsule between two points, coloured from its start to its end.
type thread struct {
	a, b     vec
	r        float64
	from, to rgb
}

func (t thread) nearest(p vec) (off vec, along float64) {
	dx, dy := t.b.x-t.a.x, t.b.y-t.a.y
	along = clamp(((p.x-t.a.x)*dx + (p.y-t.a.y)*dy) / (dx*dx + dy*dy))
	return vec{p.x - t.a.x - dx*along, p.y - t.a.y - dy*along}, along
}

type mark struct {
	size       float64
	warp, weft [2]thread
	flat       bool // too small for shadows and fine shading
}

// layout fits the mark into a square. The small logos inside the app land on
// whole pixels, so the threads and the gap between them stay sharp.
func layout(size int, tile bool) mark {
	s := float64(size)
	span, shift := s, 0.0
	width, gap, over := 0.216, 0.058, 0.076
	if tile {
		span, shift = s*612/1024, s*5/1024
	}
	m := mark{size: s, flat: !tile && size <= 64}
	if m.flat {
		width, gap = 0.235, 0.075
	}
	w, g, o := span*width, span*gap, span*over
	if !tile && size <= 40 {
		w, g, o = max(2, math.Round(w)), max(1, math.Round(g)), max(1, math.Round(o))
	}
	// The L is heavy at the bottom left, so it sits a little up and right.
	left, top := (s-span)/2+shift, (s-span)/2-shift
	r, right, bottom := w/2, left+span, top+span
	x0 := left + o + r
	y1 := bottom - o - r
	m.warp = [2]thread{
		{vec{x0, top + r}, vec{x0, bottom - r}, r, hex(0x86eaff), hex(0x2c63f5)},
		{vec{x0 + w + g, top + r}, vec{x0 + w + g, bottom - r}, r, hex(0x8af5c8), hex(0x1687e8)},
	}
	m.weft = [2]thread{
		{vec{left + r, y1 - w - g}, vec{right - r, y1 - w - g}, r, hex(0xff4f9d), hex(0xffc869)},
		{vec{left + r, y1}, vec{right - r, y1}, r, hex(0x9256ff), hex(0xff5c9c)},
	}
	return m
}

// paint returns the colour of the top thread at p.
func (m *mark) paint(p vec) (rgb, bool) {
	vi, hi := -1, -1
	var voff, hoff vec
	var vat, hat float64
	for i := range 2 {
		if off, at := m.warp[i].nearest(p); math.Hypot(off.x, off.y) <= m.warp[i].r {
			vi, voff, vat = i, off, at
		}
		if off, at := m.weft[i].nearest(p); math.Hypot(off.x, off.y) <= m.weft[i].r {
			hi, hoff, hat = i, off, at
		}
	}
	if vi < 0 && hi < 0 {
		return rgb{}, false
	}
	vertical := hi < 0 || vi >= 0 && (vi+hi)%2 == 1
	var c rgb
	if vertical {
		t := m.warp[vi]
		c = m.shade(t, voff, t.from.mix(t.to, vat))
	} else {
		t := m.weft[hi]
		c = m.shade(t, hoff, t.from.mix(t.to, hat))
	}

	// A thread darkens where it dips under the one crossing it.
	r := m.warp[0].r
	reach, depth := r*0.62, 0.5
	if m.flat {
		reach, depth = r*0.5, 0.3
	}
	for i := range 2 {
		var d float64
		if vertical {
			if (vi+i)%2 == 1 {
				continue
			}
			d = math.Abs(p.y-m.weft[i].a.y) - r
		} else {
			if (hi+i)%2 == 0 {
				continue
			}
			d = math.Abs(p.x-m.warp[i].a.x) - r
		}
		if d >= 0 && d < reach {
			f := 1 - d/reach
			c = c.mix(hex(0x14062e), depth*f*f)
		}
	}
	return c, true
}

// shade rounds a thread, lit from the upper left.
func (m *mark) shade(t thread, off vec, c rgb) rgb {
	x, y := off.x/t.r, off.y/t.r
	z := math.Sqrt(max(0, 1-x*x-y*y))
	light := -0.36*x - 0.48*y + 0.8*z
	strength, shine := 1.0, 0.24
	if m.flat {
		strength, shine = 0.5, 0.08
	}
	if k := (light - 0.72) * strength; k < 0 {
		c = c.mix(hex(0x1c0b45), min(-k*0.5, 0.6))
	} else {
		c = c.mix(hex(0xffffff), k*0.35)
	}
	return c.mix(hex(0xffffff), shine*math.Pow(max(0, light), 18))
}

// shadow is how much the mark darkens the tile at p.
func (m *mark) shadow(p vec) float64 {
	p.y -= m.size * 0.014
	blur, d := m.size*0.03, math.Inf(1)
	for _, set := range [2][2]thread{m.warp, m.weft} {
		for _, t := range set {
			off, _ := t.nearest(p)
			d = min(d, math.Hypot(off.x, off.y)-t.r)
		}
	}
	f := clamp(0.5 - d/(2*blur))
	return 0.55 * f * f * (3 - 2*f)
}

// tileEdge is the distance to the macOS icon shape: an 824 unit square on a
// 1024 unit canvas with continuous corners.
func tileEdge(p vec, size float64) float64 {
	u := size / 1024
	r := 237 * u
	qx, qy := math.Abs(p.x-size/2)-(412*u-r), math.Abs(p.y-size/2)-(412*u-r)
	if qx <= 0 || qy <= 0 {
		return max(qx, qy) - r
	}
	const n = 2.67
	return math.Pow(math.Pow(qx, n)+math.Pow(qy, n), 1/n) - r
}

// The moon, in 1024 unit icon space: a lit disk with a darker one over its
// upper right, leaving a crescent that opens away from the L and hugs its
// corner.
var (
	moonLit   = vec{676, 330}
	moonShade = vec{738, 276}
)

const (
	moonR      = 146.0
	moonShadeR = 134.0
)

// stars are small points of light on the sky: x, y and brightness.
var stars = [][3]float64{
	{886, 154, 0.9}, {520, 168, 0.6}, {872, 488, 0.55}, {478, 262, 0.4}, {610, 520, 0.35}, {900, 330, 0.3},
}

func tileColour(p vec, size, edge float64) rgb {
	u := size / 1024
	x, y := p.x/u, p.y/u
	t := clamp((y - 100) / 824)
	c := hex(0x2a2a58).mix(hex(0x0a0a1a), t)
	// Moonlight: a halo that fades into the sky.
	lit := math.Hypot(x-moonLit.x, y-moonLit.y)
	halo := clamp(1 - (lit-moonR*0.7)/300)
	c = c.mix(hex(0x8f86ff), 0.26*halo*halo)
	for _, s := range stars {
		d := math.Hypot(x-s[0], y-s[1])
		if g := clamp(1 - d/(7+4*s[2])); g > 0 {
			c = c.mix(hex(0xfff6dc), s[2]*g*g)
		}
	}
	if lit <= moonR {
		c = moon(x, y, lit, c)
	}
	if edge > -max(5*u, 0.6) {
		c = c.mix(hex(0xffffff), 0.05+0.19*(1-t))
	}
	return c
}

// moon paints the crescent over the sky colour c.
func moon(x, y, lit float64, sky rgb) rgb {
	shade := math.Hypot(x-moonShade.x, y-moonShade.y)
	// The terminator is soft; the limb is sharp.
	day := clamp((shade - moonShadeR) / 10)
	if day <= 0 {
		// The dark side keeps a trace of earthshine.
		return sky.mix(hex(0x3a3770), 0.35)
	}
	// Lit from the lower left: pale gold, warmer toward the limb.
	k := clamp(lit / moonR)
	c := hex(0xfff4d6).mix(hex(0xf2c879), k*k*0.85)
	// A few soft maria.
	for _, m := range [][3]float64{{596, 374, 24}, {640, 432, 17}, {572, 312, 13}} {
		if d := math.Hypot(x-m[0], y-m[1]); d < m[2] {
			f := 1 - d/m[2]
			c = c.mix(hex(0xe0b46e), 0.18*f)
		}
	}
	return sky.mix(c, day)
}

// Icon draws the app icon.
func Icon(size int) *image.RGBA { return render(size, true) }

// Mark draws the woven L alone, on a transparent background.
func Mark(size int) *image.RGBA { return render(size, false) }

func render(size int, tile bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	if size <= 0 {
		return img
	}
	m := layout(size, tile)
	n := 3
	if size < 256 {
		n = 6
	}
	workers := min(runtime.GOMAXPROCS(0), size)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			for y := w; y < size; y += workers {
				for x := range size {
					img.SetRGBA(x, y, m.pixel(x, y, n, tile))
				}
			}
		})
	}
	wg.Wait()
	return img
}

func (m *mark) pixel(x, y, n int, tile bool) color.RGBA {
	var sum rgb
	hit := 0
	for sy := range n {
		for sx := range n {
			p := vec{float64(x) + (float64(sx)+0.5)/float64(n), float64(y) + (float64(sy)+0.5)/float64(n)}
			edge := -1.0
			if tile {
				if edge = tileEdge(p, m.size); edge > 0 {
					continue
				}
			}
			c, on := m.paint(p)
			if !on {
				if !tile {
					continue
				}
				c = tileColour(p, m.size, edge)
				c = c.mix(hex(0x000000), m.shadow(p))
			}
			sum = rgb{sum.r + c.r, sum.g + c.g, sum.b + c.b}
			hit++
		}
	}
	// color.RGBA is premultiplied: samples outside the shape count as clear.
	all := float64(n * n)
	ch := func(v float64) uint8 { return uint8(min(255, v/all+0.5)) }
	return color.RGBA{ch(sum.r), ch(sum.g), ch(sum.b), ch(255 * float64(hit))}
}
