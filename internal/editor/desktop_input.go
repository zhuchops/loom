//go:build desktop

package editor

import (
	"image"
	"runtime"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/op/clip"
)

// zone is a part of the window that reacts to the pointer. Later zones are on
// top of earlier ones.
type zone struct {
	r      rect
	layer  int
	cursor pointer.Cursor
	grid   bool // events go to the editor as cells
	click  func()
	right  func()
	middle func()
	drag   func()
	drop   func()
	wheel  func(dx, dy int)
	step   int // pixels of travel for one wheel step
}

func (d *desktop) zone(z zone) {
	if z.r.Empty() {
		return
	}
	z.layer = d.layer
	d.zones = append(d.zones, z)
}

// button registers a clickable area and reports whether the pointer is on it.
func (d *desktop) button(r rect, click func()) bool {
	d.zone(zone{r: r, click: click, cursor: pointer.CursorPointer})
	return d.hot(r)
}

// block swallows pointer input beneath r.
func (d *desktop) block(r rect) { d.zone(zone{r: r}) }

func (d *desktop) hot(r rect) bool {
	return d.grab == nil && d.hoverLayer <= d.layer && image.Pt(d.mx, d.my).In(r)
}

func (d *desktop) zoneAt() *zone {
	p := image.Pt(d.mx, d.my)
	for i := len(d.zones) - 1; i >= 0; i-- {
		if p.In(d.zones[i].r) {
			return &d.zones[i]
		}
	}
	return nil
}

// inputArea declares the window to the input router once everything is painted.
func (d *desktop) inputArea() {
	ops := d.gtx.Ops
	d.hoverLayer = layerBase
	cursor := pointer.CursorDefault
	if z := d.zoneAt(); z != nil {
		d.hoverLayer, cursor = z.layer, z.cursor
	}
	if d.grab != nil {
		cursor = d.grab.cursor
	}
	defer clip.Rect(d.geo.window).Push(ops).Pop()
	event.Op(ops, d)
	cursor.Add(ops)
	if d.hoverLayer != layerBase || d.modal(d.m.editor) {
		return // a popup is up, so the title bar is not a handle
	}
	for _, r := range d.drags {
		area := clip.Rect(r).Push(ops)
		system.ActionInputOp(system.ActionMove).Add(ops)
		area.Pop()
	}
}

var namedKeys = map[key.Name]rune{
	key.NameReturn: tea.KeyEnter, key.NameEnter: tea.KeyEnter, key.NameEscape: tea.KeyEscape, key.NameTab: tea.KeyTab,
	key.NameDeleteBackward: tea.KeyBackspace, key.NameDeleteForward: tea.KeyDelete,
	key.NameLeftArrow: tea.KeyLeft, key.NameRightArrow: tea.KeyRight, key.NameUpArrow: tea.KeyUp, key.NameDownArrow: tea.KeyDown,
	key.NameHome: tea.KeyHome, key.NameEnd: tea.KeyEnd, key.NamePageUp: tea.KeyPgUp, key.NamePageDown: tea.KeyPgDown,
}

// Shortcuts name the Latin key in the same place on a Russian layout.
var latinKeys = func() map[rune]rune {
	m := map[rune]rune{}
	latin := []rune("qwertyuiop[]asdfghjkl;'zxcvbnm,.`")
	for i, r := range []rune("йцукенгшщзхъфывапролджэячсмитьбюё") {
		m[r] = latin[i]
	}
	return m
}()

func (d *desktop) input() {
	gtx := d.gtx
	if !d.focused {
		gtx.Execute(key.FocusCmd{Tag: d})
		d.focused = true
	}
	anyMod := key.ModCtrl | key.ModCommand | key.ModShift | key.ModAlt | key.ModSuper
	filters := []event.Filter{
		key.FocusFilter{Target: d},
		pointer.Filter{
			Target:  d,
			Kinds:   pointer.Press | pointer.Release | pointer.Move | pointer.Drag | pointer.Scroll | pointer.Leave | pointer.Cancel,
			ScrollX: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20},
			ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20},
		},
		key.Filter{Focus: d, Optional: anyMod},
	}
	for name := range namedKeys {
		filters = append(filters, key.Filter{Focus: d, Name: name, Optional: anyMod})
	}
	filters = append(filters, key.Filter{Focus: d, Name: key.NameSpace, Optional: anyMod})

	var events []event.Event
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		events = append(events, ev)
	}
	for i, ev := range events {
		switch ev := ev.(type) {
		case key.Event:
			if ev.State == key.Press {
				d.key(ev)
			}
		case key.EditEvent:
			// macOS types a character for Option+key before reporting the key;
			// the binding wins over that character.
			if !optionText(events[i+1:]) {
				d.edit(ev.Text)
			}
		case pointer.Event:
			d.pointer(ev)
		}
	}
}

func optionText(next []event.Event) bool {
	for _, ev := range next {
		switch ev := ev.(type) {
		case key.Event:
			return ev.State == key.Press && ev.Modifiers.Contain(key.ModAlt)
		case key.EditEvent:
			return false
		}
	}
	return false
}

func (d *desktop) key(ev key.Event) {
	if d.dialog != nil {
		d.dialogKey(ev)
		return
	}
	name, code := ev.Name, rune(0)
	var mod tea.KeyMod
	command := ev.Modifiers.Contain(key.ModCommand)
	if command && runtime.GOOS == "darwin" {
		// The menu bar owns Command shortcuts; these move the cursor.
		switch name {
		case key.NameLeftArrow:
			name = key.NameHome
		case key.NameRightArrow:
			name = key.NameEnd
		case key.NameUpArrow:
			name = key.NamePageUp
		case key.NameDownArrow:
			name = key.NamePageDown
		default:
			return
		}
	} else if command {
		mod |= tea.ModCtrl
	}
	if ev.Modifiers.Contain(key.ModCtrl) {
		mod |= tea.ModCtrl
	}
	if ev.Modifiers.Contain(key.ModAlt) {
		mod |= tea.ModAlt
	}
	if ev.Modifiers.Contain(key.ModShift) {
		mod |= tea.ModShift
	}
	if c, ok := namedKeys[name]; ok {
		d.send(tea.KeyPressMsg(tea.Key{Code: c, Mod: mod}))
		return
	}
	// Plain printable keys arrive as text; only chords are handled here.
	if mod&(tea.ModCtrl|tea.ModAlt) == 0 {
		return
	}
	switch {
	case name == key.NameSpace:
		code = tea.KeySpace
	case utf8.RuneCountInString(string(name)) == 1:
		code, _ = utf8.DecodeRuneInString(strings.ToLower(string(name)))
		if latin, ok := latinKeys[code]; ok {
			code = latin
		}
	default:
		return
	}
	d.send(tea.KeyPressMsg(tea.Key{Code: code, Mod: mod}))
}

func (d *desktop) edit(s string) {
	if d.dialog != nil || s == "" {
		return
	}
	if utf8.RuneCountInString(s) > 1 {
		d.send(tea.PasteMsg{Content: s})
		return
	}
	if r, _ := utf8.DecodeRuneInString(s); r >= 0x20 && r != 0x7f {
		d.send(tea.KeyPressMsg(tea.Key{Code: r, Text: s}))
	}
}

func floorDiv(a, b int) int {
	if a < 0 {
		return -((b - 1 - a) / b)
	}
	return a / b
}

// cell is the grid cell under the pointer.
func (d *desktop) cell() (int, int) {
	g := d.geo.grid.Min
	return floorDiv(d.mx-g.X, d.cellW), floorDiv(d.my-g.Y, d.cellH)
}

func (d *desktop) pointer(ev pointer.Event) {
	if d.cellW == 0 {
		return
	}
	d.mx, d.my = int(ev.Position.X), int(ev.Position.Y)
	top := d.zoneAt()
	e, v := d.m.editor, &d.m.view
	x, y := d.cell()
	mouse := tea.Mouse{X: x, Y: y}
	if ev.Modifiers.Contain(key.ModShift) {
		mouse.Mod = tea.ModShift
	}
	switch ev.Kind {
	case pointer.Press:
		if top == nil {
			return
		}
		z := *top
		switch {
		case z.grid:
			// The padding around the text belongs to the nearest cell.
			mouse.X, mouse.Y = min(d.cols-1, max(0, x)), min(d.rows-1, max(0, y))
			switch {
			case ev.Buttons.Contain(pointer.ButtonSecondary):
				mouse.Button = tea.MouseRight
			case ev.Buttons.Contain(pointer.ButtonTertiary):
				mouse.Button = tea.MouseMiddle
			default:
				mouse.Button = tea.MouseLeft
				d.grab = &z
				e.tree.focused, e.tree.popup = false, false
			}
			d.send(tea.MouseClickMsg(mouse))
		case ev.Buttons.Contain(pointer.ButtonSecondary):
			if z.right != nil {
				z.right()
			}
		case ev.Buttons.Contain(pointer.ButtonTertiary):
			if z.middle != nil {
				z.middle()
			}
		default:
			if z.click != nil {
				z.click()
			}
			if z.drag != nil {
				d.grab = &z
			}
		}
	case pointer.Release, pointer.Cancel:
		z := d.grab
		d.grab = nil
		switch {
		case z == nil:
		case z.grid:
			mouse.Button = tea.MouseLeft
			d.send(tea.MouseReleaseMsg(mouse))
		case z.drop != nil:
			z.drop()
		}
	case pointer.Move, pointer.Drag:
		switch {
		case d.grab != nil && d.grab.grid:
			mouse.Button = tea.MouseLeft
			d.send(tea.MouseMotionMsg(mouse))
		case d.grab != nil:
			d.grab.drag()
		case top != nil && top.grid:
			d.send(tea.MouseMotionMsg(mouse))
		case top == nil || top.layer != layerPopover:
			v.clearDiagnosticHover()
		}
	case pointer.Leave:
		if d.grab == nil {
			d.mx, d.my = -1, -1
			v.clearDiagnosticHover()
		}
	case pointer.Scroll:
		if top == nil {
			return
		}
		step := float32(max(1, d.u(26)))
		if top.grid {
			step = float32(d.cellH)
		} else if top.step > 0 {
			step = float32(top.step)
		}
		// A swipe is never perfectly straight, so keep to its main axis.
		if absf(ev.Scroll.Y) >= absf(ev.Scroll.X) {
			d.wheelY, d.wheelX = d.wheelY+ev.Scroll.Y, 0
		} else {
			d.wheelX, d.wheelY = d.wheelX+ev.Scroll.X, 0
		}
		dx, dy := int(d.wheelX/step), int(d.wheelY/step)
		if dx == 0 && dy == 0 {
			return
		}
		d.wheelX -= float32(dx) * step
		d.wheelY -= float32(dy) * step
		switch {
		case top.grid:
			d.gridWheel(x, y, dx, dy)
		case top.wheel != nil:
			top.wheel(dx, dy)
		}
	}
}

// gridWheel scrolls line by line, finer than a terminal's three-line notch.
func (d *desktop) gridWheel(x, y, dx, dy int) {
	e, v := d.m.editor, &d.m.view
	v.clearDiagnosticHover()
	x, y = min(d.cols-1, max(0, x)), min(d.rows-1, max(0, y))
	for _, pv := range v.previews {
		if pv.contains(x, y) {
			if dy != 0 {
				v.previewWheel(pv.p, dy)
			}
			return
		}
	}
	doc := e
	if e.directory != nil {
		doc = e.directory.doc
	}
	for _, pane := range v.panes {
		if pane.contains(x, y) && !pane.b.dashboard {
			v.paneWheel(doc, pane, dy, dx*2)
			return
		}
	}
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
