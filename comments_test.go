package main

import "testing"

func TestToggleComments(t *testing.T) {
	for _, tc := range []struct{ name, source, keys, want string }{
		{"test.go", "  one\n\ttwo\n", "2 g c c", "  // one\n\t// two\n"},
		{"test.py", "  # one\n  # two\n", "V j g c", "  one\n  two\n"},
		{"test.sql", "one\n-- two\n", "V j g c", "-- one\n-- -- two\n"},
		{"test.html", "  <p>hello</p>\n", "g c c", "  <!-- <p>hello</p> -->\n"},
		{"test.css", "  /* color: red */\n", "g c c", "  color: red\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := testEditor(t, "vim", tc.source)
			e.b.path = tc.name
			press(e, tc.keys)
			if got := string(e.b.text); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			press(e, "esc u")
			if got := string(e.b.text); got != tc.source {
				t.Fatalf("undo = %q", got)
			}
		})
	}
}

func TestUnknownCommentSyntaxDoesNotEdit(t *testing.T) {
	e := testEditor(t, "vim", "hello")
	e.b.path = "file.unknown"
	press(e, "g c c")
	if string(e.b.text) != "hello" || e.message == "" {
		t.Fatal("unknown syntax should report an error")
	}
}
