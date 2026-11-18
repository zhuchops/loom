package main

import (
	"regexp"
	"strings"
	"unicode"
)

// A styled rune, so wrapping can work on text that has already been styled.
type sr struct {
	r rune
	s cellStyle
}

// mdLine is one rendered row. start and end are the source rows it came from,
// which lets the preview follow the editor cursor.
type mdLine struct {
	cells      []sr
	start, end int
	fill       bool // paint the whole row with the code background
	rule       bool
}

var (
	mdHeading   = regexp.MustCompile(`^ {0,3}(#{1,6})\s+(.*?)\s*#*\s*$`)
	mdFence     = regexp.MustCompile("^\\s*(```+|~~~+)\\s*(\\S*)")
	mdRule      = regexp.MustCompile(`^ {0,3}([-*_])(\s*[-*_]){2,}\s*$`)
	mdList      = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])\s+(.*)$`)
	mdQuote     = regexp.MustCompile(`^ {0,3}>\s?(.*)$`)
	mdTableSep  = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$`)
	mdSetextOne = regexp.MustCompile(`^ {0,3}=+\s*$`)
	mdSetextTwo = regexp.MustCompile(`^ {0,3}-+\s*$`)
)

func renderMarkdown(text []rune, width int) []mdLine {
	src := strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n")
	return mdBlocks(src, 0, max(8, width))
}

func styled(s string, style cellStyle) []sr {
	var out []sr
	for _, r := range s {
		out = append(out, sr{r, style})
	}
	return out
}

func srWidth(cells []sr) int {
	w := 0
	for _, c := range cells {
		w += cellWidth(string(c.r))
	}
	return w
}

// wrapCells breaks styled text on spaces, splitting words that are too long.
func wrapCells(cells []sr, width int) [][]sr {
	width = max(1, width)
	var lines [][]sr
	var line []sr
	lineWidth := 0
	flush := func() {
		for len(line) > 0 && line[len(line)-1].r == ' ' {
			line = line[:len(line)-1]
		}
		lines = append(lines, line)
		line, lineWidth = nil, 0
	}
	for i := 0; i < len(cells); {
		j := i
		space := cells[i].r == ' '
		for j < len(cells) && (cells[j].r == ' ') == space {
			j++
		}
		word := cells[i:j]
		i = j
		w := srWidth(word)
		if space {
			if lineWidth > 0 && lineWidth+w <= width {
				line, lineWidth = append(line, word...), lineWidth+w
			}
			continue
		}
		if lineWidth+w > width && lineWidth > 0 {
			flush()
		}
		for srWidth(word) > width {
			cut, cw := 0, 0
			for cut < len(word) && cw+cellWidth(string(word[cut].r)) <= width {
				cw += cellWidth(string(word[cut].r))
				cut++
			}
			if cut == 0 {
				cut = 1
			}
			line = append(line, word[:cut]...)
			flush()
			word = word[cut:]
		}
		line, lineWidth = append(line, word...), lineWidth+srWidth(word)
	}
	if len(line) > 0 || len(lines) == 0 {
		flush()
	}
	return lines
}

func isBlockStart(line string) bool {
	return mdHeading.MatchString(line) || mdFence.MatchString(line) || mdRule.MatchString(line) ||
		mdQuote.MatchString(line) || mdList.MatchString(line)
}

func mdBlocks(src []string, base, width int) []mdLine {
	var out []mdLine
	blank := func() {
		if len(out) > 0 && len(out[len(out)-1].cells) > 0 || len(out) > 0 && out[len(out)-1].rule {
			out = append(out, mdLine{start: base, end: base})
		}
	}
	emit := func(first, rest []sr, content []sr, from, to, avail int) {
		for k, l := range wrapCells(content, avail) {
			prefix := rest
			if k == 0 {
				prefix = first
			}
			out = append(out, mdLine{cells: append(append([]sr{}, prefix...), l...), start: base + from, end: base + to})
		}
	}
	para, paraFrom := []string{}, 0
	flushPara := func(to int) {
		if len(para) == 0 {
			return
		}
		emit(nil, nil, inlineMarkdown(strings.Join(para, " "), baseStyle), paraFrom, to, width)
		para = nil
	}
	for i := 0; i < len(src); i++ {
		line := strings.TrimRight(src[i], " \t")
		switch {
		case strings.TrimSpace(line) == "":
			flushPara(i - 1)
			blank()
		case strings.HasPrefix(strings.TrimSpace(line), "<!--"):
			for i < len(src) && !strings.Contains(src[i], "-->") {
				i++
			}
		case mdFence.MatchString(line):
			flushPara(i - 1)
			m := mdFence.FindStringSubmatch(line)
			fence, lang, from := m[1], strings.ToLower(m[2]), i
			var code []string
			for i++; i < len(src) && !strings.HasPrefix(strings.TrimSpace(src[i]), fence[:3]); i++ {
				code = append(code, strings.ReplaceAll(src[i], "\t", "    "))
			}
			out = append(out, codeBlock(code, lang, base+from, base+min(i, len(src)-1), width)...)
		case mdHeading.MatchString(line):
			flushPara(i - 1)
			m := mdHeading.FindStringSubmatch(line)
			level := len(m[1])
			style := headingStyle
			switch level {
			case 2:
				style = baseStyle.Foreground(th.yellow).Bold(true)
			case 3:
				style = baseStyle.Foreground(th.aqua).Bold(true)
			case 4, 5, 6:
				style = baseStyle.Bold(true)
			}
			emit(nil, nil, inlineMarkdown(m[2], style), i, i, width)
			if level <= 2 {
				out = append(out, mdLine{start: base + i, end: base + i, rule: true})
			}
		case len(para) > 0 && mdSetextOne.MatchString(line):
			heading := inlineMarkdown(strings.Join(para, " "), headingStyle)
			emit(nil, nil, heading, paraFrom, i, width)
			out = append(out, mdLine{start: base + paraFrom, end: base + i, rule: true})
			para = nil
		case len(para) > 0 && mdSetextTwo.MatchString(line) && !mdList.MatchString(line):
			heading := inlineMarkdown(strings.Join(para, " "), baseStyle.Foreground(th.yellow).Bold(true))
			emit(nil, nil, heading, paraFrom, i, width)
			out = append(out, mdLine{start: base + paraFrom, end: base + i, rule: true})
			para = nil
		case mdRule.MatchString(line):
			flushPara(i - 1)
			out = append(out, mdLine{start: base + i, end: base + i, rule: true})
		case mdQuote.MatchString(line):
			flushPara(i - 1)
			from := i
			var inner []string
			for ; i < len(src) && mdQuote.MatchString(src[i]); i++ {
				inner = append(inner, mdQuote.FindStringSubmatch(src[i])[1])
			}
			i--
			bar := styled("▎ ", baseStyle.Foreground(th.border))
			for _, l := range mdBlocks(inner, base+from, width-2) {
				if len(l.cells) == 0 && !l.rule && !l.fill {
					out = append(out, l)
					continue
				}
				if l.rule || l.fill {
					out = append(out, l)
					continue
				}
				for k := range l.cells {
					l.cells[k].s = l.cells[k].s.Foreground(quoteColor(l.cells[k].s))
				}
				l.cells = append(append([]sr{}, bar...), l.cells...)
				out = append(out, l)
			}
		case strings.Contains(line, "|") && i+1 < len(src) && mdTableSep.MatchString(src[i+1]) && strings.Contains(src[i+1], "-"):
			flushPara(i - 1)
			from := i
			rows := [][]string{splitRow(line)}
			for i += 2; i < len(src) && strings.Contains(src[i], "|") && strings.TrimSpace(src[i]) != ""; i++ {
				rows = append(rows, splitRow(src[i]))
			}
			i--
			out = append(out, tableLines(rows, base+from, base+i)...)
		case mdList.MatchString(line):
			flushPara(i - 1)
			m := mdList.FindStringSubmatch(line)
			level := len(strings.ReplaceAll(m[1], "\t", "  ")) / 2
			from := i
			text := m[3]
			for i+1 < len(src) && strings.TrimSpace(src[i+1]) != "" && !isBlockStart(src[i+1]) {
				i++
				text += " " + strings.TrimSpace(src[i])
			}
			bullet := []string{"•", "◦", "▪"}[min(level, 2)]
			if m[2][0] >= '0' && m[2][0] <= '9' {
				bullet = m[2]
			}
			markStyle := baseStyle.Foreground(th.accent)
			switch {
			case strings.HasPrefix(text, "[ ] "):
				bullet, text = "☐", text[4:]
			case strings.HasPrefix(text, "[x] "), strings.HasPrefix(text, "[X] "):
				bullet, text = "☑", text[4:]
			}
			indent := strings.Repeat("  ", level)
			first := styled(indent+bullet+" ", markStyle)
			rest := styled(indent+strings.Repeat(" ", len([]rune(bullet))+1), baseStyle)
			emit(first, rest, inlineMarkdown(text, baseStyle), from, i, width-srWidth(first))
		default:
			if len(para) == 0 {
				paraFrom = i
			}
			para = append(para, strings.TrimSpace(line))
		}
	}
	flushPara(len(src) - 1)
	for len(out) > 0 && len(out[len(out)-1].cells) == 0 && !out[len(out)-1].rule {
		out = out[:len(out)-1]
	}
	return out
}

func quoteColor(s cellStyle) terminalColor {
	if s.fg == th.fg {
		return th.muted
	}
	return s.fg
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	var cells []string
	for _, c := range strings.Split(line, "|") {
		cells = append(cells, strings.TrimSpace(c))
	}
	return cells
}

func tableLines(rows [][]string, from, to int) []mdLine {
	columns := 0
	for _, r := range rows {
		columns = max(columns, len(r))
	}
	widths := make([]int, columns)
	cells := make([][][]sr, len(rows))
	for i, r := range rows {
		style := baseStyle
		if i == 0 {
			style = headingStyle
		}
		for c := 0; c < columns; c++ {
			text := ""
			if c < len(r) {
				text = r[c]
			}
			cells[i] = append(cells[i], inlineMarkdown(text, style))
			widths[c] = max(widths[c], srWidth(cells[i][c]))
		}
	}
	border := baseStyle.Foreground(th.border)
	var out []mdLine
	for i := range rows {
		var line []sr
		for c := 0; c < columns; c++ {
			if c > 0 {
				line = append(line, styled(" │ ", border)...)
			}
			line = append(line, cells[i][c]...)
			line = append(line, styled(strings.Repeat(" ", widths[c]-srWidth(cells[i][c])), baseStyle)...)
		}
		out = append(out, mdLine{cells: line, start: from + i, end: from + i})
		if i == 0 {
			var sep []sr
			for c := 0; c < columns; c++ {
				if c > 0 {
					sep = append(sep, styled("─┼─", border)...)
				}
				sep = append(sep, styled(strings.Repeat("─", widths[c]), border)...)
			}
			out = append(out, mdLine{cells: sep, start: from, end: from})
		}
	}
	return out
}

func codeBlock(code []string, lang string, from, to, width int) []mdLine {
	bg := baseStyle.Background(th.menu)
	var out []mdLine
	label := lang
	out = append(out, mdLine{cells: styled(" "+label, bg.Foreground(th.muted)), start: from, end: from, fill: true})
	text := []rune(strings.Join(code, "\n"))
	var marks []highlight
	if _, ok := syntaxSpecs[lang]; ok {
		marks = highlightGeneric(text, lang)
	}
	offset := 0
	for i, l := range code {
		runes := []rune(l)
		cells := styled(" ", bg)
		for k, r := range runes {
			style := bg
			for _, h := range marks {
				if offset+k >= h.start && offset+k < h.end {
					style = bg.Foreground(h.style.fg)
					break
				}
			}
			cells = append(cells, sr{r, style})
		}
		out = append(out, mdLine{cells: cells, start: from + 1 + i, end: from + 1 + i, fill: true})
		offset += len(runes) + 1
	}
	out = append(out, mdLine{start: to, end: to, fill: true})
	return out
}

// inlineMarkdown styles emphasis, code, links and images in one paragraph.
func inlineMarkdown(s string, base cellStyle) []sr {
	return inlineRunes([]rune(s), base)
}

func findClose(r []rune, from int, delim string) int {
	d := []rune(delim)
	for i := from; i+len(d) <= len(r); i++ {
		if r[i] == '\\' {
			i++
			continue
		}
		match := true
		for k := range d {
			if r[i+k] != d[k] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func inlineRunes(r []rune, base cellStyle) []sr {
	var out []sr
	code := baseStyle.Background(th.menu).Foreground(th.orange)
	has := func(i int, s string) bool {
		d := []rune(s)
		if i+len(d) > len(r) {
			return false
		}
		for k := range d {
			if r[i+k] != d[k] {
				return false
			}
		}
		return true
	}
	for i := 0; i < len(r); {
		ch := r[i]
		switch {
		case ch == '\\' && i+1 < len(r) && (unicode.IsPunct(r[i+1]) || unicode.IsSymbol(r[i+1])):
			out = append(out, sr{r[i+1], base})
			i += 2
			continue
		case ch == '`':
			if end := findClose(r, i+1, "`"); end > i {
				out = append(out, styled(" "+strings.TrimSpace(string(r[i+1:end]))+" ", code)...)
				i = end + 1
				continue
			}
		case has(i, "**") || has(i, "__"):
			delim := string(r[i : i+2])
			if end := findClose(r, i+2, delim); end > i+2 {
				out = append(out, inlineRunes(r[i+2:end], base.Bold(true))...)
				i = end + 2
				continue
			}
		case has(i, "~~"):
			if end := findClose(r, i+2, "~~"); end > i+2 {
				out = append(out, inlineRunes(r[i+2:end], base.Foreground(th.muted))...)
				i = end + 2
				continue
			}
		case (ch == '*' || ch == '_') && i+1 < len(r) && r[i+1] != ' ':
			boundary := i == 0 || !(unicode.IsLetter(r[i-1]) || unicode.IsDigit(r[i-1]))
			if ch == '*' || boundary {
				if end := findClose(r, i+1, string(ch)); end > i+1 && r[end-1] != ' ' {
					out = append(out, inlineRunes(r[i+1:end], base.Foreground(th.typ))...)
					i = end + 1
					continue
				}
			}
		case ch == '!' && has(i, "!["), ch == '[':
			image := ch == '!'
			open := i
			if image {
				open++
			}
			if close := findClose(r, open+1, "]"); close > 0 && close+1 < len(r) && r[close+1] == '(' {
				if end := findClose(r, close+2, ")"); end > 0 {
					label := r[open+1 : close]
					if image {
						out = append(out, styled("▣ "+string(label), base.Foreground(th.muted))...)
					} else {
						link := baseStyle.Foreground(th.aqua)
						if base.bold {
							link = link.Bold(true)
						}
						out = append(out, inlineRunes(label, link)...)
					}
					i = end + 1
					continue
				}
			}
		case ch == '<':
			if end := findClose(r, i+1, ">"); end > i {
				tag := string(r[i+1 : end])
				if strings.Contains(tag, "://") {
					out = append(out, styled(tag, baseStyle.Foreground(th.aqua))...)
					i = end + 1
					continue
				}
				if strings.HasPrefix(tag, "br") {
					out = append(out, sr{' ', base})
					i = end + 1
					continue
				}
			}
		}
		out = append(out, sr{ch, base})
		i++
	}
	return out
}
