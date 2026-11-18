package editor

import "testing"

func TestRegisterUsesSystemClipboard(t *testing.T) {
	var copied, system string
	clipboardCopy = func(s string) bool { copied = s; return true }
	clipboardPaste = func() (string, bool) { return system, system != "" }
	defer func() {
		clipboardCopy = func(string) bool { return true }
		clipboardPaste = func() (string, bool) { return "", false }
	}()

	e := testEditor(t, "vim", "one\ntwo\n")
	press(e, "y y")
	if copied != "one\n" || !e.clipboardSet {
		t.Fatalf("yank did not reach the system clipboard: %q", copied)
	}

	// Unchanged clipboard keeps the linewise paste.
	system = "one\n"
	press(e, "j p")
	if got := string(e.b.text); got != "one\ntwo\none\n" {
		t.Fatalf("linewise paste lost: %q", got)
	}

	// Text from another application is pasted as characters.
	system = "XY"
	e.b.cursor = 0
	press(e, "P")
	if got := string(e.b.text); got != "XYone\ntwo\none\n" {
		t.Fatalf("external text not pasted: %q", got)
	}
}
