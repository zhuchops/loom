package editor

import (
	"fmt"
	"strings"
	"time"

	"github.com/rivo/uniseg"
)

const notificationLimit = 200

type notification struct {
	text string
	at   time.Time
}

// The editor event loop owns the status line and history. Keep the original
// text here because the status line and picker can only show a short preview.
func (e *editor) notify(message string) {
	if e.notificationParent != nil {
		e.notificationParent.notify(message)
		return
	}
	e.message = message
	if message == "" {
		return
	}
	if len(e.notifications) == notificationLimit {
		copy(e.notifications, e.notifications[1:])
		e.notifications = e.notifications[:notificationLimit-1]
	}
	e.notifications = append(e.notifications, notification{message, time.Now()})
}

func (e *editor) openNotifications() {
	m := &menu{title: "Notification History", empty: "No notifications yet", details: true}
	if len(e.notifications) > 0 {
		m.empty = "No matching notifications"
	}
	for i := len(e.notifications) - 1; i >= 0; i-- {
		n := e.notifications[i]
		preview := strings.Join(strings.Fields(n.text), " ")
		m.options = append(m.options, option{preview, n.at.Format("15:04:05"), func() {
			e.showDetails(m, "Notification · "+n.at.Format("15:04:05"), n.text, nil)
		}, func() itemPreview { return itemPreview{title: n.at.Format("15:04:05"), message: n.text} }})
	}
	e.openMenu(m)
}

type notificationDetail struct {
	notification
	title               string
	jump                func()
	lines               []string
	width, rows, scroll int
}

func (e *editor) showDetails(parent *menu, title, text string, jump func()) {
	e.menu = parent
	e.notificationDetail = &notificationDetail{notification: notification{text: text}, title: title, jump: jump}
}

func (d *notificationDetail) move(delta int) {
	d.scroll = max(0, min(d.scroll+delta, len(d.lines)-d.rows))
}

func (e *editor) notificationInput(key string) {
	d := e.notificationDetail
	switch key {
	case "esc", "backspace":
		e.notificationDetail = nil
	case "o":
		if d.jump != nil {
			e.notificationDetail, e.menu = nil, nil
			d.jump()
		}
	case "down", "j":
		d.move(1)
	case "up", "k":
		d.move(-1)
	case "pagedown", "ctrl+d", "space":
		d.move(d.rows)
	case "pageup", "ctrl+u":
		d.move(-d.rows)
	case "home", "g":
		d.scroll = 0
	case "end", "G":
		d.move(len(d.lines))
	}
}

func (v *renderer) drawNotification(d *notificationDetail, w, h int) {
	box := centerBox(w, h, min(100, w), h-1)
	width := box.w - 2
	if d.width != width {
		d.width = width
		d.lines = wrapNotification(d.text, width)
	}
	d.rows = box.h - 3
	d.move(0)
	title := d.title
	if title == "" {
		title = "Notification · " + d.at.Format("15:04:05")
	}
	v.frame(box, title)
	end := min(len(d.lines), d.scroll+d.rows)
	for i, line := range d.lines[d.scroll:end] {
		v.write(box.x+1, box.y+1+i, width, line, menuStyle)
	}
	footer := fmt.Sprintf("%d–%d/%d  ↑/↓ Scroll  Esc Back", d.scroll+1, end, len(d.lines))
	if d.jump != nil {
		footer = fmt.Sprintf("%d–%d/%d  o Source  Esc Back", d.scroll+1, end, len(d.lines))
	}
	v.write(box.x+1, box.y+box.h-2, width, footer, menuStyle)
	v.screen.HideCursor()
}

// Wrap by terminal cells and keep explicit line breaks, including blank lines.
func wrapNotification(message string, width int) []string {
	message = strings.ReplaceAll(message, "\r\n", "\n")
	message = strings.ReplaceAll(message, "\t", "    ")
	var lines []string
	for _, line := range strings.Split(message, "\n") {
		g := uniseg.NewGraphemes(line)
		start, cells := 0, 0
		for g.Next() {
			at, _ := g.Positions()
			size := max(1, g.Width())
			if cells > 0 && cells+size > width {
				lines = append(lines, line[start:at])
				start, cells = at, 0
			}
			cells += size
		}
		lines = append(lines, line[start:])
	}
	return lines
}
