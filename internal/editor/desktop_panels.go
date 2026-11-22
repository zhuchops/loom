//go:build desktop

package editor

import (
	"fmt"
	"image"
	"path/filepath"
	"strconv"
	"strings"
)

func (d *desktop) centred(w, h int) rect {
	g := &d.geo
	top, bottom := g.title.Max.Y, g.window.Max.Y
	if !g.status.Empty() {
		bottom = g.status.Min.Y
	}
	w, h = min(w, g.window.Dx()-d.u(32)), min(h, bottom-top-d.u(24))
	x, y := (g.window.Dx()-w)/2, top+(bottom-top-h)/2
	return image.Rect(x, y, x+w, y+h)
}

// textButton draws a button ending at right and returns where it starts.
func (d *desktop) textButton(right, cy int, label string, primary bool, click func()) int {
	u, c := d.u, &d.c
	r := image.Rect(right-d.width(label, textStrong)-u(22), cy-u(13), right, cy+u(13))
	hot := d.button(r, click)
	ink := c.text
	switch {
	case primary:
		fill := c.accent
		if hot {
			fill = blend(fill, white, 0.14)
		}
		d.round(r, 6, fill)
		ink = c.onAccent
	default:
		if hot {
			d.round(r, 6, c.hover)
		}
		d.border(r, 6, c.strongLine, d.hairline())
	}
	d.textCenter((r.Min.X+r.Max.X)/2, cy, label, textStrong, ink)
	return r.Min.X
}

func (d *desktop) paintGit(e *editor) {
	p, u, c := e.panel, d.u, &d.c
	d.scrim(func() { e.panel = nil })
	rows := p.entries()
	rowH, headH, footH := u(30), u(56), u(38)
	body := max(u(180), min(u(440), len(rows)*rowH+u(16)))
	card := d.centred(u(700), headH+body+footH)
	d.card(card)
	d.block(card)

	cy := card.Min.Y + headH/2
	x := card.Min.X + u(18)
	x += d.text(x, cy, "Source Control", textTitle, c.text) + u(14)
	right := card.Max.X - u(12)
	if p.top != "" {
		right = d.textButton(right, cy, "Commit", p.hasStaged(), func() { e.commitPrompt(p) }) - u(6)
		right = d.textButton(right, cy, "Push", false, func() { e.gitRemote(p.top, "push") }) - u(6)
		right = d.textButton(right, cy, "Pull", false, func() { e.gitRemote(p.top, "pull", "--ff-only") }) - u(6)
	}
	if p.branch != "" && right-x > u(60) {
		name := d.fit(p.branch, right-x-u(44), textBody)
		box := image.Rect(x, cy-u(12), x+d.width(name, textBody)+u(34), cy+u(12))
		if d.button(box, e.openBranchPicker) {
			d.round(box, 6, c.hover)
		}
		d.icon(iconBranch, box.Min.X+u(13), cy, 13, c.dim)
		d.text(box.Min.X+u(24), cy, name, textBody, c.dim)
	}
	d.lineH(card.Min.X, card.Max.X, card.Min.Y+headH)

	list := image.Rect(card.Min.X, card.Min.Y+headH+u(8), card.Max.X, card.Max.Y-footH-u(4))
	mx, my := (list.Min.X+list.Max.X)/2, (list.Min.Y+list.Max.Y)/2
	switch {
	case p.top == "":
		d.textCenter(mx, my, "Reading repository…", textBody, c.dim)
	case len(rows) == 0:
		d.icon(iconCheck, mx, my-u(18), 24, c.green)
		d.textCenter(mx, my+u(12), "No changes. The working tree is clean.", textBody, c.dim)
	}
	count := max(1, list.Dy()/rowH)
	p.scroll = max(0, min(p.scroll, len(rows)-count))
	if p.selected < p.scroll {
		p.scroll = p.selected
		if p.scroll == 1 {
			p.scroll = 0 // keep the first heading in sight
		}
	}
	if p.selected >= p.scroll+count {
		p.scroll = p.selected - count + 1
	}
	wheel := func(_, dy int) {
		for range absInt(dy) {
			p.move(max(-1, min(1, dy)))
		}
	}
	d.zone(zone{r: list, wheel: wheel, step: rowH})

	for i := p.scroll; i < len(rows) && i < p.scroll+count; i++ {
		row := rows[i]
		y := list.Min.Y + (i-p.scroll)*rowH
		mid := y + rowH/2
		if row.header != "" {
			d.text(card.Min.X+u(18), mid+u(2), strings.ToUpper(row.header), textCaption, c.faint)
			continue
		}
		line := image.Rect(card.Min.X+u(8), y+d.hairline(), card.Max.X-u(8), y+rowH-d.hairline())
		d.zone(zone{r: line, wheel: wheel, step: rowH, click: func() {
			p.selected = i
			e.openDiffFor(row.change.path)
		}})
		hot := d.hot(line)
		switch {
		case i == p.selected:
			d.round(line, 6, c.selected)
		case hot:
			d.round(line, 6, c.hover)
		}
		d.fileIcon(line.Min.X+u(18), mid, row.change.rel)
		end := line.Max.X - u(10)
		mark := row.change.work
		if row.staged {
			mark = row.change.index
		}
		if row.change.index == '?' {
			mark = '?'
		}
		label := string(mark)
		if mark == '?' {
			label = "U"
		}
		d.textRight(end, mid, label, textCaption, readable(d.gitInk(mark), c.raised, 3.5))
		end -= u(22)
		if hot || i == p.selected {
			id := iconPlus
			if row.staged {
				id = iconMinus
			}
			d.iconButton(end-u(10), mid, id, false, func() {
				p.selected = i
				e.sourceControlInput(keyInput{key: "s"})
			})
		}
		end -= u(30)
		nx := line.Min.X + u(34)
		nx += d.text(nx, mid, d.fit(filepath.Base(row.change.rel), end-nx, textBody), textBody, c.text) + u(8)
		if dir := filepath.Dir(row.change.rel); dir != "." {
			d.text(nx, mid, d.fitLeft(dir, max(0, end-nx), textSmall), textSmall, c.faint)
		}
	}
	d.scrollbar(list, p.scroll, count, len(rows))

	foot := card.Max.Y - footH
	d.lineH(card.Min.X, card.Max.X, foot)
	pairs := []string{"↵", "diff", "s", "stage", "a", "stage all", "c", "commit", "d", "discard", "esc", "close"}
	if card.Dx() < u(600) {
		pairs = []string{"↵", "diff", "s", "stage", "esc", "close"}
	}
	d.hints(card.Min.X+u(14), foot+footH/2, false, pairs...)
}

// diffNumbers gives the old and new line number of every row of a diff.
func diffNumbers(lines []string) [][2]int {
	out := make([][2]int, len(lines))
	before, after := 0, 0
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "@@"):
			if f := strings.Fields(line); len(f) >= 3 {
				first, _, _ := strings.Cut(strings.TrimPrefix(f[1], "-"), ",")
				second, _, _ := strings.Cut(strings.TrimPrefix(f[2], "+"), ",")
				before, _ = strconv.Atoi(first)
				after, _ = strconv.Atoi(second)
			}
		case before == 0 && after == 0, strings.HasPrefix(line, `\`): // no hunk yet, or "No newline at end of file"
		case strings.HasPrefix(line, "+"):
			out[i][1] = after
			after++
		case strings.HasPrefix(line, "-"):
			out[i][0] = before
			before++
		default:
			out[i] = [2]int{before, after}
			before++
			after++
		}
	}
	return out
}

func (d *desktop) paintDiff(e *editor) {
	df, u, c := e.diff, d.u, &d.c
	d.scrim(func() { e.diff = nil })
	card := d.centred(u(980), u(620))
	d.card(card)
	d.block(card)
	headH, footH, lineH := u(46), u(38), u(21)

	cy := card.Min.Y + headH/2
	title := strings.TrimPrefix(df.title, "Diff · ")
	d.fileIcon(card.Min.X+u(24), cy, title)
	added, removed := 0, 0
	numbers := diffNumbers(df.lines)
	for _, n := range numbers {
		switch {
		case n[0] == 0 && n[1] > 0:
			added++
		case n[1] == 0 && n[0] > 0:
			removed++
		}
	}
	d.iconButton(card.Max.X-u(24), cy, iconClose, false, func() { e.diff = nil })
	right := card.Max.X - u(48)
	right -= d.textRight(right, cy, fmt.Sprintf("−%d", removed), textStrong, readable(c.red, c.raised, 4)) + u(10)
	right -= d.textRight(right, cy, fmt.Sprintf("+%d", added), textStrong, readable(c.green, c.raised, 4)) + u(16)
	x := card.Min.X + u(40)
	d.text(x, cy, d.fitLeft(title, right-x, textTitle), textTitle, c.text)

	body := image.Rect(card.Min.X+d.hairline(), card.Min.Y+headH, card.Max.X-d.hairline(), card.Max.Y-footH)
	d.fill(body, c.bg)
	d.lineH(card.Min.X, card.Max.X, body.Min.Y)
	count := max(1, (body.Dy()-u(6))/lineH)
	df.height = count
	df.cursor = max(0, min(df.cursor, len(df.lines)-1))
	if df.cursor < df.scroll {
		df.scroll = df.cursor
	}
	if df.cursor >= df.scroll+count {
		df.scroll = df.cursor - count + 1
	}
	wheel := func(_, dy int) {
		last := max(0, len(df.lines)-count)
		df.scroll = max(0, min(last, df.scroll+dy))
		df.cursor = max(df.scroll, min(df.cursor, df.scroll+count-1))
	}
	d.zone(zone{r: body, wheel: wheel, step: lineH})

	d.clipped(body, func() {
		col1, col2 := body.Min.X+u(42), body.Min.X+u(80)
		for i := df.scroll; i < len(df.lines) && i < df.scroll+count; i++ {
			line, n := strings.ReplaceAll(df.lines[i], "\t", "    "), numbers[i]
			y := body.Min.Y + u(3) + (i-df.scroll)*lineH
			mid := y + lineH/2
			row := image.Rect(body.Min.X, y, body.Max.X, y+lineH)
			d.zone(zone{r: row, wheel: wheel, step: lineH, click: func() { df.cursor = i }})
			ink, sign := c.text, ""
			switch {
			case strings.HasPrefix(line, "@@"):
				d.fill(row, c.hunk)
				ink = readable(c.blue, c.bg, 4)
			case n[0] == 0 && n[1] > 0:
				d.fill(row, c.added)
				sign, line = "+", line[1:]
			case n[1] == 0 && n[0] > 0:
				d.fill(row, c.removed)
				sign, line = "−", line[1:]
			case n[0] > 0:
				line = strings.TrimPrefix(line, " ")
			default:
				ink = c.dim
			}
			if i == df.cursor {
				d.fill(image.Rect(row.Min.X, y, row.Min.X+u(2), y+lineH), c.accent)
			}
			if n[0] > 0 {
				d.textRight(col1, mid, strconv.Itoa(n[0]), textCode, c.faint)
			}
			if n[1] > 0 {
				d.textRight(col2, mid, strconv.Itoa(n[1]), textCode, c.faint)
			}
			d.text(col2+u(12), mid, sign, textCode, ink)
			if r := []rune(line); len(r) > 240 {
				line = string(r[:240])
			}
			d.text(col2+u(28), mid, line, textCode, ink)
		}
		d.scrollbar(body, df.scroll, count, len(df.lines))
	})
	d.lineH(card.Min.X, card.Max.X, body.Max.Y)
	d.hints(card.Min.X+u(14), body.Max.Y+footH/2, false, "]c", "next change", "[c", "previous", "↵", "open here", "esc", "close")
}

func (d *desktop) paintDirectory(e *editor) {
	u, c, card, dir := d.u, &d.c, d.geo.dirCard, e.directory
	d.scrim(func() { e.closeDirectory(false) })
	d.card(card)
	d.block(card)
	cy := card.Min.Y + u(23)
	d.icon(glyphFolderOpen, card.Min.X+u(22), cy, 15, c.accent)
	right := card.Max.X - u(16)
	if dir.doc.b.dirty() {
		right -= d.textRight(right, cy, "Unsaved changes", textSmall, readable(c.yellow, c.raised, 4)) + u(12)
	}
	x := card.Min.X + u(38)
	d.text(x, cy, d.fitLeft(shortPath(dir.dir), right-x, textTitle), textTitle, c.text)

	body := image.Rect(card.Min.X+d.hairline(), card.Min.Y+u(46), card.Max.X-d.hairline(), card.Max.Y-u(38))
	d.paintEditor(e, body, c.bg)
	d.lineH(card.Min.X, card.Max.X, body.Min.Y)
	d.lineH(card.Min.X, card.Max.X, body.Max.Y)
	d.hints(card.Min.X+u(14), body.Max.Y+u(19), false, "↵", "open", ":w", "apply edits", "esc", "close")
}

func (d *desktop) paintDetail(e *editor) {
	n, u, c := e.notificationDetail, d.u, &d.c
	d.scrim(func() { e.notificationDetail = nil })
	width := min(u(600), d.geo.window.Dx()-u(32))
	lineH := u(21)
	lines := d.wrap(n.text, width-u(40), textBody)
	card := d.centred(width, u(48+38+24)+min(18, len(lines))*lineH)
	d.card(card)
	d.block(card)
	title := n.title
	if title == "" {
		title = "Notification"
	}
	d.text(card.Min.X+u(18), card.Min.Y+u(24), d.fit(title, width-u(36), textTitle), textTitle, c.text)
	d.lineH(card.Min.X, card.Max.X, card.Min.Y+u(48))

	body := image.Rect(card.Min.X, card.Min.Y+u(60), card.Max.X, card.Max.Y-u(50))
	// The keyboard scrolls by these lines, so they are kept on the editor's state.
	n.lines, n.rows = lines, max(1, body.Dy()/lineH)
	n.move(0)
	d.zone(zone{r: body, wheel: func(_, dy int) { n.move(dy) }, step: lineH})
	for i := n.scroll; i < len(lines) && i < n.scroll+n.rows; i++ {
		d.text(card.Min.X+u(18), body.Min.Y+(i-n.scroll)*lineH+lineH/2, lines[i], textBody, c.text)
	}
	d.scrollbar(body, n.scroll, n.rows, len(lines))

	foot := card.Max.Y - u(38)
	d.lineH(card.Min.X, card.Max.X, foot)
	pairs := []string{"esc", "back"}
	if n.jump != nil {
		pairs = []string{"o", "open source", "esc", "back"}
	}
	d.hints(card.Min.X+u(14), foot+u(19), false, pairs...)
}
