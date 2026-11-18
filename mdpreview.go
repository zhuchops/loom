package main

import "path/filepath"

// drawPreview renders the Markdown source of p.b into box. It reads the buffer
// on every frame, so edits show up as they are typed.
func (v *renderer) drawPreview(e *editor, p *pane, box hitBox) {
	b := p.b
	v.previews = append(v.previews, previewBox{box, p})
	v.screen.FillArea(box.x, box.y, box.w, box.h, ' ', baseStyle)
	title := " Preview · " + filepath.Base(b.path)
	v.write(box.x, box.y, box.w, title, mutedStyle)
	body := hitBox{box.x, box.y + 1, box.w, box.h - 1, 0}
	width := body.w - 4
	if body.h < 1 || width < 4 {
		return
	}
	if p.md == nil || p.mdVersion != b.version || p.mdWidth != width {
		p.md, p.mdVersion, p.mdWidth = renderMarkdown(b.text, width), b.version, width
	}
	lines := p.md
	cursor := b.rowAt(b.cursor)
	if e.b != b {
		cursor = p.mdRow
	}
	if cursor != p.mdRow {
		p.mdRow, p.mdManual = cursor, false
	}
	// Keep the block under the cursor in view, a third of the way down.
	current := 0
	for i, l := range lines {
		if l.start <= cursor {
			current = i
		}
		if l.start <= cursor && cursor <= l.end {
			current = i
			break
		}
	}
	if !p.mdManual {
		p.mdTop = max(0, current-body.h/3)
	}
	p.mdTop = max(0, min(p.mdTop, max(0, len(lines)-body.h)))
	for row := 0; row < body.h && p.mdTop+row < len(lines); row++ {
		line := lines[p.mdTop+row]
		y := body.y + row
		if line.fill {
			v.screen.FillArea(body.x+2, y, body.w-4, 1, ' ', baseStyle.Background(th.menu))
		}
		if line.rule {
			v.screen.FillArea(body.x+2, y, width, 1, '─', baseStyle.Foreground(th.border))
		}
		if cursor >= line.start && cursor <= line.end && e.b == b {
			v.write(body.x, y, 1, "▎", baseStyle.Foreground(th.accent))
		}
		x, limit := body.x+2, width
		for i := 0; i < len(line.cells) && limit > 0; {
			j := i
			text := []rune{}
			for j < len(line.cells) && line.cells[j].s == line.cells[i].s {
				text = append(text, line.cells[j].r)
				j++
			}
			w := cellWidth(string(text))
			v.write(x, y, limit, string(text), line.cells[i].s)
			x, limit, i = x+w, limit-w, j
		}
	}
	if len(lines) == 0 {
		v.write(body.x+2, body.y, width, "Nothing to preview yet.", mutedStyle)
	}
}

// previewWheel scrolls a preview by hand until the cursor row changes again.
func (v *renderer) previewWheel(p *pane, lines int) {
	p.mdManual = true
	p.mdTop = max(0, p.mdTop+lines)
}
