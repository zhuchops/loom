package editor

import "fmt"

type jumpPoint struct {
	buffer              *buffer
	row, col, top, left int
}
type jumpHistory struct {
	back, forward []jumpPoint
}

func (e *editor) jumpPoint() jumpPoint {
	b := e.b
	row := b.rowAt(b.cursor)
	return jumpPoint{b, row, b.cursor - b.lines[row], b.top, b.left}
}

func pushJump(stack []jumpPoint, p jumpPoint) []jumpPoint {
	if len(stack) == 100 {
		stack = stack[1:]
	}
	return append(stack, p)
}

func (e *editor) rememberJump(from jumpPoint) {
	to := e.jumpPoint()
	if from.buffer == to.buffer && from.row == to.row && from.col == to.col {
		return
	}
	e.jumps.back = pushJump(e.jumps.back, from)
	e.jumps.forward = nil
}

func (e *editor) travelJump(forward bool) {
	from, to := &e.jumps.back, &e.jumps.forward
	direction := "earlier"
	if forward {
		from, to = to, from
		direction = "later"
	}
	if len(*from) == 0 {
		e.notify(fmt.Sprintf("No %s jump", direction))
		return
	}
	target := (*from)[len(*from)-1]
	current := e.jumpPoint()
	available := false
	for _, b := range e.buffers {
		if b == target.buffer {
			e.switchBuffer(b)
			available = true
			break
		}
	}
	if !available && (target.buffer.path == "" || !e.open(target.buffer.path)) {
		e.notify("Jump target is unavailable")
		return
	}
	*from = (*from)[:len(*from)-1]
	*to = pushJump(*to, current)
	b := e.b
	row := min(target.row, len(b.lines)-1)
	b.cursor = min(b.lines[row]+target.col, b.lineEnd(row))
	b.top, b.left = min(target.top, len(b.lines)-1), target.left
	e.clampModalCursor()
	e.message = ""
}
