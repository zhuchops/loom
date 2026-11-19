package editor

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

type replaceHit struct {
	path     string
	row, col int
	old, new string
}

// projectReplacePrompt asks for what to find, then what to put instead.
func (e *editor) projectReplacePrompt() {
	e.openMenu(&menu{title: "Replace in Project · find (re: prefix for a regex)", submit: func(find string) {
		if find == "" {
			return
		}
		e.openMenu(&menu{title: "Replace “" + find + "” with", submit: func(with string) { e.projectReplacePreview(find, with) }})
	}})
}

// projectPattern is literal text unless prefixed with re:. Like VS Code the
// match ignores case unless the query has a capital.
func projectPattern(find string) (*regexp.Regexp, bool, error) {
	regex := strings.HasPrefix(find, "re:")
	pat := find
	if regex {
		pat = strings.TrimPrefix(find, "re:")
	} else {
		pat = regexp.QuoteMeta(find)
	}
	if !slices.ContainsFunc([]rune(find), unicode.IsUpper) {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	return re, regex, err
}

func (e *editor) walkProject(visit func(path string, text []byte)) {
	_ = filepath.WalkDir(e.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != e.root && (strings.HasPrefix(d.Name(), ".") || slices.Contains([]string{"node_modules", "vendor", "target"}, d.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
			return nil
		}
		text, err := os.ReadFile(path)
		if err != nil || bytes.IndexByte(text, 0) >= 0 {
			return nil
		}
		visit(path, text)
		return nil
	})
}

func replacementFor(re *regexp.Regexp, regex bool, with, line string) string {
	if !regex {
		with = strings.ReplaceAll(with, "$", "$$")
	}
	return re.ReplaceAllString(line, with)
}

func (e *editor) projectReplacePreview(find, with string) {
	re, regex, err := projectPattern(find)
	if err != nil {
		e.notify("Pattern: " + err.Error())
		return
	}
	m := &menu{title: "Replace in Project", empty: "No matches"}
	e.openMenu(m)
	go func() {
		var hits []replaceHit
		files := map[string]bool{}
		e.walkProject(func(path string, text []byte) {
			for row, line := range strings.Split(string(text), "\n") {
				if len(hits) >= 5000 {
					return
				}
				loc := re.FindStringIndex(line)
				if loc == nil {
					continue
				}
				files[path] = true
				hits = append(hits, replaceHit{path, row, len([]rune(line[:loc[0]])), line, replacementFor(re, regex, with, line)})
			}
		})
		e.async <- func() {
			if e.menu != m {
				return
			}
			if len(hits) > 0 {
				all := fmt.Sprintf("Replace all %d lines in %d files", len(hits), len(files))
				m.options = append(m.options, option{all, find + " → " + with, func() { e.applyProjectReplace(re, regex, with, files) }, nil})
			}
			for _, h := range hits {
				rel, _ := filepath.Rel(e.root, h.path)
				m.options = append(m.options, option{fmt.Sprintf("%s:%d", rel, h.row+1), strings.TrimSpace(h.new), func() { e.jumpTo(h.path, h.row, h.col, false) }, e.filePreview(h.path, h.row, h.col, false, nil)})
			}
			m.filter()
		}
	}()
}

// applyProjectReplace rewrites every file. Files open in a buffer are edited
// in place so the change can be undone and is not saved behind your back.
func (e *editor) applyProjectReplace(re *regexp.Regexp, regex bool, with string, files map[string]bool) {
	changedFiles, lines, open := 0, 0, 0
	for _, path := range sortedKeys(files) {
		var buf *buffer
		for _, b := range e.buffers {
			if b.path == path {
				buf = b
			}
		}
		if buf != nil {
			var edits []edit
			for row := range buf.lines {
				s := span{buf.lines[row], buf.lineEnd(row)}
				line := string(buf.text[s.start:s.end])
				if out := replacementFor(re, regex, with, line); out != line {
					edits = append(edits, edit{s, out})
					lines++
				}
			}
			if len(edits) > 0 {
				buf.replaceAll(edits, min(buf.cursor, len(buf.text)))
				changedFiles++
				open++
			}
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		parts := strings.Split(string(raw), "\n")
		changed := false
		for i, line := range parts {
			if out := replacementFor(re, regex, with, line); out != line {
				parts[i], changed = out, true
				lines++
			}
		}
		if !changed {
			continue
		}
		mode := os.FileMode(0o644)
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
		if err := os.WriteFile(path, []byte(strings.Join(parts, "\n")), mode); err != nil {
			e.notify(err.Error())
			return
		}
		changedFiles++
	}
	message := fmt.Sprintf("Replaced %d lines in %d files", lines, changedFiles)
	if open > 0 {
		message += fmt.Sprintf(" · %d open buffers changed, not saved", open)
	}
	e.notify(message)
	e.refreshGit()
}
