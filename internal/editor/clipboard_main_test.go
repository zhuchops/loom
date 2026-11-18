package editor

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	clipboardCopy = func(string) bool { return true }
	clipboardPaste = func() (string, bool) { return "", false }
	os.Exit(m.Run())
}
