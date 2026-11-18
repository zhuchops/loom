// Package markdown renders Markdown text into styled, wrapped terminal rows.
// It knows nothing about colors: every cell carries a Style naming its role.
package markdown

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/rivo/uniseg"

	"github.com/zhuchops/loom/internal/syntax"
)

// Role says what a cell is, so the caller can pick colors for it.
type Role uint8

const (
	Text Role = iota
	Muted
	H1
	H2
	H3
	H4
	Accent     // list bullets and markers
	Border     // quote bars, rules and table lines
	Emphasis   // italic text
	Link       // link text
	InlineCode // `code`
	CodeText   // fenced code; Kind picks the syntax color
	CodeLabel  // the language name above a fence
)

// Style is the look of one cell. Kind is a syntax token kind for CodeText.
type Style struct {
	Role Role
	Bold bool
	Kind string
}

// Cell is one styled rune.
type Cell struct {
	R rune
	S Style
}

// Line is one rendered row. Start and End are the source rows it came from,
// which lets a preview follow the editor cursor.
type Line struct {
	Cells      []Cell
	Start, End int
	Fill       bool // paint the whole row with the code background
	Rule       bool
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

// Render lays text out for the given width.
func Render(text []rune, width int) []Line {
	src := strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n")
	return mdBlocks(src, 0, max(8, width))
}

func styled(s string, style Style) []Cell {
	var out []Cell
	for _, r := range s {
		out = append(out, Cell{r, style})
	}
	return out
}

func cellsWidth(cells []Cell) int {
	w := 0
	for _, c := range cells {
		w += uniseg.StringWidth(string(c.R))
	}
	return w
}

// wrapCells breaks styled text on spaces, splitting words that are too long.
func wrapCells(cells []Cell, width int) [][]Cell {
	width = max(1, width)
	var lines [][]Cell
	var line []Cell
	lineWidth := 0
	flush := func() {
		for len(line) > 0 && line[len(line)-1].R == ' ' {
			line = line[:len(line)-1]
		}
		lines = append(lines, line)
		line, lineWidth = nil, 0
	}
	for i := 0; i < len(cells); {
		j := i
		space := cells[i].R == ' '
		for j < len(cells) && (cells[j].R == ' ') == space {
			j++
		}
		word := cells[i:j]
		i = j
		w := cellsWidth(word)
		if space {
			if lineWidth > 0 && lineWidth+w <= width {
				line, lineWidth = append(line, word...), lineWidth+w
			}
			continue
		}
		if lineWidth+w > width && lineWidth > 0 {
			flush()
		}
		for cellsWidth(word) > width {
			cut, cw := 0, 0
			for cut < len(word) && cw+uniseg.StringWidth(string(word[cut].R)) <= width {
				cw += uniseg.StringWidth(string(word[cut].R))
				cut++
			}
			if cut == 0 {
				cut = 1
			}
			line = append(line, word[:cut]...)
			flush()
			word = word[cut:]
		}
		line, lineWidth = append(line, word...), lineWidth+cellsWidth(word)
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

func mdBlocks(src []string, base, width int) []Line {
	var out []Line
	blank := func() {
		if len(out) > 0 && len(out[len(out)-1].Cells) > 0 || len(out) > 0 && out[len(out)-1].Rule {
			out = append(out, Line{Start: base, End: base})
		}
	}
	emit := func(first, rest []Cell, content []Cell, from, to, avail int) {
		for k, l := range wrapCells(content, avail) {
			prefix := rest
			if k == 0 {
				prefix = first
			}
			out = append(out, Line{Cells: append(append([]Cell{}, prefix...), l...), Start: base + from, End: base + to})
		}
	}
	para, paraFrom := []string{}, 0
	flushPara := func(to int) {
		if len(para) == 0 {
			return
		}
		emit(nil, nil, inlineMarkdown(strings.Join(para, " "), Style{}), paraFrom, to, width)
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
			style := Style{Role: []Role{H1, H1, H2, H3, H4, H4, H4}[level]}
			emit(nil, nil, inlineMarkdown(m[2], style), i, i, width)
			if level <= 2 {
				out = append(out, Line{Start: base + i, End: base + i, Rule: true})
			}
		case len(para) > 0 && mdSetextOne.MatchString(line):
			heading := inlineMarkdown(strings.Join(para, " "), Style{Role: H1})
			emit(nil, nil, heading, paraFrom, i, width)
			out = append(out, Line{Start: base + paraFrom, End: base + i, Rule: true})
			para = nil
		case len(para) > 0 && mdSetextTwo.MatchString(line) && !mdList.MatchString(line):
			heading := inlineMarkdown(strings.Join(para, " "), Style{Role: H2})
			emit(nil, nil, heading, paraFrom, i, width)
			out = append(out, Line{Start: base + paraFrom, End: base + i, Rule: true})
			para = nil
		case mdRule.MatchString(line):
			flushPara(i - 1)
			out = append(out, Line{Start: base + i, End: base + i, Rule: true})
		case mdQuote.MatchString(line):
			flushPara(i - 1)
			from := i
			var inner []string
			for ; i < len(src) && mdQuote.MatchString(src[i]); i++ {
				inner = append(inner, mdQuote.FindStringSubmatch(src[i])[1])
			}
			i--
			bar := styled("▎ ", Style{Role: Border})
			for _, l := range mdBlocks(inner, base+from, width-2) {
				if len(l.Cells) == 0 && !l.Rule && !l.Fill {
					out = append(out, l)
					continue
				}
				if l.Rule || l.Fill {
					out = append(out, l)
					continue
				}
				for k := range l.Cells {
					if l.Cells[k].S.Role == Text {
						l.Cells[k].S.Role = Muted
					}
				}
				l.Cells = append(append([]Cell{}, bar...), l.Cells...)
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
			markStyle := Style{Role: Accent}
			switch {
			case strings.HasPrefix(text, "[ ] "):
				bullet, text = "☐", text[4:]
			case strings.HasPrefix(text, "[x] "), strings.HasPrefix(text, "[X] "):
				bullet, text = "☑", text[4:]
			}
			indent := strings.Repeat("  ", level)
			first := styled(indent+bullet+" ", markStyle)
			rest := styled(indent+strings.Repeat(" ", len([]rune(bullet))+1), Style{})
			emit(first, rest, inlineMarkdown(text, Style{}), from, i, width-cellsWidth(first))
		default:
			if len(para) == 0 {
				paraFrom = i
			}
			para = append(para, strings.TrimSpace(line))
		}
	}
	flushPara(len(src) - 1)
	for len(out) > 0 && len(out[len(out)-1].Cells) == 0 && !out[len(out)-1].Rule {
		out = out[:len(out)-1]
	}
	return out
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

func tableLines(rows [][]string, from, to int) []Line {
	columns := 0
	for _, r := range rows {
		columns = max(columns, len(r))
	}
	widths := make([]int, columns)
	cells := make([][][]Cell, len(rows))
	for i, r := range rows {
		style := Style{}
		if i == 0 {
			style = Style{Role: H1}
		}
		for c := 0; c < columns; c++ {
			text := ""
			if c < len(r) {
				text = r[c]
			}
			cells[i] = append(cells[i], inlineMarkdown(text, style))
			widths[c] = max(widths[c], cellsWidth(cells[i][c]))
		}
	}
	border := Style{Role: Border}
	var out []Line
	for i := range rows {
		var line []Cell
		for c := 0; c < columns; c++ {
			if c > 0 {
				line = append(line, styled(" │ ", border)...)
			}
			line = append(line, cells[i][c]...)
			line = append(line, styled(strings.Repeat(" ", widths[c]-cellsWidth(cells[i][c])), Style{})...)
		}
		out = append(out, Line{Cells: line, Start: from + i, End: from + i})
		if i == 0 {
			var sep []Cell
			for c := 0; c < columns; c++ {
				if c > 0 {
					sep = append(sep, styled("─┼─", border)...)
				}
				sep = append(sep, styled(strings.Repeat("─", widths[c]), border)...)
			}
			out = append(out, Line{Cells: sep, Start: from, End: from})
		}
	}
	return out
}

func codeBlock(code []string, lang string, from, to, width int) []Line {
	var out []Line
	out = append(out, Line{Cells: styled(" "+lang, Style{Role: CodeLabel}), Start: from, End: from, Fill: true})
	text := []rune(strings.Join(code, "\n"))
	marks := syntax.Highlight(lang, "", text)
	offset := 0
	for i, l := range code {
		runes := []rune(l)
		cells := styled(" ", Style{Role: CodeText})
		for k, r := range runes {
			style := Style{Role: CodeText}
			for _, t := range marks {
				if offset+k >= t.Start && offset+k < t.End {
					style.Kind = t.Kind
					break
				}
			}
			cells = append(cells, Cell{r, style})
		}
		out = append(out, Line{Cells: cells, Start: from + 1 + i, End: from + 1 + i, Fill: true})
		offset += len(runes) + 1
	}
	out = append(out, Line{Start: to, End: to, Fill: true})
	return out
}

// inlineMarkdown styles emphasis, code, links and images in one paragraph.
func inlineMarkdown(s string, base Style) []Cell {
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

func inlineRunes(r []rune, base Style) []Cell {
	var out []Cell
	code := Style{Role: InlineCode}
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
			out = append(out, Cell{r[i+1], base})
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
				out = append(out, inlineRunes(r[i+2:end], withBold(base))...)
				i = end + 2
				continue
			}
		case has(i, "~~"):
			if end := findClose(r, i+2, "~~"); end > i+2 {
				out = append(out, inlineRunes(r[i+2:end], Style{Role: Muted, Bold: base.Bold})...)
				i = end + 2
				continue
			}
		case (ch == '*' || ch == '_') && i+1 < len(r) && r[i+1] != ' ':
			boundary := i == 0 || !(unicode.IsLetter(r[i-1]) || unicode.IsDigit(r[i-1]))
			if ch == '*' || boundary {
				if end := findClose(r, i+1, string(ch)); end > i+1 && r[end-1] != ' ' {
					out = append(out, inlineRunes(r[i+1:end], Style{Role: Emphasis, Bold: base.Bold})...)
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
						out = append(out, styled("▣ "+string(label), Style{Role: Muted, Bold: base.Bold})...)
					} else {
						link := Style{Role: Link, Bold: base.Bold}
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
					out = append(out, styled(tag, Style{Role: Link})...)
					i = end + 1
					continue
				}
				if strings.HasPrefix(tag, "br") {
					out = append(out, Cell{' ', base})
					i = end + 1
					continue
				}
			}
		}
		out = append(out, Cell{ch, base})
		i++
	}
	return out
}

func withBold(s Style) Style { s.Bold = true; return s }
