//go:build desktop

package editor

import "image/color"

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
