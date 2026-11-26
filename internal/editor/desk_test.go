package editor

import (
	"context"
	"os"
	"testing"

	"github.com/zhuchops/loom/internal/theme"
)

// Two windows on one desk each keep the theme, folder and tab width their
// editor set, whichever ran last.
func TestDeskKeepsEachWindowsProcessState(t *testing.T) {
	e, dirs := sessionFixture(t, "one", "two")
	t.Cleanup(func() { applyTheme(baselineTheme()) })
	d := newDesk()

	_ = os.Chdir(dirs[0])
	applyTheme(theme.All[0])
	tabWidth = 4
	first := d.bridge(e, context.Background(), false, nil)

	second := d.aside(func() *Bridge {
		other := blankEditor(t)
		_ = os.Chdir(dirs[1])
		applyTheme(theme.All[1])
		tabWidth = 8
		return d.bridge(other, context.Background(), false, nil)
	})
	if d.active != first || th.name != theme.All[0].Name || tabWidth != 4 || cwd(t) != dirs[0] {
		t.Fatalf("aside left %s, tab %d, in %s", th.name, tabWidth, cwd(t))
	}
	d.enter(second)
	if th.name != theme.All[1].Name || tabWidth != 8 || cwd(t) != dirs[1] {
		t.Fatalf("second window has %s, tab %d, in %s", th.name, tabWidth, cwd(t))
	}
	tabWidth = 2 // the second window's editor changes its own
	d.enter(first)
	if th.name != theme.All[0].Name || tabWidth != 4 || cwd(t) != dirs[0] {
		t.Fatalf("first window has %s, tab %d, in %s", th.name, tabWidth, cwd(t))
	}
	d.enter(second)
	if tabWidth != 2 {
		t.Fatalf("second window lost its change: tab %d", tabWidth)
	}
}
