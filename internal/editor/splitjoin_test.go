package editor

import "testing"

func TestSplitJoinArguments(t *testing.T) {
	before := "  call(\"a,b\", nested(x, y), [1, 2])"
	e := testEditor(t, "vim", before)
	press(e, "f ( l g S")
	want := "  call(\n      \"a,b\",\n      nested(x, y),\n      [1, 2]\n  )"
	if got := string(e.b.text); got != want || e.b.text[e.b.cursor] != '"' {
		t.Fatalf("split = %q, cursor = %d", got, e.b.cursor)
	}
	press(e, "g S")
	if got := string(e.b.text); got != before {
		t.Fatalf("join = %q", got)
	}
	press(e, "u")
	if got := string(e.b.text); got != want {
		t.Fatalf("undo = %q", got)
	}
}

func TestSplitJoinGo(t *testing.T) {
	for _, tc := range []struct{ expr, split string }{
		{"f(a)", "f(\n\t\ta,\n\t)"},
		{"[]int{1, 2}", "[]int{\n\t\t1,\n\t\t2,\n\t}"},
		{"(a)", "(a)"},
		{"f(a, /* comment */ b)", "f(a, /* comment */ b)"},
	} {
		prefix := "package p\r\nfunc demo() {\r\n\t"
		suffix := "\r\n}\r\n"
		e := testEditor(t, "vim", prefix+tc.expr+suffix)
		e.b.path = "demo.go"
		e.b.cursor = len(prefix) + len(tc.expr)/2
		press(e, "g S")
		want := prefix
		for _, r := range tc.split {
			if r == '\n' {
				want += "\r"
			}
			want += string(r)
		}
		want += suffix
		if got := string(e.b.text); got != want {
			t.Fatalf("split %q: %q, want %q", tc.expr, got, want)
		}
		if tc.split != tc.expr {
			press(e, "g S")
			if got := string(e.b.text); got != prefix+tc.expr+suffix {
				t.Fatalf("join %q: %q", tc.expr, got)
			}
		}
	}
}
