package editor

import (
	"slices"
	"strings"
	"unicode/utf8"
)

var namedChips = []string{"Space", "Esc", "Enter", "Tab", "Backspace", "Delete", "Home", "End", "Pageup", "Pagedown", "Left", "Right", "Up", "Down", "←", "→", "↑", "↓", "↵"}

// isKeys tells key hints ("Space q  :close", "Ctrl+P") from words.
func isKeys(hint string) bool {
	if hint == "" || len(hint) > 48 {
		return false
	}
	for _, group := range strings.Split(hint, "  ") {
		group = strings.TrimSpace(group)
		if len(group) > 1 && group[0] == ':' {
			continue
		}
		for _, k := range strings.Fields(group) {
			switch {
			case utf8.RuneCountInString(k) == 1, slices.Contains(namedChips, k):
			case strings.HasPrefix(k, "Ctrl+"), strings.HasPrefix(k, "Alt+"), strings.HasPrefix(k, "Shift+"):
			case strings.ContainsAny(k, "⌘⇧⌥⌃"):
			case len(k) <= 2 && !strings.ContainsAny(k, "aeiou"):
			default:
				return false
			}
		}
	}
	return true
}

type pickerKind struct {
	prompt, files, keys, command bool
}

func kindOfPicker(m *menu) pickerKind {
	return pickerKind{
		prompt:  len(m.options) == 0 && m.submit != nil && m.suggestions == nil && m.empty == "",
		files:   m.title == "Go to File" || m.title == "Recent Files" || m.title == "Open Buffers",
		keys:    strings.HasPrefix(m.title, "Keybindings"),
		command: m.title == "Command Palette",
	}
}

func pickerPlaceholder(m *menu, k pickerKind) string {
	switch {
	case k.command:
		return "Type a command…"
	case k.files:
		return "Search files by name…"
	case m.title == "Commit message":
		return "Describe what changed…"
	case strings.HasPrefix(m.title, "Search Project"):
		return "Text to find in this project…"
	case strings.HasPrefix(m.title, "Open Project"):
		return "Search, or type a path…"
	case m.remove != nil:
		return "Switch to a project, or type a path…"
	case strings.HasPrefix(m.title, "Go to Line"):
		return "Line number…"
	case k.prompt:
		return "Type a value…"
	}
	return "Search…"
}

// matchRunes returns which runes of label a fuzzy query matches, in order.
func matchRunes(label, query string) map[int]bool {
	query = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(query), ":")))
	if query == "" {
		return nil
	}
	lower := strings.ToLower(label)
	if at := strings.Index(lower, query); at >= 0 {
		out, start := map[int]bool{}, utf8.RuneCountInString(lower[:at])
		for i := range utf8.RuneCountInString(query) {
			out[start+i] = true
		}
		return out
	}
	out, q, k := map[int]bool{}, []rune(query), 0
	for i, r := range []rune(lower) {
		if k < len(q) && r == q[k] {
			out[i] = true
			k++
		}
	}
	if k < len(q) {
		return nil
	}
	return out
}
