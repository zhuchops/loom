package editor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

type itemPreview struct {
	title, message string
	source         *buffer
	row            int // Focus row within the source.
	column         int // Rune column within the focus row.
}

func (e *editor) filePreview(path string, row, col int, utf16 bool, problem *diagnostic) func() itemPreview {
	return func() itemPreview {
		for _, b := range e.buffers {
			if b.path == path {
				return bufferPreview(b, path, row, col, utf16, problem)
			}
		}
		p := itemPreview{title: filepath.Base(path)}
		if problem != nil {
			p.title, p.message = problemPreviewText(path, *problem)
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			p.message = strings.TrimSpace(p.message + "\nSource unavailable")
			return p
		}
		f, err := os.Open(path)
		if err != nil {
			p.message = strings.TrimSpace(p.message + "\nSource unavailable: " + err.Error())
			return p
		}
		defer f.Close()
		info, err = f.Stat()
		if err != nil || !info.Mode().IsRegular() {
			p.message = strings.TrimSpace(p.message + "\nSource unavailable")
			return p
		}
		text, err := io.ReadAll(io.LimitReader(f, (4<<20)+1))
		if err != nil {
			p.message = strings.TrimSpace(p.message + "\nSource unavailable: " + err.Error())
			return p
		}
		if len(text) > 4<<20 {
			end := bytes.LastIndexByte(text[:4<<20], '\n')
			if end < 0 {
				p.message = strings.TrimSpace(p.message + "\nSource line unavailable in preview")
				return p
			}
			text = text[:end]
		}
		if !utf8.Valid(text) || bytes.IndexByte(text, 0) >= 0 {
			p.message = strings.TrimSpace(p.message + "\nPreview is available for UTF-8 text")
			return p
		}
		b := &buffer{text: []rune(string(text))}
		b.indexLines()
		b.path = path
		target := max(0, row)
		if target >= len(b.lines) {
			p.message = strings.TrimSpace(p.message + "\nSource line unavailable in preview")
			return p
		}
		return buildSourcePreview(p, b, target, col, utf16, problem)
	}
}

func problemPreviewText(path string, problem diagnostic) (string, string) {
	pos := problem.Range.Start
	title := fmt.Sprintf("%s · %s:%d:%d", humanize(severityName(problem.Severity)), filepath.Base(path), pos.Line+1, pos.Character+1)
	return title, problem.Message
}

func bufferPreview(b *buffer, title string, row, col int, utf16 bool, problem *diagnostic) itemPreview {
	p := itemPreview{title: title}
	if problem != nil {
		p.title, p.message = problemPreviewText(title, *problem)
	}
	if row < 0 {
		row = b.rowAt(b.cursor)
		col = b.cursor - b.lines[row]
	}
	row = min(row, len(b.lines)-1)
	return buildSourcePreview(p, b, row, col, utf16, problem)
}

func buildSourcePreview(p itemPreview, b *buffer, row, col int, utf16 bool, problem *diagnostic) itemPreview {
	// A preview retains the text and line offsets from when it was selected.
	snapshot := &buffer{path: b.path, text: b.text, lines: append([]int(nil), b.lines...)}
	p.source, p.row = snapshot, row
	p.column = min(max(0, col), b.lineEnd(p.row)-b.lines[p.row])
	if utf16 {
		if at, ok := b.offset(position{Line: p.row, Character: col}); ok {
			p.column = at - b.lines[p.row]
		}
	}
	if problem != nil {
		d := *problem
		if d.Range.End.Line >= len(b.lines) {
			d.Range.End = b.lspPosition(len(b.text))
		}
		snapshot.diagnostics = []diagnostic{d}
	}
	return p
}

func directoryPreview(path string) itemPreview {
	p := itemPreview{title: path}
	f, err := os.Open(path)
	if err != nil {
		p.message = err.Error()
		return p
	}
	defer f.Close()
	entries, err := f.ReadDir(512)
	if err != nil && err != io.EOF {
		p.message = err.Error()
		return p
	}
	var lines []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}
	p.message = strings.Join(lines, "\n")
	if len(lines) == 0 {
		p.message = "Empty directory"
	}
	return p
}

func (v *renderer) drawItemPreview(p itemPreview, box hitBox, scroll *int) {
	if box.w < 2 || box.h < 1 {
		return
	}
	title := "Preview"
	if p.title != "" {
		title += " · " + p.title
	}
	v.write(box.x, box.y, box.w, title, headingStyle.Background(menuStyle.bg))
	rows := box.h - 1
	x, y := box.x, box.y+1
	var message []string
	if p.message != "" {
		message = wrapNotification(p.message, box.w)
	}
	if p.source == nil {
		*scroll = max(0, min(*scroll, max(0, len(message)-rows)))
		for i, line := range message[*scroll:min(len(message), *scroll+rows)] {
			v.write(x, y+i, box.w, line, menuStyle)
		}
		return
	}
	// Show the whole diagnostic when it fits, keeping at least one source row.
	messageRows := min(len(message), max(0, rows-1))
	messageScroll := max(0, len(message)-messageRows)
	sourceRows := rows - messageRows
	maxStart := max(0, len(p.source.lines)-sourceRows)
	start := min(maxStart, max(0, p.row-sourceRows/2))
	*scroll = max(-start, min(*scroll, messageScroll+maxStart-start))
	messageStart := min(max(0, *scroll), messageScroll)
	for i, line := range message[messageStart : messageStart+messageRows] {
		v.write(x, y+i, box.w, line, menuStyle)
	}
	start += *scroll - messageStart
	v.drawSourcePreview(p, hitBox{x: x, y: y + messageRows, w: box.w, h: sourceRows}, start)
}

func (v *renderer) drawSourcePreview(p itemPreview, box hitBox, first int) {
	b := p.source
	last := min(len(b.lines), first+box.h)
	marks := diagnosticRows(b, first, last)
	numberWidth := len(fmt.Sprint(last)) + 2
	width := box.w - numberWidth
	if width < 1 {
		return
	}
	column := cellWidth(string(b.text[b.lines[p.row] : b.lines[p.row]+p.column]))
	left := max(0, column-width/2)
	if column < width {
		left = 0
	}
	for row := first; row < last; row++ {
		y := box.y + row - first
		s := menuStyle
		if row == p.row {
			s = activeStyle
			v.screen.FillArea(box.x, y, box.w, 1, ' ', s)
		}
		v.write(box.x, y, numberWidth, fmt.Sprintf("%*d", numberWidth-2, row+1), s)
		offset, column := b.lines[row], 0
		end := b.lineEnd(row)
		g := uniseg.NewGraphemes(string(b.text[offset:end]))
		for g.Next() && column < left+width {
			text, size := g.Str(), max(1, g.Width())
			if text == "\t" {
				size = tabWidth - column%tabWidth
				text = strings.Repeat(" ", size)
			}
			next := offset + utf8.RuneCountInString(g.Str())
			if column >= left && column+size <= left+width {
				v.write(box.x+numberWidth+column-left, y, size, text, markDiagnostic(s, marks[row], offset, next))
			}
			offset, column = next, column+size
		}
		if offset == end && column >= left && column < left+width {
			v.write(box.x+numberWidth+column-left, y, 1, " ", markDiagnostic(s, marks[row], end, end+1))
		}
	}
}
