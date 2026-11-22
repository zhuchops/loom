//go:build desktop

package editor

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

// MainDesktop runs Loom in a native window and exits the process on failure.
func MainDesktop() {
	e, explicit, done, err := prepare()
	if err == nil && !done {
		go func() {
			err = runDesktop(e, explicit)
			if err != nil {
				fmt.Fprintln(os.Stderr, "loom:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}()
		app.Main()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "loom:", err)
		os.Exit(1)
	}
}

// desktop drives the same editor and renderer as the terminal build, but
// paints the cell grid with a GPU-rendered monospace font instead of
// writing escape sequences.
type desktop struct {
	w      *app.Window
	m      *terminalModel
	inbox  chan tea.Msg
	shaper *text.Shaper

	fontRegular, fontBold, fontIcon font.Font
	metric                          unit.Metric
	sp                              unit.Sp
	cellW, cellH, lift              int
	cols, rows                      int
	focused                         bool
	scrollX, scrollY                float32
	skipEdit                        bool
	leftDown                        bool
}

func runDesktop(e *editor, explicit bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	d := &desktop{inbox: make(chan tea.Msg, 256)}
	d.m = &terminalModel{
		editor: e,
		view:   renderer{screen: newTerminalFrame(0, 0)},
		ctx:    ctx, signals: nil, ticks: ticker.C,
	}
	d.w = new(app.Window)
	d.w.Option(app.Title("Loom"), app.Size(unit.Dp(1200), unit.Dp(780)))
	d.loadFonts()
	e.begin(explicit)
	defer e.finish()
	d.run(d.m.Init())
	var ops op.Ops
	for {
		switch ev := d.w.Event().(type) {
		case app.DestroyEvent:
			cancel()
			return ev.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, ev)
			d.frame(gtx)
			ev.Frame(gtx.Ops)
		}
	}
}

// run executes a Bubble Tea command in the background and queues its result.
func (d *desktop) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		if msg := cmd(); msg != nil {
			d.inbox <- msg
			d.w.Invalidate()
		}
	}()
}

func (d *desktop) send(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.BatchMsg:
		for _, cmd := range msg {
			d.run(cmd)
		}
		return
	case tea.QuitMsg:
		d.w.Perform(system.ActionClose)
		return
	}
	_, cmd := d.m.Update(msg)
	d.run(cmd)
}

func (d *desktop) drain() {
	for {
		select {
		case msg := <-d.inbox:
			d.send(msg)
		default:
			return
		}
	}
}

func (d *desktop) loadFonts() {
	var faces []font.FontFace
	load := func(path string) []font.FontFace {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		parsed, err := opentype.ParseCollection(raw)
		if err != nil {
			return nil
		}
		return parsed
	}
	home, _ := os.UserHomeDir()
	primary := os.Getenv("LOOM_FONT")
	var main []font.FontFace
	if primary != "" {
		main = load(primary)
	}
	for _, path := range []string{"/System/Library/Fonts/Menlo.ttc", "/System/Library/Fonts/Monaco.ttf", "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"} {
		if len(main) > 0 {
			break
		}
		main = load(path)
	}
	var icons []font.FontFace
	if matches, _ := filepath.Glob(filepath.Join(home, "Library/Fonts/*NerdFontMono-Regular.*")); len(matches) > 0 {
		for _, m := range matches {
			if !strings.Contains(m, "BigBlue") && !strings.Contains(m, "Gohu") {
				icons = load(m)
				break
			}
		}
		if len(icons) == 0 {
			icons = load(matches[0])
		}
	}
	faces = append(faces, main...)
	faces = append(faces, icons...)
	faces = append(faces, gofont.Collection()...)
	d.shaper = text.NewShaper(text.WithCollection(faces))
	d.fontRegular = font.Font{Typeface: "Go Mono"}
	if len(main) > 0 {
		d.fontRegular = main[0].Font
		d.fontRegular.Weight, d.fontRegular.Style = font.Normal, font.Regular
	}
	d.fontBold = d.fontRegular
	d.fontBold.Weight = font.Bold
	d.fontIcon = d.fontRegular
	if len(icons) > 0 {
		d.fontIcon = icons[0].Font
	}
}

// measure picks a font size whose glyph advance is a whole number of pixels,
// so a run of text stays aligned with the cell grid.
func (d *desktop) measure(gtx layout.Context) {
	if d.cellW > 0 && gtx.Metric == d.metric {
		return
	}
	d.metric = gtx.Metric
	d.sp = 15
	var w, h int
	for range 3 {
		w, h = d.size(gtx, d.fontRegular, strings.Repeat("M", 200))
		adv := float64(w) / 200
		target := math.Round(adv)
		if target < 1 || math.Abs(adv-target) < 0.01 {
			break
		}
		d.sp = unit.Sp(float64(d.sp) * target / adv)
	}
	d.cellW = max(1, int(math.Round(float64(w)/200)))
	d.cellH = h * 118 / 100
	d.lift = (d.cellH - h) / 2
}

func (d *desktop) size(gtx layout.Context, f font.Font, s string) (int, int) {
	gtx.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
	macro := op.Record(gtx.Ops)
	paint.ColorOp{Color: color.NRGBA{A: 255}}.Add(gtx.Ops)
	call := macro.Stop()
	m := op.Record(gtx.Ops)
	dims := widget.Label{MaxLines: 1}.Layout(gtx, d.shaper, f, d.sp, s, call)
	m.Stop()
	return dims.Size.X, dims.Size.Y
}

func (d *desktop) color(c terminalColor, fallback terminalColor) color.NRGBA {
	switch {
	case c&defaultFlag != 0:
		c = fallback
		if c&defaultFlag != 0 {
			c = rgb(0x1e222a)
		}
	}
	if c&ansiFlag != 0 {
		r, g, b := ansiRGB(int(c & 0xff))
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
	}
	r, g, b := c.RGB()
	return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

func (d *desktop) frame(gtx layout.Context) {
	d.measure(gtx)
	d.drain()
	size := gtx.Constraints.Max
	cols, rows := max(1, size.X/d.cellW), max(1, size.Y/d.cellH)
	if cols != d.cols || rows != d.rows {
		d.cols, d.rows = cols, rows
		d.send(tea.WindowSizeMsg{Width: cols, Height: rows})
	}
	d.input(gtx)
	d.m.view.draw(d.m.editor)
	if d.m.editor.quit {
		d.w.Perform(system.ActionClose)
	}
	d.paint(gtx, size)
	d.m.editor.animating() // keep the frame loop running while animations play
	if d.m.editor.animating() {
		d.w.Invalidate()
	}
}

func (d *desktop) input(gtx layout.Context) {
	tag := d
	if !d.focused {
		gtx.Execute(key.FocusCmd{Tag: tag})
		d.focused = true
	}
	// Pointer input covers the whole window.
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, tag)
	area.Pop()
	any := key.ModCtrl | key.ModCommand | key.ModShift | key.ModAlt | key.ModSuper
	filters := []event.Filter{
		key.FocusFilter{Target: tag},
		pointer.Filter{Target: tag, Kinds: pointer.Press | pointer.Release | pointer.Move | pointer.Drag | pointer.Scroll,
			ScrollX: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}},
		key.Filter{Focus: tag, Optional: any},
	}
	for _, name := range []key.Name{key.NameTab, key.NameEscape, key.NameReturn, key.NameEnter, key.NameLeftArrow, key.NameRightArrow,
		key.NameUpArrow, key.NameDownArrow, key.NameHome, key.NameEnd, key.NamePageUp, key.NamePageDown, key.NameDeleteBackward,
		key.NameDeleteForward, key.NameSpace} {
		filters = append(filters, key.Filter{Focus: tag, Name: name, Optional: any})
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		switch ev := ev.(type) {
		case key.Event:
			if ev.State == key.Press {
				d.key(ev)
			}
		case key.EditEvent:
			d.edit(ev.Text)
		case pointer.Event:
			d.pointer(ev)
		}
	}
	d.skipEdit = false
}

var guiNamedKeys = map[key.Name]rune{
	key.NameReturn: tea.KeyEnter, key.NameEnter: tea.KeyEnter, key.NameEscape: tea.KeyEscape, key.NameTab: tea.KeyTab,
	key.NameDeleteBackward: tea.KeyBackspace, key.NameDeleteForward: tea.KeyDelete,
	key.NameLeftArrow: tea.KeyLeft, key.NameRightArrow: tea.KeyRight, key.NameUpArrow: tea.KeyUp, key.NameDownArrow: tea.KeyDown,
	key.NameHome: tea.KeyHome, key.NameEnd: tea.KeyEnd, key.NamePageUp: tea.KeyPgUp, key.NamePageDown: tea.KeyPgDown,
}

func (d *desktop) key(ev key.Event) {
	var mod tea.KeyMod
	if ev.Modifiers.Contain(key.ModCtrl) || ev.Modifiers.Contain(key.ModCommand) {
		mod |= tea.ModCtrl
	}
	if ev.Modifiers.Contain(key.ModAlt) {
		mod |= tea.ModAlt
	}
	if ev.Modifiers.Contain(key.ModShift) {
		mod |= tea.ModShift
	}
	if code, ok := guiNamedKeys[ev.Name]; ok {
		d.send(tea.KeyPressMsg(tea.Key{Code: code, Mod: mod}))
		d.skipEdit = true
		return
	}
	// Plain printable keys arrive as text; only shortcuts are handled here.
	if mod&(tea.ModCtrl|tea.ModAlt) == 0 {
		return
	}
	d.skipEdit = true
	if ev.Name == key.NameSpace {
		d.send(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace, Mod: mod}))
		return
	}
	r, _ := utf8.DecodeRuneInString(strings.ToLower(string(ev.Name)))
	if utf8.RuneCountInString(string(ev.Name)) != 1 || r == utf8.RuneError {
		return
	}
	d.send(tea.KeyPressMsg(tea.Key{Code: r, Mod: mod}))
}

func (d *desktop) edit(s string) {
	if s == "" || d.skipEdit {
		return
	}
	if utf8.RuneCountInString(s) > 1 {
		d.send(tea.PasteMsg{Content: s})
		return
	}
	r, _ := utf8.DecodeRuneInString(s)
	if r < 0x20 || r == 0x7f {
		return
	}
	d.send(tea.KeyPressMsg(tea.Key{Code: r, Text: s}))
}

func (d *desktop) pointer(ev pointer.Event) {
	x, y := int(ev.Position.X)/d.cellW, int(ev.Position.Y)/d.cellH
	mod := tea.KeyMod(0)
	if ev.Modifiers.Contain(key.ModShift) {
		mod |= tea.ModShift
	}
	button := tea.MouseNone
	switch {
	case ev.Buttons.Contain(pointer.ButtonPrimary):
		button = tea.MouseLeft
	case ev.Buttons.Contain(pointer.ButtonSecondary):
		button = tea.MouseRight
	case ev.Buttons.Contain(pointer.ButtonTertiary):
		button = tea.MouseMiddle
	}
	mouse := tea.Mouse{X: x, Y: y, Button: button, Mod: mod}
	switch ev.Kind {
	case pointer.Press:
		d.leftDown = button == tea.MouseLeft
		d.send(tea.MouseClickMsg(mouse))
	case pointer.Release:
		mouse.Button = tea.MouseLeft
		d.leftDown = false
		d.send(tea.MouseReleaseMsg(mouse))
	case pointer.Move, pointer.Drag:
		d.send(tea.MouseMotionMsg(mouse))
	case pointer.Scroll:
		// One wheel step per three rows of finger travel.
		d.scrollX += ev.Scroll.X
		d.scrollY += ev.Scroll.Y
		step := float32(d.cellH * 3)
		for d.scrollY >= step {
			d.scrollY -= step
			mouse.Button = tea.MouseWheelDown
			d.send(tea.MouseWheelMsg(mouse))
		}
		for d.scrollY <= -step {
			d.scrollY += step
			mouse.Button = tea.MouseWheelUp
			d.send(tea.MouseWheelMsg(mouse))
		}
		for d.scrollX >= step {
			d.scrollX -= step
			mouse.Button = tea.MouseWheelRight
			d.send(tea.MouseWheelMsg(mouse))
		}
		for d.scrollX <= -step {
			d.scrollX += step
			mouse.Button = tea.MouseWheelLeft
			d.send(tea.MouseWheelMsg(mouse))
		}
	}
}

func (d *desktop) paint(gtx layout.Context, size image.Point) {
	f := d.m.view.screen
	fill := func(r image.Rectangle, c color.NRGBA) {
		paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
	}
	fill(image.Rectangle{Max: size}, d.color(th.bg, th.bg))
	for y := 0; y < f.Height(); y++ {
		// Backgrounds first, merged along the row.
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
				fill(image.Rect(x*d.cellW, y*d.cellH, end*d.cellW, (y+1)*d.cellH), d.color(style.bg, th.bg))
			}
			x = end
		}
	}
	// The cursor is a block that inverts the cell it covers, or a bar.
	cursorX, cursorY := -1, -1
	if f.cursorVisible {
		cursorX, cursorY = f.cursorX, f.cursorY
		r := image.Rect(cursorX*d.cellW, cursorY*d.cellH, (cursorX+1)*d.cellW, (cursorY+1)*d.cellH)
		if f.cursorShape == cursorBar {
			r.Max.X = r.Min.X + max(2, d.cellW/6)
		}
		fill(r, d.color(th.fg, th.fg))
	}
	for y := 0; y < f.Height(); y++ {
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
			ascii := len(s) == 1 && s[0] > 0x20 && s[0] < 0x7f
			if !ascii {
				d.glyph(gtx, x*d.cellW, y*d.cellH, s, d.color(fg, th.fg), style.bold)
				if style.underline != 0 {
					fill(image.Rect(x*d.cellW, (y+1)*d.cellH-2, (x+width)*d.cellW, (y+1)*d.cellH-1), d.color(style.underline, th.fg))
				}
				x += width
				continue
			}
			// Gather a run of ASCII with one look so it is shaped once.
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
					break // the cursor cell has its own colour
				}
			}
			d.glyph(gtx, start*d.cellW, y*d.cellH, run.String(), d.color(fg, th.fg), style.bold)
			if style.underline != 0 {
				fill(image.Rect(start*d.cellW, (y+1)*d.cellH-2, x*d.cellW, (y+1)*d.cellH-1), d.color(style.underline, th.fg))
			}
		}
	}
}

func isIcon(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return r >= 0xe000 && r <= 0xf8ff || r >= 0xf0000
}

func (d *desktop) glyph(gtx layout.Context, x, y int, s string, c color.NRGBA, bold bool) {
	f := d.fontRegular
	if bold {
		f = d.fontBold
	}
	if isIcon(s) {
		f = d.fontIcon
	}
	defer op.Offset(image.Pt(x, y+d.lift)).Push(gtx.Ops).Pop()
	macro := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	call := macro.Stop()
	gtx.Constraints = layout.Constraints{Max: image.Pt(1<<20, d.cellH*2)}
	widget.Label{MaxLines: 1}.Layout(gtx, d.shaper, f, d.sp, s, call)
}
