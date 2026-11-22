//go:build desktop

package editor

import (
	"image/color"
	"math"
)

// uiColors are the interface colours, derived from the editor theme so every
// theme gets the same hierarchy and readable secondary text.
type uiColors struct {
	bg, side, raised, well, line, strongLine color.NRGBA
	text, dim, faint                         color.NRGBA
	accent, onAccent                         color.NRGBA
	hover, selected, current, selection      color.NRGBA
	open, key, keyLine, thumb                color.NRGBA
	added, removed, hunk                     color.NRGBA
	red, yellow, green, blue, purple, orange color.NRGBA
	scrim, shadow                            color.NRGBA
	dark                                     bool
}

var (
	white = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	black = color.NRGBA{A: 255}
)

// nrgba resolves a theme colour. A window has no terminal to take defaults
// from, so those fall back to fallback.
func nrgba(c terminalColor, fallback color.NRGBA) color.NRGBA {
	if c&defaultFlag != 0 {
		return fallback
	}
	if c&ansiFlag != 0 {
		r, g, b := ansiRGB(int(c & 0xff))
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
	}
	r, g, b := c.RGB()
	return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

// blend mixes b over a by t (0..1).
func blend(a, b color.NRGBA, t float32) color.NRGBA {
	mix := func(x, y uint8) uint8 { return uint8(float32(x)*(1-t) + float32(y)*t + 0.5) }
	return color.NRGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 255}
}

func fade(c color.NRGBA, alpha uint8) color.NRGBA {
	c.A = alpha
	return c
}

func luminance(c color.NRGBA) float64 {
	linear := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}

func contrast(a, b color.NRGBA) float64 {
	x, y := luminance(a), luminance(b)
	return (max(x, y) + 0.05) / (min(x, y) + 0.05)
}

// readable nudges fg towards white or black until it reaches ratio on bg.
func readable(fg, bg color.NRGBA, ratio float64) color.NRGBA {
	end := white
	if contrast(black, bg) > contrast(white, bg) {
		end = black
	}
	out := fg
	for t := float32(0.04); t <= 1 && contrast(out, bg) < ratio; t += 0.04 {
		out = blend(fg, end, t)
	}
	return out
}

func deriveColors() uiColors {
	var c uiColors
	c.bg = nrgba(th.bg, color.NRGBA{R: 0x1e, G: 0x22, B: 0x2a, A: 255})
	c.dark = luminance(c.bg) < 0.3
	fg := nrgba(th.fg, color.NRGBA{R: 0xc5, G: 0xcb, B: 0xd5, A: 255})

	c.side = nrgba(th.panel, blend(c.bg, black, 0.14))
	if c.dark {
		c.raised = blend(c.bg, white, 0.045)
		c.well = blend(c.bg, black, 0.18)
		c.scrim = fade(black, 105)
		c.shadow = fade(black, 16)
	} else {
		c.raised = blend(c.bg, white, 0.45)
		c.well = blend(c.bg, black, 0.04)
		c.scrim = fade(black, 45)
		c.shadow = fade(black, 8)
	}
	c.text = readable(fg, c.raised, 7.5)
	c.dim = readable(nrgba(th.muted, blend(c.bg, c.text, 0.5)), c.raised, 4.5)
	c.faint = readable(nrgba(th.muted, blend(c.bg, c.text, 0.4)), c.side, 3)
	c.line = blend(c.bg, c.text, 0.10)
	c.strongLine = blend(c.bg, c.text, 0.19)
	c.accent = readable(nrgba(th.accent, fg), c.bg, 3)
	c.onAccent = black
	if contrast(white, c.accent) > contrast(black, c.accent) {
		c.onAccent = white
	}
	// Opaque on purpose: Gio composites in linear space, where a light tint
	// over a dark surface comes out far stronger than its alpha suggests.
	c.hover = blend(c.raised, c.text, 0.07)
	c.selected = blend(c.raised, c.accent, 0.17)
	c.open = blend(c.side, c.text, 0.085)
	c.key = blend(c.raised, c.text, 0.05)
	c.keyLine = blend(c.raised, c.text, 0.14)
	c.thumb = blend(c.bg, c.text, 0.22)
	c.current = blend(c.bg, c.text, 0.05)
	c.selection = blend(c.bg, c.accent, 0.26)
	c.red = nrgba(th.red, fg)
	c.yellow = nrgba(th.yellow, fg)
	c.blue = nrgba(th.aqua, fg)
	c.purple = nrgba(th.purple, fg)
	c.orange = nrgba(th.orange, fg)
	c.green = color.NRGBA{R: 0x8c, G: 0xc2, B: 0x7a, A: 255}
	if !c.dark {
		c.green = color.NRGBA{R: 0x3d, G: 0x8b, B: 0x37, A: 255}
	}
	c.added = blend(c.bg, c.green, 0.14)
	c.removed = blend(c.bg, c.red, 0.14)
	c.hunk = blend(c.bg, c.blue, 0.10)
	return c
}

// cellColors translate what the renderer put in the grid into window colours.
func (d *desktop) cellBG(c terminalColor) color.NRGBA {
	switch c {
	case th.bg:
		return d.c.bg
	case th.sel:
		return d.c.selection
	case th.line:
		return d.c.current
	}
	return nrgba(c, d.c.bg)
}

func (d *desktop) cellFG(c terminalColor, bg color.NRGBA) color.NRGBA {
	key := [2]color.NRGBA{nrgba(c, d.c.text), bg}
	out, ok := d.inks[key]
	if !ok {
		out = readable(key[0], bg, 3.1)
		d.inks[key] = out
	}
	return out
}

func (d *desktop) gitInk(mark byte) color.NRGBA {
	switch mark {
	case '?', 'A', '+':
		return d.c.green
	case 'D', '-':
		return d.c.red
	}
	return nrgba(gitColor(mark), d.c.yellow)
}
