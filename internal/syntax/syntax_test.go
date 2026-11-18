package syntax

import "testing"

func kinds(src, lang string) map[string]string {
	out := map[string]string{}
	r := []rune(src)
	for _, t := range Highlight(lang, "x", r) {
		out[string(r[t.Start:t.End])] = t.Kind
	}
	return out
}

func TestHighlightKinds(t *testing.T) {
	cases := []struct{ lang, src, word, want string }{
		{"java", "public class Foo { }", "public", "keyword"},
		{"java", "int x = 1; // note", "// note", "comment"},
		{"python", "def run():\n    return 1 // 2", "def", "keyword"},
		{"python", "s = \"\"\"a\nb\"\"\"\nx = 1", "\"\"\"a\nb\"\"\"", "string"},
		{"asm", "start:\n  mov rax, 1 ; set", "mov", "keyword"},
		{"asm", "start:\n  mov rax, 1 ; set", "rax", "constant"},
		{"asm", "start:\n  mov rax, 1 ; set", "start", "function"},
		{"asm", ".section .text", ".section", "keyword"},
		{"kotlin", "fun main() {}", "fun", "keyword"},
		{"go", "package main\nfunc run() {}", "run", "function"},
		{"markdown", "don't panic", "don't panic", ""},
	}
	for _, c := range cases {
		if got := kinds(c.src, c.lang)[c.word]; got != c.want {
			t.Errorf("%s: %q is %q, want %q", c.lang, c.word, got, c.want)
		}
	}
}

func TestCommentSyntax(t *testing.T) {
	if open, _, ok := CommentSyntax("asm"); !ok || open != ";" {
		t.Error("asm comment")
	}
	if _, _, ok := CommentSyntax("json"); ok {
		t.Error("json has no comments")
	}
}
