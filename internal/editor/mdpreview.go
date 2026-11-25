package editor

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/zhuchops/loom/internal/markdown"
)

// markdownStyle maps a role from the renderer to a themed cell style.
func markdownStyle(s markdown.Style) cellStyle {
	var style cellStyle
	switch s.Role {
	case markdown.Muted:
		style = baseStyle.Foreground(th.muted)
	case markdown.H1:
		style = headingStyle
	case markdown.H2:
		style = baseStyle.Foreground(th.yellow).Bold(true)
	case markdown.H3:
		style = baseStyle.Foreground(th.aqua).Bold(true)
	case markdown.H4:
		style = baseStyle.Bold(true)
	case markdown.Accent:
		style = baseStyle.Foreground(th.accent)
	case markdown.Border:
		style = baseStyle.Foreground(th.border)
	case markdown.Emphasis:
		style = baseStyle.Foreground(th.typ)
	case markdown.Link:
		style = baseStyle.Foreground(th.aqua)
	case markdown.InlineCode:
		return baseStyle.Background(th.menu).Foreground(th.orange)
	case markdown.CodeText:
		style = baseStyle.Background(th.menu)
		if hex := semanticColor(s.Kind); hex != 0 {
			style = style.Foreground(rgb(hex))
		}
		return style
	case markdown.CodeLabel:
		return baseStyle.Background(th.menu).Foreground(th.muted)
	default:
		style = baseStyle
	}
	if s.Bold {
		style = style.Bold(true)
	}
	return style
}

// drawPreview renders the Markdown source of p.b into box. It reads the buffer
// on every frame, so edits show up as they are typed.
func (v *renderer) drawPreview(e *editor, p *pane, box hitBox) {
	b := p.b
	v.previews = append(v.previews, previewBox{box, p})
	v.screen.FillArea(box.x, box.y, box.w, box.h, ' ', baseStyle)
	if v.native {
		return // the desktop app lays the preview out as a page
	}
	title := " Preview · " + filepath.Base(b.path)
	v.write(box.x, box.y, box.w, title, mutedStyle)
	body := hitBox{box.x, box.y + 1, box.w, box.h - 1, 0}
	width := body.w - 4
	if body.h < 1 || width < 4 {
		return
	}
	if p.md == nil || p.mdVersion != b.version || p.mdWidth != width {
		p.md, p.mdVersion, p.mdWidth = markdown.Render(b.text, width), b.version, width
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
		if l.Start <= cursor {
			current = i
		}
		if l.Start <= cursor && cursor <= l.End {
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
		if line.Fill {
			v.screen.FillArea(body.x+2, y, body.w-4, 1, ' ', baseStyle.Background(th.menu))
		}
		if line.Rule {
			v.screen.FillArea(body.x+2, y, width, 1, '─', baseStyle.Foreground(th.border))
		}
		if cursor >= line.Start && cursor <= line.End && e.b == b {
			v.write(body.x, y, 1, "▎", baseStyle.Foreground(th.accent))
		}
		x, limit := body.x+2, width
		for i := 0; i < len(line.Cells) && limit > 0; {
			j := i
			text := []rune{}
			for j < len(line.Cells) && line.Cells[j].S == line.Cells[i].S {
				text = append(text, line.Cells[j].R)
				j++
			}
			w := cellWidth(string(text))
			v.write(x, y, limit, string(text), markdownStyle(line.Cells[i].S))
			x, limit, i = x+w, limit-w, j
		}
	}
	if len(lines) == 0 {
		v.write(body.x+2, body.y, width, "Nothing to preview yet.", mutedStyle)
	}
}

// previewHTML is the preview of p as HTML, rebuilt when the buffer changes.
func (p *pane) previewHTML() string {
	if !p.mdHTMLSet || p.mdHTMLVersion != p.b.version {
		p.mdHTML, p.mdHTMLVersion, p.mdHTMLSet = markdown.HTML(p.b.text), p.b.version, true
	}
	return p.mdHTML
}

// previewWheel scrolls a preview by hand until the cursor row changes again.
func (v *renderer) previewWheel(p *pane, lines int) {
	p.mdManual = true
	p.mdTop = max(0, p.mdTop+lines)
}

// followLink opens what a link in preview i points to: a web page in the
// browser, or a file next to the Markdown file in the editor.
func (b *Bridge) followLink(i int, target string) {
	e, v := b.m.editor, &b.m.view
	if i < 0 || i >= len(v.previews) || target == "" {
		return
	}
	lower := strings.ToLower(target)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:") {
		if h, ok := b.host.(interface{ openURL(string) }); ok {
			h.openURL(target)
		} else {
			e.notify("Can't open links here: " + target)
		}
		return
	}
	if strings.Contains(target, "://") {
		return // no other schemes
	}
	path, _, _ := strings.Cut(target, "#")
	if p, err := url.PathUnescape(path); err == nil {
		path = p
	}
	if path == "" {
		return
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(v.previews[i].p.b.path), path)
	}
	if info, err := os.Stat(path); err != nil {
		e.notify("No such file: " + shortPath(path))
	} else if info.IsDir() {
		e.notify(shortPath(path) + " is a folder")
	} else {
		e.open(path)
	}
}
