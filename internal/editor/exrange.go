package editor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// exRange is a parsed `:[range]command` line.
type exRange struct {
	first, last int // 0-based rows
	explicit    bool
	name        string // s, g, v or d
	rest        string // text after the command name
}

// parseExRange recognises ranges with :s, :g, :v and :d. Anything else is left
// to the ordinary commands, so :set and :split are not caught.
func (e *editor) parseExRange(line string) (exRange, bool, error) {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ":"))
	b := e.b
	r := exRange{first: b.rowAt(b.cursor), last: b.rowAt(b.cursor)}
	rest := line
	if strings.HasPrefix(rest, "%") {
		r.first, r.last, r.explicit, rest = 0, len(b.lines)-1, true, rest[1:]
	} else if row, after, ok, err := e.exAddr(rest); err != nil {
		return r, false, err
	} else if ok {
		r.first, r.last, r.explicit, rest = row, row, true, after
		if strings.HasPrefix(rest, ",") {
			second, after, ok, err := e.exAddr(rest[1:])
			if err != nil {
				return r, false, err
			}
			if !ok {
				second, after = b.rowAt(b.cursor), rest[1:]
			}
			r.last, rest = second, after
		}
	}
	rest = strings.TrimLeft(rest, " ")
	name, tail := "", ""
	switch {
	case strings.HasPrefix(rest, "global!"):
		name, tail = "v", rest[len("global!"):]
	case strings.HasPrefix(rest, "vglobal"):
		name, tail = "v", rest[len("vglobal"):]
	case strings.HasPrefix(rest, "global"):
		name, tail = "g", rest[len("global"):]
	case strings.HasPrefix(rest, "g!"):
		name, tail = "v", rest[2:]
	case strings.HasPrefix(rest, "delete"):
		name, tail = "d", rest[len("delete"):]
	case strings.HasPrefix(rest, "substitute"):
		name, tail = "s", rest[len("substitute"):]
	case rest != "" && strings.ContainsRune("sgvd", rune(rest[0])):
		name, tail = rest[:1], rest[1:]
	default:
		return r, false, nil
	}
	if name == "d" {
		if strings.TrimSpace(tail) != "" {
			return r, false, nil
		}
	} else if tail == "" || unicode.IsLetter(rune(tail[0])) || unicode.IsDigit(rune(tail[0])) || tail[0] == ' ' {
		return r, false, nil
	}
	if r.first > r.last {
		r.first, r.last = r.last, r.first
	}
	if r.first < 0 || r.last >= len(b.lines) {
		return r, false, fmt.Errorf("Line must be between 1 and %d", len(b.lines))
	}
	if (name == "g" || name == "v") && !r.explicit {
		r.first, r.last = 0, len(b.lines)-1
	}
	r.name, r.rest = name, tail
	return r, true, nil
}

// exAddr reads one address: a number, ., $, 'x, with optional +N / -N offsets.
func (e *editor) exAddr(s string) (row int, rest string, ok bool, err error) {
	b := e.b
	i := 0
	switch {
	case s == "":
		return 0, s, false, nil
	case s[0] >= '0' && s[0] <= '9':
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		n, _ := strconv.Atoi(s[:i])
		row = n - 1
	case s[0] == '.':
		row, i = b.rowAt(b.cursor), 1
	case s[0] == '$':
		row, i = len(b.lines)-1, 1
	case s[0] == '\'' && len(s) >= 2:
		offset, found := b.marks[rune(s[1])]
		if !found {
			return 0, s, false, fmt.Errorf("Mark %c is not set", s[1])
		}
		row, i = b.rowAt(min(offset, len(b.text))), 2
	case s[0] == '+' || s[0] == '-':
		row = b.rowAt(b.cursor)
	default:
		return 0, s, false, nil
	}
	for i < len(s) && (s[i] == '+' || s[i] == '-') {
		sign := 1
		if s[i] == '-' {
			sign = -1
		}
		i++
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		n := 1
		if j > i {
			n, _ = strconv.Atoi(s[i:j])
		}
		row += sign * n
		i = j
	}
	return row, s[i:], true, nil
}

// splitDelimited cuts s at unescaped delimiters. `\/` becomes `/`.
func splitDelimited(s string, delim byte, max int) []string {
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s) && s[i+1] == delim:
			cur.WriteByte(delim)
			i++
		case s[i] == '\\' && i+1 < len(s):
			cur.WriteByte(s[i])
			cur.WriteByte(s[i+1])
			i++
		case s[i] == delim && len(parts) < max-1:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(s[i])
		}
	}
	return append(parts, cur.String())
}

// expandTemplate turns a Vim replacement (& \1 \n \t) into a Go template.
func expandTemplate(rep, newline string) string {
	var out strings.Builder
	for i := 0; i < len(rep); i++ {
		switch c := rep[i]; {
		case c == '$':
			out.WriteString("$$")
		case c == '&':
			out.WriteString("${0}")
		case c == '\\' && i+1 < len(rep):
			i++
			switch n := rep[i]; {
			case n >= '0' && n <= '9':
				out.WriteString("${" + string(n) + "}")
			case n == 'n' || n == 'r':
				out.WriteString(newline)
			case n == 't':
				out.WriteByte('\t')
			case n == '&':
				out.WriteByte('&')
			case n == '$':
				out.WriteString("$$")
			default:
				out.WriteByte(n)
			}
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

func compilePattern(pat, flags string) (*regexp.Regexp, error) {
	if pat == "" {
		if last := lastPattern; last != nil {
			return last, nil
		}
		return nil, fmt.Errorf("No previous pattern")
	}
	if strings.Contains(flags, "i") {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, fmt.Errorf("Pattern: %v", err)
	}
	lastPattern = re
	return re, nil
}

var lastPattern *regexp.Regexp

// exRangeLike reports whether line is a range command, for the palette.
func (e *editor) exRangeLike(line string) bool {
	_, ok, _ := e.parseExRange(line)
	return ok
}

// runExRange executes the command and reports whether the line was one.
func (e *editor) runExRange(line string) bool {
	r, ok, err := e.parseExRange(line)
	if err != nil {
		e.notify(err.Error())
		return true
	}
	if !ok {
		return false
	}
	rows := make([]int, 0, r.last-r.first+1)
	for row := r.first; row <= r.last; row++ {
		rows = append(rows, row)
	}
	switch r.name {
	case "d":
		e.deleteRows(rows)
	case "s":
		err = e.substituteCommand(rows, r.rest)
	case "g", "v":
		err = e.globalCommand(rows, r.rest, r.name == "v")
	}
	if err != nil {
		e.notify(err.Error())
	}
	return true
}

func (e *editor) deleteRows(rows []int) {
	b := e.b
	var edits []edit
	var removed strings.Builder
	for _, row := range rows {
		s := b.lineSpan(row, row)
		removed.WriteString(string(b.text[s.start:s.end]))
		if n := len(edits); n > 0 && edits[n-1].end == s.start {
			edits[n-1].end = s.end
		} else {
			edits = append(edits, edit{s, ""})
		}
	}
	text := removed.String()
	if !strings.HasSuffix(text, "\n") {
		text += b.newline()
	}
	e.yanking = false
	e.setRegister(text, true, nil)
	first := rows[0]
	b.replaceAll(edits, 0)
	e.normal()
	b.cursor = b.lines[min(first, len(b.lines)-1)]
	e.notify(fmt.Sprintf("%d line(s) deleted", len(rows)))
}

func (e *editor) substituteCommand(rows []int, rest string) error {
	parts := splitDelimited(rest[1:], rest[0], 3)
	if len(parts) < 2 {
		return fmt.Errorf("Use :s/pattern/replacement/[flags]")
	}
	flags := ""
	if len(parts) == 3 {
		flags = strings.TrimSpace(parts[2])
	}
	re, err := compilePattern(parts[0], flags)
	if err != nil {
		return err
	}
	return e.substitute(rows, re, parts[1], flags)
}

func (e *editor) substitute(rows []int, re *regexp.Regexp, rep, flags string) error {
	b := e.b
	template := expandTemplate(rep, b.newline())
	global := strings.Contains(flags, "g")
	var edits []edit
	subs, lines, lastRow := 0, 0, -1
	for _, row := range rows {
		s := span{b.lines[row], b.lineEnd(row)}
		line := string(b.text[s.start:s.end])
		var out string
		if global {
			n := len(re.FindAllStringIndex(line, -1))
			if n == 0 {
				continue
			}
			out, subs = re.ReplaceAllString(line, template), subs+n
		} else {
			loc := re.FindStringSubmatchIndex(line)
			if loc == nil {
				continue
			}
			out = line[:loc[0]] + string(re.ExpandString(nil, template, line, loc)) + line[loc[1]:]
			subs++
		}
		lines++
		lastRow = row
		if out != line {
			edits = append(edits, edit{s, out})
		}
	}
	if lines == 0 {
		return fmt.Errorf("Pattern not found")
	}
	e.normal()
	if len(edits) > 0 {
		b.replaceAll(edits, b.cursor)
	}
	b.cursor = b.lines[min(lastRow, len(b.lines)-1)]
	e.clampModalCursor()
	e.notify(fmt.Sprintf("%d substitution(s) on %d line(s)", subs, lines))
	return nil
}

// globalCommand runs :g/pattern/command (or :v for lines that do not match).
func (e *editor) globalCommand(rows []int, rest string, invert bool) error {
	parts := splitDelimited(rest[1:], rest[0], 2)
	re, err := compilePattern(parts[0], "")
	if err != nil {
		return err
	}
	b := e.b
	var hit []int
	for _, row := range rows {
		if re.MatchString(string(b.text[b.lines[row]:b.lineEnd(row)])) != invert {
			hit = append(hit, row)
		}
	}
	if len(hit) == 0 {
		return fmt.Errorf("Pattern not found")
	}
	cmd := ""
	if len(parts) == 2 {
		cmd = strings.TrimSpace(parts[1])
	}
	switch {
	case cmd == "":
		e.normal()
		b.cursor = b.lines[hit[0]]
		e.notify(fmt.Sprintf("%d line(s) match", len(hit)))
	case cmd == "d":
		e.deleteRows(hit)
	case cmd[0] == 's' && len(cmd) > 2 && !unicode.IsLetter(rune(cmd[1])):
		return e.substituteCommand(hit, cmd)
	default:
		return fmt.Errorf("Supported commands after :g are d and s/…/…/")
	}
	return nil
}
