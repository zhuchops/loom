package editor

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProblemJumpAndHoverDetails(t *testing.T) {
	e := testEditor(t, "vim", "")
	path := fixture(t, e.root, "bad.txt", "abc bad xyz\n")
	d := diagnostic{Range: lspRange{position{0, 4}, position{0, 7}}, Severity: 1, Message: "the complete error\nsecond line"}
	e.problems[path] = []diagnostic{d}
	e.showProblemList("Problems", []problemEntry{{path, d}})
	press(e, "enter")
	if e.menu != nil || e.b.path != path || e.b.cursor != 4 || len(e.b.diagnostics) != 1 {
		t.Fatal("problem did not jump to the diagnostic in the newly opened file")
	}
	press(e, "g h")
	if e.menu == nil || len(e.menu.options) != 1 || e.menu.options[0].label != "Error" || e.menu.selectedPreview().message != d.Message {
		t.Fatal("hover did not offer the diagnostic and its complete preview")
	}
	press(e, "enter")
	if e.notificationDetail == nil || e.notificationDetail.text != d.Message {
		t.Fatal("diagnostic details did not open")
	}
}

func TestHoverDocumentationHasOneOption(t *testing.T) {
	e := testEditor(t, "vim", "word")
	content := "first line\n\nsecond line"
	raw, _ := json.Marshal(map[string]string{"contents": content})
	e.showHover(raw, "")
	if len(e.menu.options) != 1 || e.menu.options[0].label != "Documentation" || strings.Contains(e.menu.options[0].label, "first") || e.menu.selectedPreview().message != content {
		t.Fatal("hover list should contain one documentation option with the full content in its preview")
	}
}
