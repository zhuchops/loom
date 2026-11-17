package main

import "testing"

func TestAutoPairs(t *testing.T) {
	for _, tc := range []struct {
		name, before, keys, after string
	}{
		{"nested and skip", "", "i ( [ x ] )", "([x])"},
		{"delete empty", "", "i { backspace", ""},
		{"insert inside and delete", "", "i ( x backspace backspace", ""},
		{"existing closer stays literal", "()", "a )", "())"},
		{"existing empty pair stays literal", "()", "a backspace", ")"},
		{"apostrophe", "don", "A ' t", "don't"},
		{"escaped quote", "", "i \" \\ \" \"", "\"\\\"\""},
		{"surround selection", "abc", "v l l { esc", "{abc}"},
		{"undo pair", "", "i ( esc u", ""},
		{"redo restores pair", "", "i ( esc u ctrl+r a backspace", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t, "vim", tc.before)
			press(e, tc.keys)
			if got := string(e.b.text); got != tc.after {
				t.Fatalf("text = %q, want %q", got, tc.after)
			}
		})
	}
	e := testEditor(t, "vim", "")
	e.handle(keyInput{paste: "{"})
	e.cfg.AutoPairs = false
	press(e, "[")
	if got := string(e.b.text); got != "{[" {
		t.Fatalf("paste or disabled pairs changed the text: %q", got)
	}
}

func TestEnterBetweenBraces(t *testing.T) {
	for _, tc := range []struct{ path, before, keys, want string }{
		{"test.js", "  ", "A { enter x", "  {\n      x\n  }"},
		{"test.go", "\t{}\r\n", "^ a enter x", "\t{\r\n\t\tx\r\n\t}\r\n"},
		{"test.js", "  abc", "A enter x", "  abc\n  x"},
	} {
		e := testEditor(t, "vim", tc.before)
		e.b.path = tc.path
		press(e, tc.keys)
		if got := string(e.b.text); got != tc.want {
			t.Fatalf("%s %q: %q, want %q", tc.path, tc.keys, got, tc.want)
		}
	}
}
