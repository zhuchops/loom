package editor

import (
	"fmt"
	"strconv"
	"strings"
)

// Ex commands that have no palette command of their own. Others are shown through their command.
var exCommands = []struct{ syntax, title string }{
	{"wq", "Save and Quit"}, {"q!", "Quit Without Saving"}, {"noh", "Clear Search Highlight"},
}

func exAddress(line string) (row int, ok bool) {
	line = strings.TrimSpace(line)
	for _, prefix := range []string{"goto ", "line "} {
		if strings.HasPrefix(line, prefix) {
			line = strings.TrimSpace(strings.TrimPrefix(line, prefix))
			break
		}
	}
	n, err := strconv.Atoi(line)
	return n, err == nil
}

func (e *editor) exAddress(line string) bool {
	row, ok := exAddress(line)
	if line == "$" {
		row, ok = len(e.b.lines), true
	}
	if !ok {
		return false
	}
	if row < 1 || row > len(e.b.lines) {
		e.notify(fmt.Sprintf("Line must be between 1 and %d", len(e.b.lines)))
		return true
	}
	from := e.jumpPoint()
	e.normal()
	e.b.cursor = e.b.lines[row-1]
	e.scrollLock = false
	e.rememberJump(from)
	e.message = ""
	return true
}

func (e *editor) paletteExact(line string) bool {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ":"))
	id, arg, _ := strings.Cut(line, " ")
	if _, ok := exAddress(line); ok || line == "$" || e.exRangeLike(line) {
		return true
	}
	switch id {
	case "w", "write", "e", "edit", "cd", "split", "sp", "vsplit", "vs", "tabnext", "tabprev", "tabn", "tabp", "tabfirst", "tablast", "tabmove", "tabclose", "set", "only", "close", "ExploreClose":
		return true
	case "q", "quit", "q!", "quit!", "wq", "qa", "qall", "qa!", "qall!", "noh", "nohlsearch", "vim":
		return strings.TrimSpace(arg) == ""
	}
	return e.commands[id].run != nil && arg == ""
}
