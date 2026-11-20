package editor

import (
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
	clipboardCopy = func(string) bool { return true }
	clipboardPaste = func() (string, bool) { return "", false }
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
