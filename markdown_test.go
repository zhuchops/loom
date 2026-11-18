package main

import (
	"strings"
	"testing"
)

func mdText(lines []mdLine) []string {
	var out []string
	for _, l := range lines {
		var sb strings.Builder
		for _, c := range l.cells {
			sb.WriteRune(c.r)
		}
		out = append(out, sb.String())
	}
	return out
}

func TestRenderMarkdownBlocks(t *testing.T) {
	src := "# Title\n\nSome **bold** and `code` with [a link](http://x).\n\n- one\n- [x] done\n\n> quoted\n\n| a | b |\n|---|---|\n| 1 | 22 |\n\n```go\nfunc main() {}\n```\n"
	got := strings.Join(mdText(renderMarkdown([]rune(src), 40)), "\n")
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
	lines := mdText(renderMarkdown([]rune("one two three four five six seven"), 12))
	for _, l := range lines {
		if len([]rune(l)) > 12 {
			t.Fatalf("line too long %q", l)
		}
	}
	if len(lines) < 3 {
		t.Fatalf("expected wrapping, got %v", lines)
	}
}

func TestMarkdownPreviewUpdatesLive(t *testing.T) {
	e := testEditor(t, "vim", "")
	path := fixture(t, e.root, "doc.md", "# Hello\n")
	e.open(path)
	press(e, "space m")
	if len(e.layout.leaves()) != 2 || e.activePane.preview {
		t.Fatalf("preview split not created")
	}
	v := renderer{screen: newTerminalFrame(100, 12)}
	v.draw(e)
	if !strings.Contains(screenDump(v.screen), "Hello") {
		t.Fatal("preview missing heading")
	}
	press(e, "A")
	for _, r := range " brave world" {
		e.handle(keyInput{key: string(r)})
	}
	v.draw(e)
	dump := screenDump(v.screen)
	if strings.Count(dump, "brave world") < 2 {
		t.Fatalf("preview did not follow typing:\n%s", dump)
	}
	press(e, "esc")
	press(e, "ctrl+w l")
	if e.activePane.preview {
		t.Fatal("preview took focus")
	}
	press(e, "space m")
	if len(e.layout.leaves()) != 1 {
		t.Fatal("toggle did not close preview")
	}
}

func screenDump(f *terminalFrame) string {
	var sb strings.Builder
	w, h := f.Size()
	for y := 0; y < h; y++ {
		for x := 0; x < w; {
			s, _, width := f.Get(x, y)
			if s == "" {
				s = " "
			}
			sb.WriteString(s)
			x += max(1, width)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
