package editor

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Popups: pickers, the find bar, completion, signature help, hover, which-key,
// toasts and notification details, described as data for the web view.

// listState is the picker's scroll position; it follows the selection.
type listState struct {
	m             *menu
	input         string
	top, selected int
	rows          int // how many rows the web view shows
}

type previewCache struct {
	b     *buffer
	spans []highlight
}

const pickerWindow = 40 // rows sent around the visible ones

func (b *Bridge) popups(ui map[string]any) {
	e := b.m.editor
	if e.notificationDetail != nil {
		ui["detail"] = b.detailState()
	} else if e.menu != nil {
		ui["menu"] = b.menuState()
	}
	if e.menu == nil && e.panel == nil && e.diff == nil && e.directory == nil {
		if e.bar != nil {
			ui["find"] = b.findState()
		}
		if c := b.completionState(); c != nil {
			ui["completion"] = c
		}
		if h := b.hoverState(); h != nil {
			ui["hover"] = h
		}
	}
	if e.menu == nil && e.panel == nil && e.diff == nil && e.notificationDetail == nil {
		if w := b.whichKeyState(); w != nil {
			ui["whichkey"] = w
		}
	}
	if t := b.toastState(); t != nil {
		ui["toast"] = t
	}
}

// fieldState is a text field's text split at its caret and selection.
func fieldState(text string, f *lineField) map[string]any {
	before, middle, after := f.parts(text)
	_, _, selecting := f.selection(text)
	caretFirst := selecting && f.caret(text) < utf8.RuneCountInString(before+middle)
	return map[string]any{"before": before, "sel": middle, "after": after, "caretFirst": caretFirst}
}

func (b *Bridge) menuState() map[string]any {
	e := b.m.editor
	m := e.menu
	k := kindOfPicker(m)
	s := map[string]any{"title": m.title, "selected": m.selected, "total": len(m.matches)}
	preview := false
	if !m.hidePreview && !k.keys && !k.prompt {
		preview = m.details || slices.ContainsFunc(m.options, func(o option) bool { return o.preview != nil }) || k.files
	}
	if k.prompt {
		s["prompt"] = true
	}
	if m.confirmation {
		s["confirmation"] = true
	} else {
		s["field"] = fieldState(m.input, &m.field)
		s["placeholder"] = pickerPlaceholder(m, k)
	}
	if !k.prompt && len(m.options) > 0 {
		count := fmt.Sprint(len(m.matches))
		if len(m.matches) < len(m.options) {
			count = fmt.Sprintf("%d of %d", len(m.matches), len(m.options))
		}
		s["count"] = count
	}
	switch {
	case m.confirmation:
		s["hints"] = []string{"↵", "apply", "esc", "cancel"}
	case k.prompt:
		s["hints"] = []string{"↵", "confirm", "esc", "cancel"}
	case m.details:
		s["hints"] = []string{"↑↓", "navigate", "↵", "details", "esc", "close"}
	case m.remove != nil:
		s["hints"] = []string{"↑↓", "navigate", "↵", "switch", "Ctrl+X", "close session", "esc", "cancel"}
	default:
		s["hints"] = []string{"↑↓", "navigate", "↵", "select", "esc", "close"}
	}

	n := len(m.matches)
	if n == 0 {
		msg := m.empty
		switch {
		case m.submit != nil && strings.TrimSpace(m.input) != "":
			msg = "Press Enter to use “" + m.input + "”"
		case m.input != "":
			msg = "No results for “" + m.input + "”"
		case msg == "" && m.submit != nil:
			msg = "Type, then press Enter"
		case msg == "":
			msg = "Nothing to show"
		}
		if !k.prompt {
			s["empty"] = msg
		}
	}
	ls := &b.list
	count := max(1, ls.rows)
	if ls.m != m || ls.input != m.input || ls.selected != m.selected {
		if ls.m != m || ls.input != m.input {
			ls.top = 0
		}
		ls.m, ls.input, ls.selected = m, m.input, m.selected
		ls.top = max(m.selected-count+1, min(m.selected, ls.top))
	}
	ls.top = max(0, min(ls.top, max(0, n-count)))
	s["top"] = ls.top
	from, to := max(0, ls.top-pickerWindow/4), min(n, ls.top+pickerWindow)
	rows := make([]any, 0, to-from)
	for i := from; i < to; i++ {
		rows = append(rows, b.menuRow(m, k, m.matches[i]))
	}
	s["from"], s["rows"] = from, rows
	if preview {
		s["preview"] = b.previewState(m)
	}
	return s
}

// matchRanges is which runes of label a fuzzy query matches, as ranges.
func matchRanges(label, query string) [][2]int {
	hit := matchRunes(label, query)
	var out [][2]int
	for i := range utf8.RuneCountInString(label) {
		if !hit[i] {
			continue
		}
		if n := len(out); n > 0 && out[n-1][1] == i {
			out[n-1][1] = i + 1
		} else {
			out = append(out, [2]int{i, i + 1})
		}
	}
	return out
}

func (b *Bridge) menuRow(m *menu, k pickerKind, o option) map[string]any {
	e := b.m.editor
	label, hint := o.label, o.hint
	if k.keys {
		label, hint = o.hint, o.label
	}
	row := map[string]any{}
	switch {
	case k.files && label == "Dashboard":
		row["icon"] = []string{"logo"}
		label = "Welcome"
	case k.files:
		path, changed := strings.CutSuffix(label, " *")
		if filepath.IsAbs(path) {
			if !e.noProject && within(e.root, path) {
				if rel, err := filepath.Rel(e.root, path); err == nil {
					path = rel
				}
			} else {
				path = shortPath(path)
			}
		}
		mark := b.fileIcon(path)
		row["icon"] = []string{"file", mark[0], mark[1]}
		label = filepath.Base(path)
		if dir := filepath.Dir(path); dir != "." {
			row["detail"] = dir
		}
		if changed {
			label += " •"
		}
		hint = ""
	case m.remove != nil:
		id, ink := "folderOpen", "var(--accent)"
		parts := strings.Split(hint, sessionSep)
		switch parts[0] {
		case "":
			id = "newFolder"
		case "recent":
			id, ink = "folder", "var(--faint)"
		}
		row["icon"] = []string{"svg", id, ink}
		if hint != "" {
			row["detail"], hint = parts[len(parts)-1], strings.Join(parts[:len(parts)-1], sessionSep)
		}
	case m.title == "Switch Branch":
		row["icon"] = []string{"svg", "branch", "var(--dim)"}
	case strings.HasPrefix(m.title, "Open Project"):
		row["icon"] = []string{"svg", "folder", "var(--accent)"}
		label = strings.TrimSuffix(label, "/")
		if hint == "browse" {
			hint = ""
		}
	}
	row["label"] = label
	if hits := matchRanges(label, m.input); len(hits) > 0 {
		row["hits"] = hits
	}
	switch {
	case hint == "":
	case hint == "current":
		row["hint"] = []string{"check"}
	case hint == "on" || hint == "off":
		row["hint"] = []string{"toggle", hint}
	case isKeys(hint) && (k.keys || k.command || m.title == "Editor Actions" || m.title == "Explorer Actions"):
		row["hint"] = []string{"keys", hint}
	default:
		row["hint"] = []string{"text", strings.Join(strings.Fields(hint), " ")}
	}
	return row
}

func (b *Bridge) previewState(m *menu) map[string]any {
	e := b.m.editor
	if len(m.matches) == 0 || m.selected < len(m.matches) && m.matches[m.selected].preview == nil {
		return map[string]any{"empty": "Nothing to preview"}
	}
	p := m.selectedPreview()
	title := p.title
	if filepath.IsAbs(title) {
		if !e.noProject && within(e.root, title) {
			if rel, err := filepath.Rel(e.root, title); err == nil {
				title = rel
			}
		} else {
			title = shortPath(title)
		}
	}
	if title == "Dashboard" {
		title = "Welcome"
	}
	s := map[string]any{"title": title}
	if p.message != "" {
		s["message"] = p.message
	}
	if p.source != nil {
		s["code"] = b.sourceLines(p, &m.previewScroll, 26)
	}
	return s
}

// sourceLines is count lines of a preview's source around its focus row,
// highlighted, as [number, current, diagnostic colour, runs].
func (b *Bridge) sourceLines(p itemPreview, scroll *int, count int) []any {
	e, src, c := b.m.editor, p.source, &b.frame.colors
	if b.preview.b != src {
		b.preview = previewCache{b: src}
		// Highlighting has to keep up with arrowing through the list.
		if len(src.text) < 200_000 {
			b.preview.spans = highlightSyntax(src, e.cfg.languageFor(src.path))
		}
	}
	start := min(max(0, len(src.lines)-count), max(0, p.row-count/2))
	*scroll = max(-start, min(*scroll, max(0, len(src.lines)-count)-start))
	start += *scroll
	last := min(len(src.lines), start+count)
	at := seekHighlight(b.preview.spans, src.lines[start])
	marks := diagnosticRows(src, start, last)
	tab := e.tabSizeFor(src.path)
	var out []any
	for row := start; row < last; row++ {
		diag := ""
		if sev := diagnosticAt(marks[row], src.lines[row], src.lineEnd(row)+1); sev != 0 {
			diag = hexColor(nrgba(diagnosticColor(sev), c.red))
		}
		var runs [][2]string
		var text strings.Builder
		runFG, column := th.fg, 0
		flush := func() {
			if text.Len() > 0 {
				runs = append(runs, [2]string{text.String(), hexColor(b.cellFG(runFG, c.bg))})
				text.Reset()
			}
		}
		end := min(src.lineEnd(row), src.lines[row]+240)
		for off := src.lines[row]; off < end; off++ {
			fg := th.fg
			if style, ok := styleAt(b.preview.spans, &at, off); ok {
				fg = style.fg
			}
			if fg != runFG {
				flush()
				runFG = fg
			}
			switch ch := src.text[off]; ch {
			case '\t':
				n := max(1, tab-column%max(1, tab))
				text.WriteString(strings.Repeat(" ", n))
				column += n
			case '\r':
			default:
				text.WriteRune(ch)
				column++
			}
		}
		flush()
		current := row == p.row && (p.row > 0 || p.column > 0)
		out = append(out, []any{row + 1, current, diag, runs})
	}
	return out
}

func (b *Bridge) findState() map[string]any {
	e := b.m.editor
	bar := e.bar
	count, bad := e.findCount(e.matches(e.b))
	s := map[string]any{
		"find": fieldState(bar.find, &bar.findField), "field": bar.field, "replaceOn": bar.showReplace,
		"case": e.searchOpts.caseSensitive, "word": e.searchOpts.wholeWord, "regex": e.searchOpts.regex,
		"count": count, "bad": bad,
	}
	if bar.showReplace {
		s["replace"] = fieldState(bar.replace, &bar.replaceField)
	}
	return s
}

func (b *Bridge) completionState() map[string]any {
	e, f := b.m.editor, b.m.view.screen
	if e.mode != "insert" || !f.cursorVisible {
		return nil
	}
	comp, sig := &e.completion, e.signature
	s := map[string]any{"x": f.cursorX, "y": f.cursorY}
	if len(comp.items) > 0 {
		first := max(0, comp.selected-8)
		var items []any
		for i := first; i < len(comp.items) && i < first+9; i++ {
			items = append(items, []string{comp.items[i].Label, comp.items[i].Detail})
		}
		s["items"], s["first"], s["selected"], s["total"] = items, first, comp.selected, len(comp.items)
		if comp.selected < len(comp.items) {
			if docs := strings.TrimSpace(comp.items[comp.selected].docs()); docs != "" {
				s["docs"] = docs
			}
		}
	}
	if sig.label != "" && sig.start >= 0 && sig.start <= sig.end && sig.end <= len(sig.label) {
		clean := func(s string) string { return strings.Map(signatureDisplayRune, s) }
		s["signature"] = []string{clean(sig.label[:sig.start]), clean(sig.label[sig.start:sig.end]), clean(sig.label[sig.end:])}
	}
	if s["items"] == nil && s["signature"] == nil {
		return nil
	}
	return s
}

func (b *Bridge) hoverState() map[string]any {
	e := b.m.editor
	h := &b.m.view.diagnosticHover
	if h.buffer == nil || h.message == "" || len(e.completion.items) > 0 {
		return nil
	}
	title := h.title
	switch {
	case title == "Hover Documentation":
		title = ""
	case title == "":
		title = "Problem"
	}
	return map[string]any{"x": h.x, "y": h.y, "title": title, "text": h.message}
}

func (b *Bridge) whichKeyState() map[string]any {
	e := b.m.editor
	doc := e
	if e.directory != nil {
		doc = e.directory.doc
	}
	if doc.prefix == "" {
		return nil
	}
	var entries [][2]string
	bound := doc.keys[doc.cfg.Profile+"."+doc.mode]
	tabs := false
	for _, seq := range sortedKeys(bound) {
		suffix, ok := strings.CutPrefix(seq, doc.prefix+" ")
		if !ok {
			continue
		}
		id := bound[seq]
		if n, ok := strings.CutPrefix(id, "tab-"); ok {
			// One entry stands in for the nine numbered tab shortcuts.
			if _, err := strconv.Atoi(n); err == nil {
				if !tabs {
					tabs = true
					at := strings.TrimRight(keyLabel(suffix), "0123456789")
					entries = append(entries, [2]string{at + "1–9", "Go to Tab"})
				}
				continue
			}
		}
		title := doc.commands[id].title
		if title == "" {
			title = humanize(id)
		}
		entries = append(entries, [2]string{keyLabel(suffix), title})
	}
	if len(entries) == 0 {
		return nil
	}
	slices.SortStableFunc(entries, func(a, b [2]string) int { return len(strings.Fields(a[0])) - len(strings.Fields(b[0])) })
	return map[string]any{"title": chordTitle(doc.prefix), "entries": entries}
}

// toastState is the newest notification while the editor still shows it;
// the web view decides how long it stays.
func (b *Bridge) toastState() map[string]any {
	e := b.m.editor
	n := len(e.notifications)
	if n == 0 || e.message == "" || e.notificationDetail != nil {
		return nil
	}
	last := e.notifications[n-1]
	if !last.at.After(b.m.switchedAt) {
		return nil // older than the session on screen
	}
	stay := 3500
	if looksBad(last.text) || len(last.text) > 90 {
		stay = 7000
	}
	return map[string]any{"id": last.at.UnixNano(), "text": last.text, "bad": looksBad(last.text), "stay": stay}
}

func (b *Bridge) detailState() map[string]any {
	n := b.m.editor.notificationDetail
	title := n.title
	if title == "" {
		title = "Notification"
	}
	// The keyboard scrolls by lines; the web view shows about this many.
	n.lines, n.rows = strings.Split(n.text, "\n"), 18
	n.move(0)
	hints := []string{"esc", "back"}
	if n.jump != nil {
		hints = []string{"o", "open source", "esc", "back"}
	}
	return map[string]any{"title": title, "text": n.text, "scroll": n.scroll, "hints": hints}
}

func looksBad(s string) bool {
	s = strings.ToLower(s)
	for _, word := range []string{"error", "failed", "cannot", "can't", "could not", "not found", "denied", "fatal", "invalid", "no such"} {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}

// popupAct handles clicks and drags in the popups.
func (b *Bridge) popupAct(name string, i int, s string) bool {
	e := b.m.editor
	m := e.menu
	switch name {
	case "menu-close":
		e.menu = nil
	case "menu-pick":
		if m == nil || i < 0 || i >= len(m.matches) {
			return true
		}
		m.selected = i
		if !m.confirmation && m.matches[i].run != nil {
			e.menu = nil
			m.matches[i].run()
		}
	case "menu-rows":
		b.list.rows = max(1, i)
	case "menu-scroll":
		if m != nil {
			b.list.top = max(0, min(len(m.matches)-max(1, b.list.rows), b.list.top+i))
		}
	case "preview-scroll":
		if m != nil {
			m.previewScroll += i
		}
	case "field":
		// s is "menu", "find" or "replace", then "press", "shift" or "drag".
		which, how, _ := strings.Cut(s, "-")
		text, f := b.fieldFor(which)
		if f == nil {
			return true
		}
		if which == "find" {
			e.barAction("focus-find")
		} else if which == "replace" {
			e.barAction("focus-replace")
		}
		switch how {
		case "drag":
			f.drag(*text, i)
		default:
			f.press(*text, i, how == "shift")
		}
	case "bar":
		if e.bar != nil {
			e.barAction(s)
		}
	case "complete":
		comp := &e.completion
		if i >= 0 && i < len(comp.items) {
			comp.selected = i
			e.completionInput("enter")
			e.scheduleLanguage()
			e.scheduleGit()
		}
	case "complete-scroll":
		comp := &e.completion
		comp.selected = max(0, min(len(comp.items)-1, comp.selected+i))
	case "hover-close":
		b.m.view.clearDiagnosticHover()
	case "notifications":
		e.openNotifications()
	case "detail-scroll":
		if n := e.notificationDetail; n != nil {
			n.scroll = max(0, i)
		}
	case "detail-close":
		e.notificationDetail = nil
	default:
		return false
	}
	return true
}

func (b *Bridge) fieldFor(which string) (*string, *lineField) {
	e := b.m.editor
	switch {
	case which == "menu" && e.menu != nil && !e.menu.confirmation:
		return &e.menu.input, &e.menu.field
	case which == "find" && e.bar != nil:
		return &e.bar.find, &e.bar.findField
	case which == "replace" && e.bar != nil:
		return &e.bar.replace, &e.bar.replaceField
	}
	return nil, nil
}
