package main

import (
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

var (
	baseStyle    = cellStyle{}.Background(rgb(0x2d353b)).Foreground(rgb(0xd3c6aa))
	mutedStyle   = baseStyle.Foreground(rgb(0x89968b))
	headingStyle = baseStyle.Foreground(rgb(0xa7c080)).Bold(true)
	menuStyle    = baseStyle.Background(rgb(0x343f44)).Foreground(rgb(0xd3c6aa))
	activeStyle  = menuStyle.Background(rgb(0x4b5558)).Bold(true)
	selectStyle  = rgb(0x543a48)
)

type highlight struct {
	span
	style cellStyle
}

func centerBox(w, h, width, height int) hitBox {
	return hitBox{(w - width) / 2, (h - height) / 2, width, height, 0}
}

func (v *renderer) frame(box hitBox, title string) {
	v.screen.FillArea(box.x, box.y, box.w, box.h, ' ', menuStyle)
	edge := menuStyle.Foreground(rgb(0xa7c080))
	v.screen.FillArea(box.x, box.y, box.w, 1, '─', edge)
	v.screen.FillArea(box.x, box.y+box.h-1, box.w, 1, '─', edge)
	v.screen.FillArea(box.x, box.y, 1, box.h, '│', edge)
	v.screen.FillArea(box.x+box.w-1, box.y, 1, box.h, '│', edge)
	v.screen.Put(box.x, box.y, "╭", 1, edge)
	v.screen.Put(box.x+box.w-1, box.y, "╮", 1, edge)
	v.screen.Put(box.x, box.y+box.h-1, "╰", 1, edge)
	v.screen.Put(box.x+box.w-1, box.y+box.h-1, "╯", 1, edge)
	if title != "" {
		v.write(box.x+2, box.y, box.w-4, " "+title+" ", edge.Bold(true))
	}
}

func (v *renderer) dim(w, h int) {
	darken := func(c terminalColor) terminalColor {
		r, g, b := c.RGB()
		return rgb(r*55/100<<16 | g*55/100<<8 | b*55/100)
	}
	for y := range h {
		for x := 0; x < w; {
			s, style, width := v.screen.Get(x, y)
			if s == "" {
				s = " "
			}
			v.screen.Put(x, y, s, max(1, width), style.Foreground(darken(style.fg)).Background(darken(style.bg)))
			x += max(1, width)
		}
	}
}

func (v *renderer) drawMenu(m *menu, w, h int) {
	visible := min(12, max(0, h-5), len(m.matches))
	empty := visible == 0 && m.empty != ""
	height := visible + 3
	if empty {
		height++
	}
	box := centerBox(w, h, min(78, w-2), height)
	v.frame(box, m.title)
	input := m.input
	for cellWidth(input) > max(0, box.w-6) && input != "" {
		_, n := utf8.DecodeRuneInString(input)
		input = input[n:]
	}
	prompt := "› "
	if m.exact != nil {
		prompt = ": "
	}
	if m.confirmation {
		v.write(box.x+2, box.y+1, box.w-4, "Enter Apply   Esc Cancel", mutedStyle.Background(menuStyle.GetBackground()))
	} else {
		v.write(box.x+2, box.y+1, box.w-4, prompt+input, activeStyle)
	}
	start := max(0, m.selected-visible+1)
	v.menuBox = hitBox{box.x + 1, box.y + 2, box.w - 2, visible, start}
	if empty {
		v.write(box.x+2, box.y+2, box.w-4, m.empty, menuStyle)
	}
	for i := start; i < start+visible; i++ {
		o := m.matches[i]
		style := menuStyle
		if i == m.selected {
			style = activeStyle
		}
		y := box.y + 2 + i - start
		v.screen.FillArea(box.x+1, y, box.w-2, 1, ' ', style)
		v.writeOption(box.x+2, y, box.w-4, o, style)
	}
	if m.confirmation {
		v.screen.HideCursor()
		return
	}
	v.screen.SetCursorStyle(cursorBar)
	v.screen.ShowCursor(min(box.x+box.w-2, box.x+4+cellWidth(input)), box.y+1)
}

// writeOption draws the label on the left and the hint right-aligned in a muted color.
func (v *renderer) writeOption(x, y, width int, o option, style cellStyle) {
	hintWidth := 0
	if o.hint != "" {
		hintWidth = min(cellWidth(o.hint), width/2)
	}
	labelWidth := width
	if hintWidth > 0 {
		labelWidth = width - hintWidth - 2
	}
	v.write(x, y, labelWidth, o.label, style)
	if hintWidth > 0 {
		v.write(x+width-hintWidth, y, hintWidth, o.hint, style.Bold(false).Foreground(rgb(0x89968b)))
	}
}

var chordNames = map[string]string{
	"space": "Leader", "g": "Go", "ctrl+w": "Window",
	"[": "Previous", "]": "Next", "m": "Surround", "s": "Surround",
}

// keyLabel shows a stored key sequence the way a person reads it.
func keyLabel(sequence string) string {
	parts := strings.Fields(sequence)
	for i, part := range parts {
		switch part {
		case "left":
			parts[i] = "←"
		case "right":
			parts[i] = "→"
		case "up":
			parts[i] = "↑"
		case "down":
			parts[i] = "↓"
		default:
			parts[i] = formatKey(part)
		}
	}
	return strings.Join(parts, " ")
}

// humanize turns a command ID such as "split-below" into "Split below".
func humanize(id string) string {
	text := strings.Join(strings.Fields(strings.ReplaceAll(id, "-", " ")), " ")
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

func chordTitle(prefix string) string {
	if name, ok := chordNames[prefix]; ok {
		return keyLabel(prefix) + " · " + name
	}
	return keyLabel(prefix) + " …"
}

func (v *renderer) drawWhichKey(e *editor, w, h int) {
	keys := e.keys[e.cfg.Profile+"."+e.mode]
	prefix := e.prefix + " "
	type entry struct{ key, title string }
	var entries []entry
	for _, seq := range sortedKeys(keys) {
		suffix, ok := strings.CutPrefix(seq, prefix)
		if !ok {
			continue
		}
		id := keys[seq]
		title := e.commands[id].title
		if title == "" {
			title = humanize(id)
		}
		entries = append(entries, entry{keyLabel(suffix), title})
	}
	if len(entries) == 0 {
		return
	}
	keyWidth, titleWidth := 0, 0
	for _, en := range entries {
		keyWidth = max(keyWidth, cellWidth(en.key))
		titleWidth = max(titleWidth, cellWidth(en.title))
	}
	const gap = 4
	cell := keyWidth + 3 + titleWidth
	maxRows := max(1, h-4)
	columns, rows := 1, min(len(entries), maxRows)
	for c := 3; c >= 1; c-- {
		r := (len(entries) + c - 1) / c
		if r <= maxRows && c*cell+(c-1)*gap+4 <= w-4 {
			columns, rows = c, r
			break
		}
	}
	shown := min(len(entries), rows*columns)
	width := min(w-4, columns*cell+(columns-1)*gap+4)
	box := centerBox(w, h, width, rows+2)
	v.frame(box, chordTitle(e.prefix))
	keyStyle := menuStyle.Foreground(rgb(0xa7c080)).Bold(true)
	for i, en := range entries[:shown] {
		x := box.x + 2 + (i/rows)*(cell+gap)
		y := box.y + 1 + i%rows
		v.write(x, y, keyWidth, en.key, keyStyle)
		v.write(x+keyWidth+3, y, titleWidth, en.title, menuStyle)
	}
	v.screen.HideCursor()
}

func (v *renderer) drawCompletion(c completionState, w, h int) {
	rows := min(8, len(c.items), max(1, h-4))
	box := centerBox(w, h, min(66, w-2), rows+2)
	v.frame(box, "Completion")
	first := max(0, c.selected-rows+1)
	v.completeBox = hitBox{box.x + 1, box.y + 1, box.w - 2, rows, first}
	for i := range rows {
		item := c.items[first+i]
		style := menuStyle
		if first+i == c.selected {
			style = activeStyle
		}
		v.screen.FillArea(box.x+1, box.y+i+1, box.w-2, 1, ' ', style)
		v.write(box.x+2, box.y+i+1, box.w-4, item.Label+"  "+item.Detail, style)
	}
	v.screen.HideCursor()
}

func (v *renderer) write(x, y, limit int, s string, style cellStyle) {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	for s != "" && limit > 0 {
		g, rest, width, _ := uniseg.FirstGraphemeClusterInString(s, -1)
		if width > limit {
			break
		}
		if width > 0 {
			v.screen.Put(x, y, g, width, style)
		}
		x += width
		limit -= width
		s = rest
	}
}

func cellWidth(s string) int {
	g := uniseg.NewGraphemes(s)
	width := 0
	for g.Next() {
		if g.Str() == "\t" {
			width += 4 - width%4
		} else {
			width += max(1, g.Width())
		}
	}
	return width
}

func (e *editor) closeServers() {
	var group sync.WaitGroup
	for _, s := range e.sessions {
		group.Go(s.close)
	}
	done := make(chan struct{})
	go func() { group.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(450 * time.Millisecond):
	}
}
