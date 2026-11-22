//go:build desktop

package editor

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"gioui.org/io/pointer"
)

// shortcutText shows the platform's own spelling of a window shortcut.
func shortcutText(mac, other string) string {
	if runtime.GOOS == "darwin" {
		return mac
	}
	return other
}

func (d *desktop) iconButton(cx, cy int, id iconID, on bool, click func()) {
	h := d.u(12)
	r := image.Rect(cx-h, cy-h, cx+h, cy+h)
	col := d.c.dim
	if d.button(r, click) {
		d.round(r, 6, d.c.hover)
		col = d.c.text
	} else if on {
		col = d.c.text
	}
	d.icon(id, cx, cy, 15, col)
}

func (d *desktop) paintTitle(e *editor) {
	r, u, c := d.geo.title, d.u, &d.c
	d.fill(r, c.side)
	d.lineH(r.Min.X, r.Max.X, r.Max.Y-d.hairline())
	cy := r.Min.Y + max(d.px(titleCenter), u(12)+d.hairline())
	var used []rect
	place := func(x0, x1 int) rect {
		box := image.Rect(x0, cy-u(12), x1, cy+u(12))
		used = append(used, box)
		return box
	}

	x := d.px(titleInset)
	if !e.noProject {
		box := place(x, x+u(24))
		d.iconButton((box.Min.X+box.Max.X)/2, cy, iconSidebar, e.tree.sidebar, func() { d.command("sidebar") })
		x = box.Max.X + u(6)
	}
	name := "Open Project"
	if !e.noProject {
		name = filepath.Base(e.root)
	}
	name = d.fit(name, u(170), textStrong)
	count, countW := "", 0
	if n := len(e.hub.live); n > 1 {
		count = strconv.Itoa(n)
		countW = max(u(17), d.width(count, textCaption)+u(10)) + u(6)
	}
	box := place(x, x+d.width(name, textStrong)+countW+u(34))
	if d.button(box, func() { d.command("sessions") }) {
		d.round(box, 6, c.hover)
	}
	tx := box.Min.X + u(9)
	tx += d.text(tx, cy, name, textStrong, c.text)
	if countW > 0 {
		pill := image.Rect(tx+u(6), cy-u(8), tx+countW, cy+u(8))
		d.round(pill, 8, c.selected)
		d.textCenter((pill.Min.X+pill.Max.X)/2, cy, count, textCaption, readable(c.accent, c.selected, 4.5))
		tx = pill.Max.X
	}
	d.icon(iconDown, tx+u(11), cy, 11, c.dim)
	left := box.Max.X + u(4)

	if e.git.branch != "" && !e.noProject && r.Dx() > u(760) {
		branch := d.fit(e.git.branch, u(140), textBody)
		box := place(left, left+d.width(branch, textBody)+u(34))
		col := c.dim
		if d.button(box, func() { d.command("branch") }) {
			d.round(box, 6, c.hover)
			col = c.text
		}
		d.icon(iconBranch, box.Min.X+u(14), cy, 13, col)
		d.text(box.Min.X+u(25), cy, branch, textBody, col)
		left = box.Max.X + u(4)
	}

	right := r.Max.X - u(10)
	action := func(id iconID, cmd string, dot bool) {
		box := place(right-u(24), right)
		mx := (box.Min.X + box.Max.X) / 2
		d.iconButton(mx, cy, id, false, func() { d.command(cmd) })
		if dot {
			d.round(image.Rect(mx+u(4), cy-u(8), mx+u(10), cy-u(2)), 3, c.accent)
		}
		right = box.Min.X - u(5)
	}
	action(iconSettings, "settings", false)
	action(iconCommand, "palette", false)
	if !e.noProject {
		action(iconBranch, "git", len(e.git.files) > 0)
	}

	// The search field sits in the middle unless the buttons push it aside.
	width := min(u(380), r.Dx()/3)
	sx := (r.Dx() - width) / 2
	if sx < left+u(10) {
		sx = left + u(10)
		width = min(width, right-u(10)-sx)
	}
	if width >= u(120) {
		box := place(sx, sx+width)
		label, keys, cmd := "Search files", shortcutText("⌘P", "Ctrl+P"), "files"
		if e.noProject {
			label, keys, cmd = "Search commands", shortcutText("⇧⌘P", "Space p"), "palette"
		}
		fill, line := c.well, c.line
		if d.button(box, func() { d.command(cmd) }) {
			fill, line = c.bg, c.strongLine
		}
		d.round(box, 7, fill)
		d.border(box, 7, line, d.hairline())
		d.icon(iconSearch, box.Min.X+u(14), cy, 12, c.dim)
		d.text(box.Min.X+u(26), cy, d.fit(label, width-u(80), textSmall), textSmall, c.dim)
		d.keyText(box.Max.X-u(10)-d.keyTextWidth(keys, textSmall), cy, keys, textSmall, c.faint)
	}

	slices.SortFunc(used, func(a, b rect) int { return a.Min.X - b.Min.X })
	x = 0
	if titleInset > 20 {
		x = d.px(titleInset) // leave the traffic lights to the system
	}
	for _, box := range used {
		if box.Min.X > x {
			d.drags = append(d.drags, image.Rect(x, r.Min.Y, box.Min.X, r.Max.Y))
		}
		x = max(x, box.Max.X)
	}
	if x < r.Max.X {
		d.drags = append(d.drags, image.Rect(x, r.Min.Y, r.Max.X, r.Max.Y))
	}
}

func (d *desktop) paintSidebar(e *editor) {
	r, u, c, t := d.geo.sidebar, d.u, &d.c, &e.tree
	if r.Empty() {
		return
	}
	d.fill(r, c.side)
	d.lineV(r.Max.X-d.hairline(), r.Min.Y, r.Max.Y)

	header := image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+u(36))
	cy := (header.Min.Y + header.Max.Y) / 2
	treeKey := func(key string) {
		if !e.b.dashboard && e.mode == "insert" {
			e.normal()
		}
		t.focused = true
		e.treeInput(key)
	}
	if t.filtering || t.filter != "" {
		box := image.Rect(r.Min.X+u(8), cy-u(12), r.Max.X-u(8), cy+u(12))
		d.round(box, 6, c.well)
		line := c.line
		if t.filtering {
			line = c.accent
		}
		d.border(box, 6, line, d.hairline())
		d.icon(iconSearch, box.Min.X+u(13), cy, 12, c.dim)
		x := box.Min.X + u(25)
		x += d.text(x, cy, d.fitLeft(t.filter, box.Dx()-u(54), textBody), textBody, c.text)
		if t.filtering {
			d.caret(x+u(1), cy, u(14))
		}
		d.iconButton(box.Max.X-u(12), cy, iconClose, false, func() {
			t.filter, t.filtering = "", false
			e.refreshTree("")
		})
	} else {
		d.text(r.Min.X+u(14), cy, "FILES", textCaption, c.dim)
		x := r.Max.X - u(20)
		for _, b := range []struct {
			id  iconID
			key string
		}{{iconSearch, "/"}, {iconNewFolder, "A"}, {iconNewFile, "a"}} {
			d.iconButton(x, cy, b.id, false, func() { treeKey(b.key) })
			x -= u(27)
		}
		if t.sortMode != 0 && r.Dx() > u(190) {
			d.textRight(x, cy, "by "+treeSorts[t.sortMode], textSmall, c.faint)
		}
	}

	body := image.Rect(r.Min.X, header.Max.Y, r.Max.X-d.hairline(), r.Max.Y-u(4))
	rowH := u(25)
	count := max(1, body.Dy()/rowH)
	last := max(0, len(t.rows)-count)
	t.scroll = max(0, min(t.scroll, last))
	if !t.manualScroll && (t.focused || t.reveal) {
		t.scroll = max(0, max(t.selected-count+1, min(t.selected, t.scroll)))
	}
	t.reveal = false
	d.zone(zone{r: body, wheel: func(_, dy int) {
		t.manualScroll = true
		t.scroll = max(0, min(last, t.scroll+dy))
	}, step: rowH})

	for i := t.scroll; i < len(t.rows) && i < t.scroll+count; i++ {
		row := t.rows[i]
		y := body.Min.Y + (i-t.scroll)*rowH
		line := image.Rect(body.Min.X+u(6), y+d.hairline(), body.Max.X-u(6), y+rowH-d.hairline())
		pick := func() { t.selected, t.focused, t.manualScroll = i, true, false }
		d.zone(zone{
			r: line, cursor: pointer.CursorPointer, step: rowH,
			click: func() { pick(); e.treeInput("enter") },
			right: func() { pick(); e.treeContext() },
			wheel: func(_, dy int) {
				t.manualScroll = true
				t.scroll = max(0, min(last, t.scroll+dy))
			},
		})
		current := !row.dir && row.path == e.b.path
		switch {
		case i == t.selected && t.focused:
			d.round(line, 6, c.selected)
		case current:
			d.round(line, 6, c.open)
		case d.hot(line):
			d.round(line, 6, c.hover)
		}
		mid := y + rowH/2
		for k := 1; k < row.depth; k++ {
			d.lineV(line.Min.X+u(float32(11+(k-1)*13))+u(13), y, y+rowH)
		}
		x := line.Min.X + u(11) + row.depth*u(13)
		open := row.dir && (t.expanded[row.path] || t.filter != "")
		if row.dir {
			id, folder := iconRight, glyphFolder
			if open {
				id, folder = iconDown, glyphFolderOpen
			}
			d.icon(id, x, mid, 11, c.dim)
			d.icon(folder, x+u(17), mid, 15, c.accent)
		} else {
			d.fileIcon(x+u(17), mid, row.path)
		}
		x += u(30)
		st, col := textBody, c.text
		if i == 0 {
			st = textStrong
		}
		right := line.Max.X - u(8)
		if mark := e.git.files[row.path]; mark != 0 {
			ink := readable(d.gitInk(mark), c.side, 3.5)
			if row.dir {
				k := u(3)
				d.round(image.Rect(right-2*k, mid-k, right, mid+k), 3, ink)
				right -= u(14)
			} else {
				label := string(mark)
				if mark == '?' {
					label = "U"
				}
				right -= d.textRight(right, mid, label, textCaption, ink) + u(8)
				col = ink
			}
		}
		if t.cut == row.path {
			col = c.faint
		}
		d.text(x, mid, d.fit(filepath.Base(row.path), right-x, st), st, col)
	}
	d.scrollbar(body, t.scroll, count, len(t.rows))

	edge := image.Rect(r.Max.X-u(3), r.Min.Y, r.Max.X+u(3), r.Max.Y)
	if d.hot(edge) {
		d.fill(image.Rect(r.Max.X-u(2), r.Min.Y, r.Max.X, r.Max.Y), c.accent)
	}
	d.zone(zone{r: edge, cursor: pointer.CursorColResize, drag: func() { t.width = max(18, min(64, d.mx/u(9))) }})
}

type tabSlot struct {
	r rect
	b *buffer
}

func bufferName(b *buffer) string {
	switch {
	case b.dashboard:
		return "Welcome"
	case b.path == "":
		return "Untitled"
	}
	return filepath.Base(b.path)
}

func (d *desktop) paintTabs(e *editor) {
	r, u, c, v := d.geo.tabs, d.u, &d.c, &d.m.view
	if r.Empty() {
		return
	}
	d.fill(r, c.side)
	d.lineH(r.Min.X, r.Max.X, r.Max.Y-d.hairline())

	names := make([]string, len(e.buffers))
	for i, b := range e.buffers {
		names[i] = bufferName(b)
	}
	total, activeX, activeW := 0, 0, 0
	widths := make([]int, len(names))
	for i, b := range e.buffers {
		label := names[i]
		// Equal names get their folder to tell them apart.
		if b.path != "" {
			for j, other := range names {
				if j != i && other == names[i] {
					label += "  " + filepath.Base(filepath.Dir(b.path))
					break
				}
			}
		}
		w := min(u(230), max(u(118), d.width(label, textStrong)+u(66)))
		if b == e.b {
			activeX, activeW = total, w
		}
		widths[i] = w
		total += w
	}
	if e.b != d.tabShown {
		d.tabShown = e.b
		if activeX < d.tabScroll {
			d.tabScroll = activeX
		} else if activeX+activeW > d.tabScroll+r.Dx() {
			d.tabScroll = activeX + activeW - r.Dx()
		}
	}
	d.tabScroll = max(0, min(d.tabScroll, total-r.Dx()))
	wheel := func(dx, dy int) { d.tabScroll += (dx + dy) * u(32) }
	d.zone(zone{r: r, wheel: wheel, step: u(12)})

	d.clipped(r, func() {
		x, cy := r.Min.X-d.tabScroll, (r.Min.Y+r.Max.Y)/2
		for i, b := range e.buffers {
			tab := image.Rect(x, r.Min.Y, x+widths[i], r.Max.Y)
			x = tab.Max.X
			seen := tab.Intersect(r)
			if seen.Empty() {
				continue
			}
			d.tabs = append(d.tabs, tabSlot{tab, b})
			d.zone(zone{
				r: seen, cursor: pointer.CursorPointer, wheel: wheel, step: u(12),
				click: func() {
					v.tabPrev, v.tabStartX = e.b, d.mx
					e.switchBuffer(b)
				},
				middle: func() { d.closeTab(b) },
				drag:   func() { d.dragTab(e, b) },
				drop: func() {
					if v.tabMoving {
						v.dropTab(e)
					}
					v.tabDrag, v.tabMoving, v.drop = nil, false, dropZone{}
				},
			})
			active, hover := b == e.b, d.hot(seen)
			col, st := c.dim, textBody
			switch {
			case active:
				d.fill(tab, c.bg)
				d.fill(image.Rect(tab.Min.X, tab.Min.Y, tab.Max.X, tab.Min.Y+u(2)), c.accent)
				col, st = c.text, textStrong
			case hover:
				d.fill(image.Rect(tab.Min.X, tab.Min.Y, tab.Max.X, tab.Max.Y-d.hairline()), c.hover)
				col = c.text
			}
			d.lineV(tab.Max.X-d.hairline(), tab.Min.Y, tab.Max.Y)

			ix := tab.Min.X + u(18)
			if b.dashboard {
				d.logo(ix, cy, 14)
			} else {
				d.fileIcon(ix, cy, b.path)
			}
			tx, closeX := ix+u(14), tab.Max.X-u(18)
			end := closeX - u(12)
			tx += d.text(tx, cy, d.fit(names[i], end-tx, st), st, col) + u(6)
			if b.path != "" && slices.ContainsFunc(e.buffers, func(o *buffer) bool { return o != b && bufferName(o) == names[i] }) {
				d.text(tx, cy, d.fit(filepath.Base(filepath.Dir(b.path)), end-tx, textSmall), textSmall, c.faint)
			}
			box := image.Rect(closeX-u(9), cy-u(9), closeX+u(9), cy+u(9))
			over := d.hot(box.Intersect(r))
			switch {
			case b.dirty() && !over:
				k := u(3.5)
				d.round(image.Rect(closeX-k, cy-k, closeX+k, cy+k), 4, c.yellow)
			case active || hover:
				if over {
					d.round(box, 5, c.hover)
				}
				d.icon(iconClose, closeX, cy, 12, col)
			default:
				continue
			}
			d.button(box.Intersect(r), func() { d.closeTab(b) })
		}
	})
}

// dragTab reorders tabs along the strip and offers splits over the text.
func (d *desktop) dragTab(e *editor, b *buffer) {
	v := &d.m.view
	if !v.tabMoving && absInt(d.mx-v.tabStartX) < d.u(5) && d.my < d.geo.tabs.Max.Y {
		return
	}
	v.tabDrag, v.tabMoving, v.drop = b, true, dropZone{}
	if d.my < d.geo.tabs.Max.Y {
		from := slices.Index(e.buffers, b)
		for _, t := range d.tabs {
			if d.mx < t.r.Min.X || d.mx >= t.r.Max.X || t.b == b || e.b != b {
				continue
			}
			to, middle := slices.Index(e.buffers, t.b), (t.r.Min.X+t.r.Max.X)/2
			if to > from && d.mx > middle || to >= 0 && to < from && d.mx < middle {
				e.moveTab(to)
			}
		}
		return
	}
	x, y := d.cell()
	for _, pane := range v.panes {
		if pane.leaf != nil && pane.contains(x, y) && !pane.b.dashboard {
			v.drop = dropZoneAt(pane, x, y)
			return
		}
	}
}

// pathFill is the band behind the path bar, tinted so it stands apart from
// both the tabs and the code.
func (d *desktop) pathFill() color.NRGBA { return blend(d.c.bg, d.c.accent, 0.12) }

// paintCrumbs draws where the cursor is: the file's path, then the functions
// and blocks around it.
func (d *desktop) paintCrumbs(e *editor) {
	r, u, c := d.geo.crumbs, d.u, &d.c
	if r.Empty() {
		return
	}
	band := d.pathFill()
	d.fill(r, band)
	d.fill(image.Rect(r.Min.X, r.Max.Y-d.hairline(), r.Max.X, r.Max.Y), blend(band, c.accent, 0.4))

	path := shortPath(e.b.path)
	if !e.noProject && within(e.root, e.b.path) {
		if rel, err := filepath.Rel(e.root, e.b.path); err == nil {
			path = rel
		}
	}
	parts := strings.Split(strings.Trim(filepath.ToSlash(path), "/"), "/")
	dirs, name := parts[:len(parts)-1], parts[len(parts)-1]

	x, cy, right := r.Min.X+u(14), (r.Min.Y+r.Max.Y)/2, r.Max.X-u(14)
	mark := u(24) // the file's icon, ahead of its name
	var scopes []string
	if e.cfg.Breadcrumbs {
		scopes = e.scopeTrail(e.b)
	}
	tail := 0
	for _, s := range scopes {
		tail += u(22) + d.width(s, textPath)
	}
	// Short of room the folders give way first, then the innermost blocks.
	dirs = d.fitFolders(dirs, right-x-mark-d.width(name, textPathFile)-tail)
	folder, chevron := readable(c.dim, band, 4.5), readable(c.dim, band, 3)
	for _, dir := range dirs {
		x += d.text(x, cy, dir, textPath, folder)
		d.icon(iconRight, x+u(11), cy, 11, chevron)
		x += u(22)
	}
	d.fileIcon(x+u(9), cy, e.b.path)
	x += mark
	ink := readable(c.text, band, 4.5)
	x += d.text(x, cy, d.fit(name, right-x, textPathFile), textPathFile, ink)
	for _, scope := range scopes {
		if right-x < u(50) {
			break
		}
		d.icon(iconRight, x+u(11), cy, 11, chevron)
		x += u(22)
		x += d.text(x, cy, d.fit(scope, right-x, textPath), textPath, ink)
	}
}

// fitFolders drops folders from the front until they fit in w, so the file's
// name is the last thing to be cut.
func (d *desktop) fitFolders(dirs []string, w int) []string {
	used := func(list []string) int {
		n := 0
		for _, s := range list {
			n += d.width(s, textPath) + d.u(22)
		}
		return n
	}
	if used(dirs) <= w {
		return dirs
	}
	for i := 1; i <= len(dirs); i++ {
		if short := append([]string{"…"}, dirs[i:]...); used(short) <= w {
			return short
		}
	}
	return nil
}

func (d *desktop) paintStatus(e *editor) {
	r, u, c, st := d.geo.status, d.u, &d.c, textSmall
	if r.Empty() {
		return
	}
	d.fill(r, c.side)
	d.lineH(r.Min.X, r.Max.X, r.Min.Y)
	cy := (r.Min.Y + r.Max.Y) / 2

	doc := e
	if e.directory != nil {
		doc = e.directory.doc
	}
	b := doc.b
	x, right := r.Min.X+u(8), r.Max.X-u(6)
	if !b.dashboard || e.menu != nil || e.tree.focused {
		mode, modeColor := e.modeBadge()
		fill := nrgba(modeColor, c.accent)
		ink := black
		if contrast(white, fill) > contrast(black, fill) {
			ink = white
		}
		w := d.width(mode, textCaption)
		d.round(image.Rect(x, cy-u(9), x+w+u(16), cy+u(9)), 5, fill)
		d.text(x+u(8), cy, mode, textCaption, ink)
		x += w + u(26)
	}

	keys := keyLabel(doc.prefix)
	if doc.pending.kind != "" {
		keys += " " + doc.pending.kind
	}
	if doc.count > 0 {
		keys = fmt.Sprintf("%d %s", doc.count, keys)
	}
	if doc.recording != "" {
		keys = "Recording @" + doc.recording + " " + keys
	}
	if keys = strings.TrimSpace(keys); keys != "" {
		w := d.width(keys, st)
		d.round(image.Rect(x, cy-u(9), x+w+u(14), cy+u(9)), 5, c.hover)
		d.text(x+u(7), cy, keys, st, c.text)
		x += w + u(24)
	}

	if !b.dashboard && e.problemsVisible(b) {
		file, project := countProblems(b.diagnostics), e.projectProblems()
		errors, warnings := file.errors+project.errors, file.warnings+project.warnings
		es, ws := fmt.Sprint(errors), fmt.Sprint(warnings)
		box := image.Rect(x, r.Min.Y, x+d.width(es, st)+d.width(ws, st)+u(58), r.Max.Y)
		if d.button(box, func() { d.command("problems") }) {
			d.fill(box, c.hover)
		}
		ec, wc := c.dim, c.dim
		if errors > 0 {
			ec = readable(c.red, c.side, 3.5)
		}
		if warnings > 0 {
			wc = readable(c.yellow, c.side, 3.5)
		}
		d.icon(iconError, x+u(12), cy, 13, ec)
		tx := x + u(22)
		tx += d.text(tx, cy, es, st, ec)
		d.icon(iconWarning, tx+u(13), cy, 13, wc)
		d.text(tx+u(23), cy, ws, st, wc)
		x = box.Max.X + u(8)
	}

	segment := func(s string, click func()) {
		w := d.width(s, st) + u(16)
		if right-w < x {
			return
		}
		box, col := image.Rect(right-w, r.Min.Y, right, r.Max.Y), c.dim
		if click != nil && d.button(box, click) {
			d.fill(box, c.hover)
			col = c.text
		}
		d.text(box.Min.X+u(8), cy, s, st, col)
		right = box.Min.X
	}
	if b.dashboard {
		segment("Loom "+version, nil)
		return
	}
	if e.directory != nil {
		return
	}
	row := b.rowAt(b.cursor)
	segment(fmt.Sprintf("Ln %d, Col %d", row+1, b.cursor-b.lines[row]+1), func() { d.command("goto-line") })
	lang := languageName(e.cfg.languageFor(b.path))
	segment(lang, nil)
	ending := "LF"
	if b.newline() == "\r\n" {
		ending = "CRLF"
	}
	segment(ending, nil)
	segment("UTF-8", nil)
	if b.external {
		segment("Changed on disk", func() { d.command("save") })
	}
}

func languageName(id string) string {
	names := map[string]string{
		"": "Plain Text", "go": "Go", "mod": "Go Module", "gomod": "Go Module", "md": "Markdown", "markdown": "Markdown",
		"js": "JavaScript", "javascript": "JavaScript", "ts": "TypeScript", "typescript": "TypeScript", "tsx": "TSX", "jsx": "JSX",
		"py": "Python", "python": "Python", "rs": "Rust", "rust": "Rust", "yml": "YAML", "yaml": "YAML", "json": "JSON",
		"html": "HTML", "css": "CSS", "sh": "Shell", "c": "C", "cpp": "C++", "toml": "TOML", "sql": "SQL",
		"asm": "Assembly", "cmake": "CMake", "csharp": "C#", "dockerfile": "Dockerfile", "graphql": "GraphQL",
		"javascriptreact": "JavaScript JSX", "makefile": "Makefile", "ocaml": "OCaml", "php": "PHP", "proto": "Protocol Buffers",
		"shellscript": "Shell", "typescriptreact": "TypeScript TSX", "xml": "XML",
	}
	if name, ok := names[strings.ToLower(id)]; ok {
		return name
	}
	return humanize(id)
}
