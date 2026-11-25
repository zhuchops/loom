package editor

import (
	"encoding/json"
	"fmt"
	"image/color"
	"strings"
)

// frameState is what the web view already has, so a frame carries only
// what changed.
type frameState struct {
	rows   []string // each row's encoding as last sent
	colors uiColors
	inks   map[[2]color.NRGBA]color.NRGBA
	look   string
	ui     string
}

// cellRun is a stretch of one row in one style: [text, x, cells, fg, bg,
// flags, underline]. Flags: 1 bold, 2 italic.
type cellRun []any

// gridFrame is one update for the web view. Rows maps a row index to its
// runs; only changed rows are present.
type gridFrame struct {
	Cols   int               `json:"cols"`
	Rows   int               `json:"rows"`
	Grid   map[int][]cellRun `json:"grid,omitempty"`
	Cursor *cursorState      `json:"cursor"`
	Theme  map[string]string `json:"theme,omitempty"`
	Look   map[string]any    `json:"look,omitempty"`
	UI     map[string]any    `json:"ui,omitempty"`
}

type cursorState struct {
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Shape string `json:"shape"`
	Blink bool   `json:"blink"`
}

func hexColor(c color.NRGBA) string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%.3f)", c.R, c.G, c.B, float64(c.A)/255)
}

func (b *Bridge) cellBG(c terminalColor) color.NRGBA {
	ui := &b.frame.colors
	switch c {
	case th.bg:
		return ui.bg
	case th.sel:
		return ui.selection
	case th.line:
		return ui.current
	}
	return nrgba(c, ui.bg)
}

func (b *Bridge) cellFG(c terminalColor, bg color.NRGBA) color.NRGBA {
	key := [2]color.NRGBA{nrgba(c, b.frame.colors.text), bg}
	out, ok := b.frame.inks[key]
	if !ok {
		out = readable(key[0], bg, 3.1)
		b.frame.inks[key] = out
	}
	return out
}

// render lays the frame out and sends what changed to the web view.
func (b *Bridge) render() {
	if b.host == nil || !b.started || b.cols == 0 {
		return
	}
	e := b.m.editor
	v := &b.m.view
	v.draw(e)
	out := gridFrame{Cols: b.cols, Rows: b.rows}

	if c := deriveColors(); c != b.frame.colors || b.frame.inks == nil {
		b.frame.colors, b.frame.inks, b.frame.rows = c, map[[2]color.NRGBA]color.NRGBA{}, nil
		out.Theme = themeVars(c)
		b.host.appearance(c.dark)
		b.frame.ui = "" // the chrome's colours changed with the theme
	}
	o := &e.cfg.options
	look := map[string]any{
		"family": o.FontFamily, "fallback": o.FontFallback, "ui": o.UIFontFamily,
		"weight": o.FontWeight, "bold": o.FontBoldWeight, "uiWeight": o.UIFontWeight,
		"size": o.FontSize, "height": o.LineHeight, "spacing": o.LetterSpacing, "uiSize": o.UIFontSize,
		"ligatures": o.FontLigatures, "animations": o.Animations && animationsOn, "glide": o.CursorGlide, "smooth": o.SmoothScroll,
	}
	if key := fmt.Sprint(look); key != b.frame.look {
		b.frame.look, out.Look = key, look
	}

	f := v.screen
	w, h := f.Width(), f.Height()
	if len(b.frame.rows) != h {
		b.frame.rows = make([]string, h)
	}
	for y := range h {
		runs := b.encodeRow(f, y, w)
		key := fmt.Sprint(runs)
		if key == b.frame.rows[y] {
			continue
		}
		b.frame.rows[y] = key
		if out.Grid == nil {
			out.Grid = map[int][]cellRun{}
		}
		out.Grid[y] = runs
	}

	if f.cursorVisible {
		cfg := &e.cfg.options
		shape := cfg.CursorNormal
		if f.cursorShape == cursorBar {
			shape = cfg.CursorInsert
		}
		out.Cursor = &cursorState{X: f.cursorX, Y: f.cursorY, Shape: shape, Blink: cfg.CursorBlink}
	}
	ui := b.chrome()
	if data, err := json.Marshal(ui); err == nil && string(data) != b.frame.ui {
		b.frame.ui, out.UI = string(data), ui
	}
	b.host.emit(out)
}

// encodeRow groups a row of the grid into runs of one style.
func (b *Bridge) encodeRow(f *terminalFrame, y, w int) []cellRun {
	ui := &b.frame.colors
	var runs []cellRun
	for x := 0; x < w; {
		s, style, width := f.Get(x, y)
		width = max(1, width)
		bg := b.cellBG(style.bg)
		fg := b.cellFG(style.fg, bg)
		if s == "▏" { // indent guide
			fg = blend(bg, ui.text, 0.13)
		}
		flags := 0
		if style.bold {
			flags |= 1
		}
		if style.italic {
			flags |= 2
		}
		under := ""
		if style.underline != 0 {
			under = hexColor(nrgba(style.underline, ui.red))
		}
		if s == "" {
			s = " "
		}
		start := x
		var text strings.Builder
		text.WriteString(s)
		x += width
		// Plain ASCII of one style shares a run; anything else stands alone
		// so the grid keeps its columns.
		if simpleCell(s) {
			for x < w {
				t, next, nw := f.Get(x, y)
				if t == "" {
					t = " "
				}
				if next != style || !simpleCell(t) {
					break
				}
				text.WriteString(t)
				x += max(1, nw)
			}
		}
		if text.String() == strings.Repeat(" ", x-start) && style.bg == th.bg && under == "" {
			continue // the row's background shows through
		}
		runs = append(runs, cellRun{text.String(), start, x - start, hexColor(fg), hexColor(bg), flags, under})
	}
	return runs
}

func simpleCell(s string) bool { return len(s) == 1 && s[0] >= 0x20 && s[0] < 0x7f }

// synKinds are the token kinds a Markdown preview colours.
var synKinds = []string{"keyword", "string", "regexp", "number", "constant", "comment", "function", "method", "type",
	"class", "property", "namespace", "parameter", "operator", "decorator", "label", "macro", "modifier", "enumMember"}

// themeVars are the interface colours as CSS custom properties.
func themeVars(c uiColors) map[string]string {
	vars := map[string]string{
		"bg": hexColor(c.bg), "side": hexColor(c.side), "raised": hexColor(c.raised), "well": hexColor(c.well),
		"line": hexColor(c.line), "strong-line": hexColor(c.strongLine), "text": hexColor(c.text), "dim": hexColor(c.dim),
		"faint": hexColor(c.faint), "accent": hexColor(c.accent), "on-accent": hexColor(c.onAccent), "hover": hexColor(c.hover),
		"selected": hexColor(c.selected), "current": hexColor(c.current), "selection": hexColor(c.selection), "open": hexColor(c.open),
		"key": hexColor(c.key), "key-line": hexColor(c.keyLine), "thumb": hexColor(c.thumb), "added": hexColor(c.added),
		"removed": hexColor(c.removed), "hunk": hexColor(c.hunk), "red": hexColor(c.red), "yellow": hexColor(c.yellow),
		"green": hexColor(c.green), "blue": hexColor(c.blue), "purple": hexColor(c.purple), "orange": hexColor(c.orange),
	}
	// Code colours, for code blocks in the Markdown preview.
	for _, kind := range synKinds {
		if h := semanticColor(kind); h != 0 {
			vars["syn-"+kind] = hexColor(nrgba(rgb(h), c.text))
		}
	}
	vars["syn-comment"] = hexColor(nrgba(th.comment, c.dim))
	vars["scheme"] = "light"
	if c.dark {
		vars["scheme"] = "dark"
	}
	return vars
}
