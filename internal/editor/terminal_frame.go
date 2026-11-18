package editor

import (
	"image/color"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Keep theme colors concrete and comparable so rendering and tests can look
// up the style of a cell without depending on terminal input or output.
type terminalColor uint32

func rgb(hex int32) terminalColor  { return terminalColor(hex) }
func (c terminalColor) Hex() int32 { return int32(c) }
func (c terminalColor) RGB() (int32, int32, int32) {
	if c&ansiFlag != 0 {
		return ansiRGB(int(c & 0xff))
	}
	if c&defaultFlag != 0 {
		return 0, 0, 0
	}
	return int32(c>>16) & 255, int32(c>>8) & 255, int32(c) & 255
}
func (c terminalColor) RGBA() (r, g, b, a uint32) {
	red, green, blue := c.RGB()
	return uint32(red) * 257, uint32(green) * 257, uint32(blue) * 257, 65535
}

type cellStyle struct {
	fg, bg    terminalColor
	bold      bool
	underline terminalColor
}

func (s cellStyle) Foreground(c terminalColor) cellStyle { s.fg = c; return s }
func (s cellStyle) Background(c terminalColor) cellStyle { s.bg = c; return s }
func (s cellStyle) Bold(on bool) cellStyle               { s.bold = on; return s }
func (s cellStyle) GetForeground() terminalColor         { return s.fg }
func (s cellStyle) GetBackground() terminalColor         { return s.bg }

// uvColor maps palette and default colors to the terminal's own.
func (c terminalColor) uvColor() color.Color {
	switch {
	case c&defaultFlag != 0:
		return nil
	case c&ansiFlag != 0:
		return ansi.BasicColor(c & 0xff)
	}
	return c
}

// ansiRGB approximates a palette color, for dimming and blending only.
func ansiRGB(n int) (int32, int32, int32) {
	table := [16]int32{0x000000, 0xcd3131, 0x0dbc79, 0xe5e510, 0x2472c8, 0xbc3fbc, 0x11a8cd, 0xe5e5e5,
		0x666666, 0xf14c4c, 0x23d18b, 0xf5f543, 0x3b8eea, 0xd670d6, 0x29b8db, 0xffffff}
	c := table[n&15]
	return c >> 16 & 255, c >> 8 & 255, c & 255
}

func (s cellStyle) uvStyle() uv.Style {
	style := uv.Style{Fg: s.fg.uvColor(), Bg: s.bg.uvColor()}
	if s.bold {
		style.Attrs = uv.AttrBold
	}
	if s.underline != 0 {
		style.Underline, style.UnderlineColor = uv.UnderlineCurly, s.underline.uvColor()
	}
	return style
}

func frameColor(c color.Color) terminalColor {
	switch value := c.(type) {
	case terminalColor:
		return value
	case ansi.BasicColor:
		return ansiColor(int(value))
	case nil:
		if th.terminal {
			return defaultFlag
		}
	}
	return rgb(0x202629)
}

type cursorShape uint8

const (
	cursorBlock cursorShape = iota
	cursorBar
)

// terminalFrame holds the composed cells and cursor for one draw. Bubble Tea
// owns terminal output and diffs the frame against what it last displayed.
type terminalFrame struct {
	*uv.Buffer
	cursorX, cursorY int
	cursorVisible    bool
	cursorShape      cursorShape
}

func newTerminalFrame(w, h int) *terminalFrame {
	return &terminalFrame{Buffer: uv.NewBuffer(w, h)}
}

func (f *terminalFrame) Size() (int, int) { return f.Width(), f.Height() }

func (f *terminalFrame) Fill(r rune, style cellStyle) {
	f.FillArea(0, 0, f.Width(), f.Height(), r, style)
}
func (f *terminalFrame) FillArea(x, y, w, h int, r rune, style cellStyle) {
	f.Buffer.FillArea(&uv.Cell{Content: string(r), Style: style.uvStyle(), Width: 1}, uv.Rect(x, y, w, h).Intersect(f.Bounds()))
}

func (f *terminalFrame) Put(x, y int, cluster string, width int, style cellStyle) {
	if width > 0 && x >= 0 && x+width <= f.Width() {
		f.SetCell(x, y, &uv.Cell{Content: cluster, Style: style.uvStyle(), Width: width})
	}
}

func (f *terminalFrame) Get(x, y int) (string, cellStyle, int) {
	cell := f.CellAt(x, y)
	if cell == nil {
		return "", cellStyle{}, 0
	}
	s := cell.Style
	style := cellStyle{fg: frameColor(s.Fg), bg: frameColor(s.Bg), bold: s.Attrs&uv.AttrBold != 0}
	if s.Underline != uv.UnderlineNone {
		style.underline = frameColor(s.UnderlineColor)
	}
	return cell.Content, style, cell.Width
}

func (f *terminalFrame) ShowCursor(x, y int) {
	f.cursorX, f.cursorY = x, y
	f.cursorVisible = uv.Pos(x, y).In(f.Bounds())
}
func (f *terminalFrame) HideCursor()                  { f.cursorVisible = false }
func (f *terminalFrame) SetCursorStyle(s cursorShape) { f.cursorShape = s }
