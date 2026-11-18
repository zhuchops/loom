package editor

import (
	"fmt"
	"strings"
)

type diagnosticHover struct {
	buffer                *buffer
	version, x, y, scroll int
	message               string
	diagnostic            string
	title                 string
	at                    int
	requestDocumentation  bool
}

func (v *renderer) clearDiagnosticHover() {
	v.diagnosticHover = diagnosticHover{}
	v.diagnosticBox = hitBox{}
}

func diagnosticHoverAllowed(e *editor) bool {
	return e.menu == nil && e.notificationDetail == nil && e.directory == nil &&
		e.panel == nil && e.diff == nil && !e.tree.popup && !e.tree.focused
}

func (v *renderer) mouseDiagnosticHover(e *editor, event pointerEvent, wheel int) bool {
	if !diagnosticHoverAllowed(e) || event.leftDown || event.button != pointerNone && wheel == 0 {
		v.clearDiagnosticHover()
		return false
	}
	if v.diagnosticBox.contains(event.x, event.y) && v.diagnosticHover.buffer != nil {
		v.diagnosticHover.scroll += wheel
		return true
	}
	previous := v.diagnosticHover
	v.clearDiagnosticHover()
	if wheel != 0 {
		return false
	}
	for _, pane := range v.panes {
		if !pane.contains(event.x, event.y) || pane.b.dashboard {
			continue
		}
		b := pane.b
		row := b.top + event.y - pane.y
		if row >= len(b.lines) {
			return false
		}
		at := cellOffset(b, row, event.x-pane.x-pane.gutter+b.left)
		var messages []string
		for _, d := range b.diagnostics {
			start, startOK := b.offset(d.Range.Start)
			end, endOK := b.offset(d.Range.End)
			if !startOK || !endOK || start > end {
				continue
			}
			if event.x < pane.x+pane.gutter {
				if max(start, b.lines[row]) >= min(max(start+1, end), b.lineEnd(row)+1) {
					continue
				}
			} else {
				column := event.x - pane.x - pane.gutter + b.left
				// cellOffset clamps beyond EOL; only the first empty cell can be marked.
				if column > cellWidth(string(b.text[b.lines[row]:b.lineEnd(row)])) || at < start || at >= max(start+1, end) {
					continue
				}
			}
			messages = append(messages, humanize(severityName(d.Severity))+": "+d.Message)
		}
		if len(messages) > 0 {
			diagnostic := strings.Join(messages, "\n\n")
			v.diagnosticHover = diagnosticHover{buffer: b, version: b.version, x: event.x, y: event.y, at: at, message: diagnostic, diagnostic: diagnostic, requestDocumentation: event.x >= pane.x+pane.gutter && at < b.lineEnd(row)}
		} else if event.x >= pane.x+pane.gutter && at < b.lineEnd(row) {
			v.diagnosticHover = diagnosticHover{buffer: b, version: b.version, x: event.x, y: event.y, at: at, title: "Hover Documentation", requestDocumentation: true}
		}
		if v.diagnosticHover.requestDocumentation && previous.requestDocumentation && previous.buffer == b && previous.version == b.version && previous.at == at && previous.diagnostic == v.diagnosticHover.diagnostic {
			v.diagnosticHover.message, v.diagnosticHover.scroll = previous.message, previous.scroll
		}
		return false
	}
	return false
}

func (v *renderer) drawDiagnosticHover(e *editor) {
	d := &v.diagnosticHover
	v.diagnosticBox = hitBox{}
	if d.buffer == nil || d.message == "" {
		return
	}
	visible := false
	for _, pane := range v.panes {
		visible = visible || pane.b == d.buffer
	}
	if !diagnosticHoverAllowed(e) || !visible || d.buffer.version != d.version || len(e.completion.items) > 0 {
		v.clearDiagnosticHover()
		return
	}
	w, h := v.screen.Size()
	if w < 8 || h < 6 {
		return
	}
	width := min(100, w-2)
	lines := wrapNotification(d.message, width-2)
	height := min(h-2, len(lines)+2)
	rows := height - 2
	overflow := len(lines) > rows
	if overflow {
		rows--
	}
	d.scroll = max(0, min(d.scroll, len(lines)-rows))
	x := max(0, min(d.x-1, w-width))
	y := d.y + 1
	if y+height > h-1 {
		y = max(1, d.y-height)
	}
	box := hitBox{x: x, y: y, w: width, h: height}
	title := d.title
	if title == "" {
		title = "Diagnostic"
	}
	v.frame(box, title)
	for i, line := range lines[d.scroll : d.scroll+min(rows, len(lines)-d.scroll)] {
		v.write(x+1, y+1+i, width-2, line, menuStyle)
	}
	if overflow {
		v.write(x+1, y+height-2, width-2, fmt.Sprintf("%d–%d/%d · Scroll for more", d.scroll+1, d.scroll+rows, len(lines)), mutedStyle.Background(menuStyle.bg))
	}
	v.diagnosticBox = box
}
