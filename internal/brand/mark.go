// Package brand draws the Loom mark: a crescent moon (Loom, lune) with one
// thread woven through its horns, over the upper one and under the lower.
// The app icon sets it on a night sky; the logo inside the app is the mark
// alone. Both come from here.
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

// The design, in 1024 units of the mark's square. The lit disk minus the
// shade disk leaves a crescent that opens to the upper right; its horns are
// at about (435, 227) and (781, 626).
var (
	litCentre   = vec{500, 520}
	shadeCentre = vec{650, 390}
	threadFrom  = vec{318, 160}
	threadTo    = vec{830, 751}
)

const (
	litR    = 300.0
	shadeR  = 270.0
	threadR = 15.0
	// The thread runs under the moon past this point along it.
	threadUnder = 0.5
)

// stars are points of light on the sky: x, y and brightness, in icon units.
var stars = [][3]float64{
	{820, 196, 1}, {226, 300, 0.7}, {900, 420, 0.55}, {300, 860, 0.5}, {640, 180, 0.4}, {170, 560, 0.45}, {860, 860, 0.35},
}

type mark struct {
	size, unit float64 // pixels, and pixels per design unit
	origin     vec     // where the design's square starts
	flat       bool    // too small for shading, maria and shadows
}

func layout(size int, tile bool) mark {
	s := float64(size)
	m := mark{size: s, flat: size <= 40}
	if tile {
		// The moon fills most of the 824 unit tile, a little up and left of
		// centre so the thread's ends have room.
		span := s * 760 / 1024
		m.unit = span / 1024
		// The crescent is heavy on its left; nudge it right and up.
		m.origin = vec{(s-span)/2 + 30*m.unit, (s-span)/2 - 40*m.unit}
	} else {
		m.unit = s / 1024
	}
	return m
}

// design maps a pixel position to design units.
func (m *mark) design(p vec) vec {
	return vec{(p.x - m.origin.x) / m.unit, (p.y - m.origin.y) / m.unit}
}

// moonAt reports whether d is on the crescent, and its colour there.
func (m *mark) moonAt(d vec) (rgb, bool) {
	lit := math.Hypot(d.x-litCentre.x, d.y-litCentre.y)
	shade := math.Hypot(d.x-shadeCentre.x, d.y-shadeCentre.y)
	if lit > litR || shade < shadeR {
		return rgb{}, false
	}
	// Pale gold at the heart of the crescent, warmer toward the limb.
	k := clamp(lit / litR)
	c := hex(0xfff6dc).mix(hex(0xf5c46a), k*k*0.9)
	if m.flat {
		return c, true
	}
	// Lit from the lower left: the inner edge, toward the shade, dims.
	edge := clamp((shade - shadeR) / 70)
	c = c.mix(hex(0xd99a4c), 0.35*(1-edge)*(1-edge))
	for _, sea := range [][3]float64{{330, 560, 46}, {400, 690, 34}, {300, 440, 26}, {470, 780, 22}} {
		if r := math.Hypot(d.x-sea[0], d.y-sea[1]); r < sea[2] {
			f := 1 - r/sea[2]
			c = c.mix(hex(0xe2ad62), 0.3*min(1, f*1.8))
		}
	}
	// A rim of light along the limb.
	if rim := litR - lit; rim < 10 {
		c = c.mix(hex(0xffffff), 0.25*(1-rim/10))
	}
	return c, true
}

// threadAt reports whether d is on the thread, how far along it, and its
// colour there.
func (m *mark) threadAt(d vec) (rgb, float64, bool) {
	dx, dy := threadTo.x-threadFrom.x, threadTo.y-threadFrom.y
	along := clamp(((d.x-threadFrom.x)*dx + (d.y-threadFrom.y)*dy) / (dx*dx + dy*dy))
	off := vec{d.x - threadFrom.x - dx*along, d.y - threadFrom.y - dy*along}
	if math.Hypot(off.x, off.y) > m.threadR() {
		return rgb{}, 0, false
	}
	c := hex(0x6fd8ff).mix(hex(0x9a5cff), clamp(along*1.6)).mix(hex(0xff5c9c), clamp(along*1.6-0.6))
	return m.shade(off, c), along, true
}

// shade rounds the thread, lit from the upper left.
func (m *mark) shade(off vec, c rgb) rgb {
	x, y := off.x/m.threadR(), off.y/m.threadR()
	z := math.Sqrt(max(0, 1-x*x-y*y))
	light := -0.36*x - 0.48*y + 0.8*z
	strength, shine := 1.0, 0.3
	if m.flat {
		strength, shine = 0.4, 0.06
	}
	if k := (light - 0.72) * strength; k < 0 {
		c = c.mix(hex(0x1c0b45), min(-k*0.5, 0.6))
	} else {
		c = c.mix(hex(0xffffff), k*0.35)
	}
	return c.mix(hex(0xffffff), shine*math.Pow(max(0, light), 18))
}

// paint returns the mark's colour at pixel p, if it covers p.
func (m *mark) paint(p vec) (rgb, bool) {
	d := m.design(p)
	moon, onMoon := m.moonAt(d)
	thread, along, onThread := m.threadAt(d)
	switch {
	case onThread && (!onMoon || along < threadUnder):
		if !onMoon && !m.flat {
			// Where it dips under the moon, the thread darkens.
			if gap := m.moonGap(d); gap < 28 && along >= threadUnder {
				f := 1 - gap/28
				thread = thread.mix(hex(0x14062e), 0.5*f*f)
			}
		}
		return thread, true
	case onMoon:
		if !m.flat && along < threadUnder {
			// The thread over the upper horn casts a soft shadow on it.
			if gap := m.threadGap(vec{d.x - 6, d.y - 12}); gap < 18 {
				f := 1 - gap/18
				moon = moon.mix(hex(0x7a4a1e), 0.45*f*f)
			}
		}
		return moon, true
	}
	return rgb{}, false
}

// moonGap is how far d is outside the crescent.
func (m *mark) moonGap(d vec) float64 {
	lit := math.Hypot(d.x-litCentre.x, d.y-litCentre.y) - litR
	shade := shadeR - math.Hypot(d.x-shadeCentre.x, d.y-shadeCentre.y)
	return max(lit, shade, 0)
}

// threadGap is how far d is from the thread's edge.
func (m *mark) threadGap(d vec) float64 {
	dx, dy := threadTo.x-threadFrom.x, threadTo.y-threadFrom.y
	along := clamp(((d.x-threadFrom.x)*dx + (d.y-threadFrom.y)*dy) / (dx*dx + dy*dy))
	return max(0, math.Hypot(d.x-threadFrom.x-dx*along, d.y-threadFrom.y-dy*along)-m.threadR())
}

// threadR is the thread's radius in design units; small marks draw it
// thicker, at least a pixel and a half wide.
func (m *mark) threadR() float64 { return max(threadR, 0.8/m.unit) }

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

// sky is the tile behind the mark: deep blue going to black, moonlight
// around the crescent, a few stars and a lit rim along the top.
func (m *mark) sky(p vec, edge float64) rgb {
	u := m.size / 1024
	x, y := p.x/u, p.y/u
	t := clamp((y - 100) / 824)
	c := hex(0x232456).mix(hex(0x07071a), t)
	d := m.design(p)
	glow := clamp(1 - (math.Hypot(d.x-litCentre.x, d.y-litCentre.y)-litR*0.6)/520)
	c = c.mix(hex(0x8b7cff), 0.34*glow*glow)
	for _, s := range stars {
		r := math.Hypot(x-s[0], y-s[1])
		if g := clamp(1 - r/(7+5*s[2])); g > 0 {
			c = c.mix(hex(0xfff6dc), s[2]*g*g)
		}
	}
	if edge > -max(5*u, 0.6) {
		c = c.mix(hex(0xffffff), 0.05+0.19*(1-t))
	}
	return c
}

// Icon draws the app icon.
func Icon(size int) *image.RGBA { return render(size, true) }

// Mark draws the moon and its thread alone, on a transparent background.
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
				c = m.sky(p, edge)
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
