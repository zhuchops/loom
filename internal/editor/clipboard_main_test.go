package editor

import (
	"github.com/zhuchops/loom/internal/theme"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Never read or write the real config directory (themes, recent files).
	dir, err := os.MkdirTemp("", "loom-test-config")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.RemoveAll(dir)
	animationsOn = false
	// Tests assert colours, so pin the theme that was the default when they were written.
	if t, ok := theme.Find("everforest"); ok {
		applyTheme(t)
	}
	clipboardCopy = func(string) bool { return true }
	clipboardPaste = func() (string, bool) { return "", false }
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// baselineTheme is the palette the colour assertions in the tests were written against.
func baselineTheme() theme.Theme {
	t, _ := theme.Find("everforest")
	return t
}
