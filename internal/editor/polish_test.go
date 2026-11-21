package editor

import (
	"strings"
	"testing"
	"time"
)

func TestPaneResizeAnimates(t *testing.T) {
	animationsOn = true
	defer func() { animationsOn = false }()
	p := &pane{vertical: true, ratio: 500}
	p.setRatio(800, true)
	if got := p.shownRatio(); got < 500 || got > 800 {
		t.Fatalf("not between the sizes: %d", got)
	}
	if p.ratio != 800 {
		t.Fatal("the target must be set at once")
	}
	time.Sleep(animDuration + 20*time.Millisecond)
	if got := p.shownRatio(); got != 800 {
		t.Fatalf("did not settle: %d", got)
	}
	p.setRatio(300, false)
	if p.shownRatio() != 300 || p.anim.active(time.Now()) {
		t.Fatal("a drag must not animate")
	}
}

func TestSplitGrowsIn(t *testing.T) {
	animationsOn = true
	defer func() { animationsOn = false }()
	e := testEditor(t, "vim", "x")
	e.splitPane(true, "")
	if !e.animating() {
		t.Fatal("a new split should animate")
	}
}

func TestPolishedScreen(t *testing.T) {
	e := testEditor(t, "vim", "func a() {\n\tif x {\n\t\ty()\n\t}\n}\n")
	e.showDashboard()
	e.recent = []string{}
	v := &renderer{screen: newTerminalFrame(100, 30)}
	v.draw(e)
	dump := func() string {
		var out strings.Builder
		for y := 0; y < 30; y++ {
			for x := 0; x < 100; x++ {
				c, _, _ := v.screen.Get(x, y)
				out.WriteString(c)
			}
			out.WriteByte('\n')
		}
		return out.String()
	}
	if !strings.Contains(dump(), "██╗") || !strings.Contains(dump(), "Command Palette") {
		t.Fatalf("dashboard logo or actions missing:\n%s", dump())
	}
	e.b = &buffer{}
	e2 := testEditor(t, "vim", "func a() {\n\t\tif x {\n\t\t\ty()\n\t\t}\n}\n")
	v.draw(e2)
	if !strings.Contains(dump(), "▏") {
		t.Fatalf("no indent guide drawn:\n%s", dump())
	}
	if !strings.Contains(dump(), "\uf108 NORMAL") {
		t.Fatalf("mode block missing from the statusline:\n%s", dump())
	}
}
