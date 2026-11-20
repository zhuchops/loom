package editor

import "time"

const animDuration = 160 * time.Millisecond

// animationsOn lets tests turn motion off so geometry is exact.
var animationsOn = true

// paneAnim eases a split's ratio from one value to another.
type paneAnim struct {
	from, to int
	start    time.Time
}

func (a paneAnim) active(now time.Time) bool {
	return animationsOn && !a.start.IsZero() && now.Sub(a.start) < animDuration
}

// shownRatio is the ratio to lay out with right now.
func (p *pane) shownRatio() int {
	now := time.Now()
	if !p.anim.active(now) {
		return p.ratio
	}
	t := float64(now.Sub(p.anim.start)) / float64(animDuration)
	t = 1 - (1-t)*(1-t)*(1-t) // ease out
	return p.anim.from + int(float64(p.anim.to-p.anim.from)*t)
}

// setRatio changes a split, gliding to the new size when animate is set.
func (p *pane) setRatio(to int, animate bool) {
	from := p.shownRatio()
	if from <= 0 {
		from = 500
	}
	p.ratio = to
	if animate && animationsOn {
		target := to
		if target <= 0 {
			target = 500
		}
		p.anim = paneAnim{from: from, to: target, start: time.Now()}
		return
	}
	p.anim = paneAnim{}
}

// animating reports whether any split is still moving, so the screen keeps redrawing.
func (e *editor) animating() bool {
	now := time.Now()
	var walk func(*pane) bool
	walk = func(p *pane) bool {
		if p == nil || p.b != nil {
			return false
		}
		return p.anim.active(now) || walk(p.first) || walk(p.second)
	}
	return walk(e.layout)
}
