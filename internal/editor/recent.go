package editor

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const recentLimit = 50

func recentPath() string { return filepath.Join(filepath.Dir(configPath()), "recent") }

func (e *editor) loadRecent() {
	raw, err := os.ReadFile(recentPath())
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if line != "" {
			e.recent = append(e.recent, line)
		}
	}
}

// touchRecent moves path to the front and remembers the list across runs.
func (e *editor) touchRecent(path string) {
	if path == "" {
		return
	}
	e.recent = slices.DeleteFunc(e.recent, func(p string) bool { return p == path })
	e.recent = append([]string{path}, e.recent...)
	if len(e.recent) > recentLimit {
		e.recent = e.recent[:recentLimit]
	}
	if err := os.MkdirAll(filepath.Dir(recentPath()), 0o755); err == nil {
		_ = os.WriteFile(recentPath(), []byte(strings.Join(e.recent, "\n")+"\n"), 0o644)
	}
}

func (e *editor) recentOptions(skipCurrent bool) []option {
	var out []option
	for _, path := range e.recent {
		if skipCurrent && path == e.b.path {
			continue
		}
		if _, err := os.Stat(path); err != nil || !within(e.root, path) {
			continue
		}
		rel, _ := filepath.Rel(e.root, path)
		out = append(out, option{rel, "recent", func() { e.openPreview(path) }, e.filePreview(path, -1, 0, false, nil)})
	}
	return out
}

func (e *editor) recentMenu() {
	m := &menu{title: "Recent Files", empty: "No recent files in this project", options: e.recentOptions(false)}
	e.openMenu(m)
}

func (e *editor) jumpsMenu() {
	m := &menu{title: "Jump List", empty: "The jump list is empty"}
	back := e.jumps.back
	for i := len(back) - 1; i >= 0; i-- {
		p := back[i]
		if p.row >= len(p.buffer.lines) {
			continue
		}
		name := p.buffer.path
		if rel, err := filepath.Rel(e.root, name); err == nil && name != "" && within(e.root, name) {
			name = rel
		} else if name == "" {
			name = "Untitled"
		}
		text := strings.TrimSpace(string(p.buffer.text[p.buffer.lines[p.row]:p.buffer.lineEnd(p.row)]))
		m.options = append(m.options, option{name + ":" + itoa(p.row+1), text, func() {
			from := e.jumpPoint()
			e.switchBuffer(p.buffer)
			row := min(p.row, len(e.b.lines)-1)
			e.b.cursor = min(e.b.lines[row]+p.col, e.b.lineEnd(row))
			e.clampModalCursor()
			e.scrollLock = false
			e.rememberJump(from)
		}, nil})
	}
	e.openMenu(m)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for ; n > 0; n /= 10 {
		d = append([]byte{byte('0' + n%10)}, d...)
	}
	return string(d)
}
