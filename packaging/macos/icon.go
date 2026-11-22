//go:build ignore

// icon.go draws the Loom app icon: warp and weft threads woven over a dark
// rounded square. Usage: go run icon.go out.png
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const size = 1024

// roundedBox is the signed distance from (x, y) to a rounded rectangle.
func roundedBox(x, y, cx, cy, hw, hh, r float64) float64 {
	qx, qy := math.Abs(x-cx)-(hw-r), math.Abs(y-cy)-(hh-r)
	return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - r
}

func cover(d float64) float64 { return math.Max(0, math.Min(1, 0.5-d)) }

func mix(a, b color.NRGBA, t float64) color.NRGBA {
	f := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t) }
	return color.NRGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 255}
}

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	top, bottom := color.NRGBA{0x2c, 0x32, 0x3d, 255}, color.NRGBA{0x17, 0x1a, 0x21, 255}
	warp := []color.NRGBA{{0x61, 0xaf, 0xef, 255}, {0x56, 0xb6, 0xc2, 255}}
	weft := []color.NRGBA{{0xc8, 0x82, 0xe7, 255}, {0xe0, 0x6c, 0x75, 255}}
	const margin, box = 100.0, 824.0
	const threads = 5
	pitch := (box - 2*130) / float64(threads)
	width := pitch * 0.62
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			x, y := float64(px)+0.5, float64(py)+0.5
			mask := cover(roundedBox(x, y, size/2, size/2, box/2, box/2, 186))
			if mask == 0 {
				continue
			}
			c := mix(top, bottom, y/size)
			// Soft inner highlight along the top edge.
			c = mix(c, color.NRGBA{0xff, 0xff, 0xff, 255}, 0.05*math.Max(0, 1-(y-margin)/180))
			ox, oy := margin+130, margin+130
			var vi, hi = -1, -1
			var vd, hd float64 = 1e9, 1e9
			for i := 0; i < threads; i++ {
				cx := ox + pitch*(float64(i)+0.5)
				d := roundedBox(x, y, cx, size/2, width/2, box/2-100, width/2)
				if d < vd {
					vd, vi = d, i
				}
				cy := oy + pitch*(float64(i)+0.5)
				d = roundedBox(x, y, size/2, cy, box/2-100, width/2, width/2)
				if d < hd {
					hd, hi = d, i
				}
			}
			vcol := mix(warp[0], warp[1], float64(vi)/float64(threads-1))
			hcol := mix(weft[0], weft[1], float64(hi)/float64(threads-1))
			va, ha := cover(vd), cover(hd)
			// Over and under: the thread on top alternates like a real weave.
			if (vi+hi)%2 == 0 {
				c = mix(c, hcol, ha)
				c = mix(c, color.NRGBA{0, 0, 0, 255}, 0.28*va*ha)
				c = mix(c, vcol, va)
			} else {
				c = mix(c, vcol, va)
				c = mix(c, color.NRGBA{0, 0, 0, 255}, 0.28*va*ha)
				c = mix(c, hcol, ha)
			}
			c.A = uint8(255 * mask)
			img.SetNRGBA(px, py, c)
		}
	}
	f, err := os.Create(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}
