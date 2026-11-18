package editor

import (
	"strings"
	"testing"
)

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
