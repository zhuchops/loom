package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type span struct{ start, end int }
type edit struct {
	span
	text string
}
type snapshot struct {
	text   []rune
	cursor int
	tags   []int
	pairs  map[int]int
}

type buffer struct {
	savedExists, external             bool
	observed                          os.FileInfo
	tags                              []int
	pairs                             map[int]int // Automatically inserted closer offset to opener offset.
	mergeUndo                         bool
	lastInsert                        time.Time
	semantic, lexical                 []highlight
	semanticVersion, highlightVersion int
	semanticPath, highlightPath       string
	diagnostics                       []diagnostic

	path      string
	dashboard bool
	// preview marks the preview tab, which the next file opened for a look
	// replaces. Editing it keeps it.
	preview     bool
	text        []rune
	cursor      int
	anchor      int
	visualLine  bool
	visualBlock bool
	marks       map[rune]int
	lines       []int
	version     int
	saved       string
	undo, redo  []snapshot
	top, left   int
	folds       foldState
	// wantCol is the column vertical moves aim for. It is only valid while the
	// cursor still sits at wantAt, where the last vertical move left it.
	wantCol, wantAt int
	wantSet         bool
}

func loadBuffer(path string) (*buffer, error) {
	b := &buffer{anchor: -1, version: 1}
	if path != "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			abs = resolved
		}
		if info, err := os.Stat(abs); err == nil && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("expected a regular file")
		}
		raw, err := os.ReadFile(abs)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if !utf8.Valid(raw) || strings.ContainsRune(string(raw), 0) {
			return nil, fmt.Errorf("expected a UTF-8 text file")
		}
		b.path, b.text, b.saved = abs, []rune(string(raw)), string(raw)
		b.savedExists = err == nil
		b.observed, _ = os.Lstat(abs)
	}
	b.indexLines()
	return b, nil
}

func (b *buffer) indexLines() {
	b.lines = append(b.lines[:0], 0)
	for i, r := range b.text {
		if r == '\n' {
			b.lines = append(b.lines, i+1)
		}
	}
}

func (b *buffer) rowAt(offset int) int {
	lo, hi := 0, len(b.lines)
	for lo+1 < hi {
		mid := (lo + hi) / 2
		if b.lines[mid] <= offset {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

func (b *buffer) lineEnd(row int) int {
	if row+1 == len(b.lines) {
		return len(b.text)
	}
	end := b.lines[row+1] - 1
	if end > b.lines[row] && b.text[end-1] == '\r' {
		end--
	}
	return end
}

func (b *buffer) newline() string {
	for i, r := range b.text {
		if r == '\n' {
			if i > 0 && b.text[i-1] == '\r' {
				return "\r\n"
			}
			break
		}
	}
	return "\n"
}

func (b *buffer) selection() span {
	if b.anchor < 0 {
		return span{b.cursor, min(len(b.text), b.cursor+1)}
	}
	if b.visualLine {
		first, last := b.rowAt(min(b.anchor, b.cursor)), b.rowAt(max(b.anchor, b.cursor))
		return b.lineSpan(first, last)
	}
	return span{min(b.anchor, b.cursor), min(len(b.text), max(b.anchor, b.cursor)+1)}
}

func (b *buffer) lineSpan(first, last int) span {
	end := len(b.text)
	if last+1 < len(b.lines) {
		end = b.lines[last+1]
	}
	return span{b.lines[first], end}
}

func (b *buffer) replace(s span, value string) {
	b.replaceAll([]edit{{s, value}}, s.start+len([]rune(value)))
}

// Edits are in document order and refer to the same original snapshot.
func (b *buffer) replaceAll(edits []edit, cursor int) {
	if !b.mergeUndo || len(b.undo) == 0 {
		b.undo = pushHistory(b.undo, b.snapshot())
	}
	b.redo = nil
	var result []rune
	var tags []int
	last := 0
	for _, e := range edits {
		result = append(result, b.text[last:e.start]...)
		inserted := []rune(e.text)
		result = append(result, inserted...)
		if b.tags != nil {
			tags = append(tags, b.tags[last:e.start]...)
			tags = append(tags, make([]int, len(inserted))...)
		}
		last = e.end
	}
	result = append(result, b.text[last:]...)
	if b.tags != nil {
		tags = append(tags, b.tags[last:]...)
		if tags == nil {
			tags = []int{}
		}
	}
	shiftMarks(b.marks, edits)
	b.restore(snapshot{result, cursor, tags, shiftAutoPairs(b.pairs, edits)})
}

func pushHistory(history []snapshot, entry snapshot) []snapshot {
	if len(history) == 128 {
		history = history[1:]
	}
	return append(history, entry)
}

func (b *buffer) snapshot() snapshot { return snapshot{b.text, b.cursor, b.tags, b.pairs} }

func (b *buffer) restore(s snapshot) {
	b.tags = s.tags
	b.pairs = s.pairs
	b.text, b.cursor, b.anchor = s.text, min(s.cursor, len(s.text)), -1
	b.visualLine, b.visualBlock = false, false
	b.version++
	b.diagnostics = nil
	b.indexLines()
}

func (b *buffer) history(redo bool) {
	from, to := &b.undo, &b.redo
	if redo {
		from, to = to, from
	}
	if len(*from) == 0 {
		return
	}
	last := len(*from) - 1
	*to = pushHistory(*to, b.snapshot())
	b.restore((*from)[last])
	*from = (*from)[:last]
}

func (b *buffer) dirty() bool { return string(b.text) != b.saved }

// isPreview tells whether b is still a preview tab; an edit keeps it for good.
func (b *buffer) isPreview() bool {
	if b.preview && b.dirty() {
		b.preview = false
	}
	return b.preview
}

func (b *buffer) save(expected *diskState) error {
	if b.path == "" {
		return fmt.Errorf("use :w PATH to name this buffer")
	}
	mode := os.FileMode(0644)
	if info, err := os.Stat(b.path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("expected a regular file")
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(b.path), ".loom-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.WriteString(string(b.text)); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if expected != nil {
		d, err := readDisk(b.path)
		if err != nil {
			return err
		}
		if d != *expected {
			return errDiskChanged
		}
	}
	if err := os.Rename(tmp.Name(), b.path); err != nil {
		return err
	}
	b.saved = string(b.text)
	b.savedExists, b.external = true, false
	b.observed, _ = os.Lstat(b.path)
	return nil
}

func wordClass(r rune) int {
	switch {
	case unicode.IsSpace(r):
		return 0
	case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '_':
		return 1
	default:
		return 2
	}
}

func motion(b *buffer, name string, at int) int {
	row := b.rowAt(at)
	start, end := b.lines[row], b.lineEnd(row)
	last := max(start, end-1)
	switch name {
	case "left":
		return max(start, at-1)
	case "right":
		return min(last, at+1)
	case "up", "down":
		target := b.visibleStep(row, -1)
		if name == "down" {
			target = b.visibleStep(row, 1)
		}
		col := at - start
		if b.wantSet && b.wantAt == at {
			col = b.wantCol
		}
		first := b.lines[target]
		return min(first+col, max(first, b.lineEnd(target)-1))
	case "home":
		return start
	case "first-nonblank":
		for start < end && (b.text[start] == ' ' || b.text[start] == '\t') {
			start++
		}
		return min(start, last)
	case "end":
		return last
	case "top":
		return 0
	case "bottom":
		return b.lines[len(b.lines)-1]
	case "word":
		if at == len(b.text) {
			return at
		}
		class := wordClass(b.text[at])
		for at < len(b.text) && wordClass(b.text[at]) == class {
			at++
		}
		for at < len(b.text) && wordClass(b.text[at]) == 0 {
			at++
		}
	case "back-word":
		at = max(0, at-1)
		for at > 0 && wordClass(b.text[at]) == 0 {
			at--
		}
		class := 0
		if at < len(b.text) {
			class = wordClass(b.text[at])
		}
		for at > 0 && wordClass(b.text[at-1]) == class {
			at--
		}
	case "word-end":
		if at+1 >= len(b.text) {
			return at
		}
		at++
		for at+1 < len(b.text) && wordClass(b.text[at]) == 0 {
			at++
		}
		class := wordClass(b.text[at])
		for at+1 < len(b.text) && wordClass(b.text[at+1]) == class {
			at++
		}
	}
	return at
}

func delimiters(r rune) (rune, rune) {
	for _, pair := range []string{"()", "[]", "{}", "<>"} {
		runes := []rune(pair)
		if r == runes[0] || r == runes[1] {
			return runes[0], runes[1]
		}
	}
	return r, r
}

func enclosing(b *buffer, r rune, target span) (span, bool) {
	return enclosingPair(b, r, target, false)
}

func enclosingPair(b *buffer, r rune, target span, strictlyOuter bool) (span, bool) {
	open, close := delimiters(r)
	var starts []int
	var best span
	found := false
	for i, ch := range b.text {

		if open == close && ch == open {
			backslashes := 0
			for j := i - 1; j >= 0 && b.text[j] == '\\'; j-- {
				backslashes++
			}
			if backslashes%2 == 1 {
				continue
			}
		}
		if ch == open && (open != close || len(starts) == 0) {
			starts = append(starts, i)
			continue
		}
		if ch == close && len(starts) != 0 {
			start := starts[len(starts)-1]
			starts = starts[:len(starts)-1]
			covers := start <= target.start && i+1 >= target.end
			if covers && (!strictlyOuter || start < target.start || i+1 > target.end) && (!found || i+1-start < best.end-best.start) {
				best, found = span{start, i + 1}, true
			}
		}
	}
	return best, found
}

func textObject(b *buffer, r rune, around bool) (span, bool) {
	if r == 'w' {
		if b.cursor >= len(b.text) {
			return span{}, false
		}
		s := span{b.cursor, b.cursor + 1}
		class := wordClass(b.text[b.cursor])
		if class == 0 {
			return span{}, false
		}
		for s.start > 0 && wordClass(b.text[s.start-1]) == class {
			s.start--
		}
		for s.end < len(b.text) && wordClass(b.text[s.end]) == class {
			s.end++
		}
		if around {
			for s.end < len(b.text) && (b.text[s.end] == ' ' || b.text[s.end] == '\t') {
				s.end++
			}
		}
		return s, true
	}
	s, ok := enclosing(b, r, b.selection())
	if ok && !around {
		s.start++
		s.end--
	}
	return s, ok
}
