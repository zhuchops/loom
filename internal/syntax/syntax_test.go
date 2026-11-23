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

func TestElixirHighlight(t *testing.T) {
	src := "defmodule MyApp.User do\n  @moduledoc \"\"\"\n  Docs.\n  \"\"\"\n  @spec run(map) :: :ok\n  def valid?(user), do: user |> check() |> done\n  defp done!(x), do: ~r/ab+/i\n  # note\nend\n"
	cases := []struct{ word, want string }{
		{"defmodule", "keyword"}, {"MyApp", "type"}, {"do", "keyword"},
		{"@moduledoc", "decorator"}, {"\"\"\"\n  Docs.\n  \"\"\"", "comment"},
		{"@spec", "decorator"}, {":ok", "constant"}, {"def", "keyword"},
		{"valid?", "function"}, {"do:", "property"}, {"check", "function"}, {"done", "function"},
		{"defp", "keyword"}, {"done!", "function"}, {"~r/ab+/i", "regexp"}, {"# note", "comment"}, {"end", "keyword"},
	}
	got := kinds(src, "elixir")
	for _, c := range cases {
		if got[c.word] != c.want {
			t.Errorf("%q is %q, want %q", c.word, got[c.word], c.want)
		}
	}
	for src, want := range map[string]string{
		"x = :\"quoted atom\"": ":\"quoted atom\"", "a = ?z": "?z", "n = 1_000.5": "1_000.5", "s = \"a\nb\"": "\"a\nb\"",
	} {
		if _, ok := kinds(src, "elixir")[want]; !ok {
			t.Errorf("%q: no token %q", src, want)
		}
	}
	if k := kinds("x = a::b", "elixir"); k[":"] != "" || k["::b"] != "" {
		t.Errorf("a double colon is not an atom: %v", k)
	}
	if open, _, ok := CommentSyntax("elixir"); !ok || open != "#" {
		t.Error("elixir comment toggle")
	}
}

func TestDingoHighlight(t *testing.T) {
	src := "enum Result {\n    Ok(value: int),\n}\nfunc run(a: int) -> Result {\n    x := f()? // note\n    match x {\n        None => 1,\n    }\n}\n"
	got := kinds(src, "dingo")
	for word, want := range map[string]string{
		"enum": "keyword", "match": "keyword", "func": "keyword", "Result": "type", "Ok": "function",
		"// note": "comment", "None": "constant", "run": "function",
	} {
		if got[word] != want {
			t.Errorf("%q is %q, want %q", word, got[word], want)
		}
	}
	if open, _, ok := CommentSyntax("dingo"); !ok || open != "//" {
		t.Error("dingo comment toggle")
	}
}
