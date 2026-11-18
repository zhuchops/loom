package markdown

import (
	"strings"
	"testing"
)

func mdText(lines []Line) []string {
	var out []string
	for _, l := range lines {
		var sb strings.Builder
		for _, c := range l.Cells {
			sb.WriteRune(c.R)
		}
		out = append(out, sb.String())
	}
	return out
}

func TestRenderMarkdownBlocks(t *testing.T) {
	src := "# Title\n\nSome **bold** and `code` with [a link](http://x).\n\n- one\n- [x] done\n\n> quoted\n\n| a | b |\n|---|---|\n| 1 | 22 |\n\n```go\nfunc main() {}\n```\n"
	got := strings.Join(mdText(Render([]rune(src), 40)), "\n")
	for _, want := range []string{"Title", "Some bold and  code  with a link.", "• one", "☑ done", "▎ quoted", "a │ b", "1 │ 22", "func main() {}"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "**") || strings.Contains(got, "](") {
		t.Errorf("markup leaked:\n%s", got)
	}
}

func TestRenderMarkdownWraps(t *testing.T) {
	lines := mdText(Render([]rune("one two three four five six seven"), 12))
	for _, l := range lines {
		if len([]rune(l)) > 12 {
			t.Fatalf("line too long %q", l)
		}
	}
	if len(lines) < 3 {
		t.Fatalf("expected wrapping, got %v", lines)
	}
}
