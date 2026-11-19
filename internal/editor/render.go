package editor

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

var (
	baseStyle, mutedStyle, headingStyle, menuStyle, activeStyle cellStyle
	selectStyle                                                 terminalColor
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
	edge := menuStyle.Foreground(th.accent)
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
	// The terminal's own colors are unknown, so assume gray text on near black.
	for y := range h {
		for x := 0; x < w; {
			s, style, width := v.screen.Get(x, y)
			if style.fg&defaultFlag != 0 {
				style.fg = rgb(0x808080)
			}
			if style.bg&defaultFlag != 0 {
				style.bg = rgb(0x181818)
			}
			if s == "" {
				s = " "
			}
			if style.underline != 0 {
				style.underline = darken(style.underline)
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
	width := min(78, w-2)
	hasPreview := !m.hidePreview && (len(m.matches) != 0 || m.input != "")
	previewRows := 0
	sideBySide := hasPreview && width >= 68
	if sideBySide {
		height = h - 2
		visible = min(len(m.matches), height-3)
	} else if hasPreview && h >= 9 && width >= 14 {
		visible = min(visible, min(6, h/3), h-8)
		height = h - 2
		previewRows = height - visible - 3
	}
	if empty && height < h-2 {
		height++
	}
	box := centerBox(w, h, width, height)
	v.frame(box, m.title)
	contentWidth := box.w - 4
	if sideBySide {
		contentWidth = (box.w - 5) / 2
		edge := box.x + contentWidth + 2
		v.screen.FillArea(edge, box.y+1, 1, box.h-2, '│', mutedStyle.Background(menuStyle.bg))
		v.previewBox = hitBox{x: edge + 2, y: box.y + 1, w: box.x + box.w - edge - 4, h: box.h - 2}
	} else if previewRows > 0 {
		edge := box.y + 2 + visible
		if empty {
			edge++
		}
		v.screen.FillArea(box.x+1, edge, box.w-2, 1, '─', mutedStyle.Background(menuStyle.bg))
		v.previewBox = hitBox{x: box.x + 2, y: edge + 1, w: contentWidth, h: box.y + box.h - edge - 2}
	}
	input := m.input
	for cellWidth(input) > max(0, contentWidth-2) && input != "" {
		_, n := utf8.DecodeRuneInString(input)
		input = input[n:]
	}
	prompt := "› "
	if m.exact != nil {
		prompt = ": "
	}
	if m.confirmation {
		v.write(box.x+2, box.y+1, contentWidth, "Enter Apply   Esc Cancel", mutedStyle.Background(menuStyle.GetBackground()))
	} else {
		v.write(box.x+2, box.y+1, contentWidth, prompt+input, activeStyle)
	}
	start := max(0, m.selected-visible+1)
	v.menuBox = hitBox{box.x + 1, box.y + 2, contentWidth + 1, visible, start}
	if empty {
		v.write(box.x+2, box.y+2, contentWidth, m.empty, menuStyle)
	}
	for i := start; i < start+visible; i++ {
		o := m.matches[i]
		style := menuStyle
		if i == m.selected {
			style = activeStyle
		}
		y := box.y + 2 + i - start
		v.screen.FillArea(box.x+1, y, v.menuBox.w, 1, ' ', style)
		v.writeOption(box.x+2, y, contentWidth, o, style)
	}
	if v.previewBox.h > 0 {
		p := m.selectedPreview()
		if m.details && m.hovering {
			p.source = nil
		}
		v.drawItemPreview(p, v.previewBox, &m.previewScroll)
	}
	if m.details {
		v.write(box.x+2, box.y+box.h-1, box.w-4, " Enter Details · Esc Close ", menuStyle)
	}
	if m.confirmation {
		v.screen.HideCursor()
		return
	}
	v.screen.SetCursorStyle(cursorBar)
	v.screen.ShowCursor(min(box.x+contentWidth+1, box.x+4+cellWidth(input)), box.y+1)
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
		v.write(x+width-hintWidth, y, hintWidth, o.hint, style.Bold(false).Foreground(th.muted))
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
	keyStyle := menuStyle.Foreground(th.accent).Bold(true)
	for i, en := range entries[:shown] {
		x := box.x + 2 + (i/rows)*(cell+gap)
		y := box.y + 1 + i%rows
		v.write(x, y, keyWidth, en.key, keyStyle)
		v.write(x+keyWidth+3, y, titleWidth, en.title, menuStyle)
	}
	v.screen.HideCursor()
}

func (v *renderer) drawCompletion(c completionState, top, bottom int, signature bool) {
	w := v.screen.Width()
	x, y := v.screen.cursorX, v.screen.cursorY
	rows := min(8, len(c.items))
	below, above := bottom-y-1, y-top
	lowerGap, upperGap := 0, 0
	if signature && above == 0 {
		lowerGap = 1
		below--
	} else if signature && below == 0 {
		upperGap = 1
		above--
	}
	flip := below < rows+2 && above > below
	space := below
	if flip {
		space = above
	}
	if space < 1 {
		return
	}
	border := 1
	if space < 3 {
		border = 0
	}
	rows = min(rows, space-2*border)
	maxWidth := min(66, w)
	width := min(16, maxWidth)
	for _, item := range c.items {
		width = min(maxWidth, max(width, 4+cellWidth(item.Label)+cellWidth(item.Detail)))
		if width == maxWidth {
			break
		}
	}
	height := rows + 2*border
	box := hitBox{x: max(0, min(x-border-1, w-width)), y: y + 1 + lowerGap, w: width, h: height}
	if flip {
		box.y = y - height - upperGap
	}
	if border != 0 {
		v.frame(box, "Completion")
	}
	first := max(0, c.selected-rows+1)
	v.completeBox = hitBox{box.x + border, box.y + border, box.w - 2*border, rows, first}
	if flip {
		bottom = y - upperGap
	} else {
		top = y + 1 + lowerGap
	}
	v.drawCompletionPreview(c.items[c.selected], box, top, bottom)
	for i := range rows {
		item := c.items[first+i]
		style := menuStyle
		if first+i == c.selected {
			style = activeStyle
		}
		row := v.completeBox.y + i
		v.screen.FillArea(v.completeBox.x, row, v.completeBox.w, 1, ' ', style)
		v.write(v.completeBox.x+1, row, v.completeBox.w-2, item.Label+"  "+item.Detail, style)
	}
}

func (v *renderer) drawCompletionPreview(item completionItem, list hitBox, top, bottom int) {
	var docs string
	if json.Unmarshal(item.Documentation, &docs) != nil {
		var markup struct{ Value string }
		_ = json.Unmarshal(item.Documentation, &markup)
		docs = markup.Value
	}
	detail := strings.TrimSpace(item.Detail + "\n" + docs)
	if detail == "" {
		detail = item.InsertText
		if item.TextEdit != nil {
			detail = item.TextEdit.NewText
		}
		if detail == "" || detail == item.Label {
			return
		}
	}
	left, right := list.x, v.screen.Width()-(list.x+list.w)
	available := max(left, right)
	var box hitBox
	if available >= 22 && bottom-top >= 4 {
		width := min(48, available)
		x := list.x + list.w
		if left > right {
			x = list.x - width
		}
		height := min(3+len(wrapNotification(detail, width-4)), bottom-top)
		panel := hitBox{x: x, y: min(list.y, bottom-height), w: width, h: height}
		v.frame(panel, "")
		box = hitBox{x: x + 2, y: panel.y + 1, w: width - 4, h: panel.h - 2}
	} else if v.completeBox.w >= 30 && v.completeBox.h >= 2 {
		in := v.completeBox
		v.completeBox.w = in.w / 2
		x := in.x + v.completeBox.w
		v.screen.FillArea(x, in.y, 1, in.h, '│', mutedStyle.Background(menuStyle.bg))
		box = hitBox{x: x + 2, y: in.y, w: in.w - v.completeBox.w - 3, h: in.h}
	}
	if box.w > 0 {
		scroll := 0
		v.drawItemPreview(itemPreview{title: item.Label, message: detail}, box, &scroll)
	}
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
			width += tabWidth - width%tabWidth
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
