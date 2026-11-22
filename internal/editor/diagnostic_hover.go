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
	if !v.hoverValid(e) {
		v.clearDiagnosticHover()
		return
	}
	w, h := v.screen.Size()
	if w < 8 || h < 6 {
		return
	}
	width := min(100, w-2)
	lines := wrapNotification(d.message, width-2)
	// Never cover the pointer: use whichever side of it has more room, and
	// stay short enough that moving the mouse away always dismisses the box.
	below, above := h-1-(d.y+1), d.y-1
	room, onTop := below, false
	if above > below && len(lines)+2 > below {
		room, onTop = above, true
	}
	room = min(room, max(6, h/2))
	if room < 4 {
		return
	}
	height := min(len(lines)+2, room)
	rows := height - 2
	overflow := len(lines) > rows
	if overflow {
		rows--
	}
	d.scroll = max(0, min(d.scroll, len(lines)-rows))
	x := max(0, min(d.x-1, w-width))
	y := d.y + 1
	if onTop {
		y = d.y - height
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

// hoverValid reports whether the hover still describes text that is on screen.
func (v *renderer) hoverValid(e *editor) bool {
	d := &v.diagnosticHover
	if !diagnosticHoverAllowed(e) || d.buffer.version != d.version || len(e.completion.items) > 0 {
		return false
	}
	for _, pane := range v.panes {
		if pane.b == d.buffer {
			return true
		}
	}
	return false
}
