package main

import "testing"

func kindsAt(src, lang string) map[string]int32 {
	out := map[string]int32{}
	r := []rune(src)
	for _, h := range highlightGeneric(r, lang) {
		out[string(r[h.start:h.end])] = int32(h.style.GetForeground())
	}
	return out
}

func TestMoreLanguagesHighlight(t *testing.T) {
	cases := []struct {
		lang, src, word string
		want            int32
	}{
		{"java", "public class Foo { }", "public", int32(th.red)},
		{"java", "int x = 1; // note", "// note", int32(th.comment)},
		{"python", "def run():\n    return 1 // 2", "def", int32(th.red)},
		{"python", "s = \"\"\"a\nb\"\"\"\nx = 1", "\"\"\"a\nb\"\"\"", int32(th.yellow)},
		{"asm", "start:\n  mov rax, 1 ; set", "mov", int32(th.red)},
		{"asm", "start:\n  mov rax, 1 ; set", "rax", int32(th.purple)},
		{"asm", "start:\n  mov rax, 1 ; set", "start", int32(th.accent)},
		{"asm", ".section .text", ".section", int32(th.red)},
		{"kotlin", "fun main() {}", "fun", int32(th.red)},
		{"markdown", "don't panic", "don't panic", -1},
	}
	for _, c := range cases {
		got, ok := kindsAt(c.src, c.lang)[c.word]
		if c.want == -1 {
			if ok {
				t.Errorf("%s: %q should not be highlighted", c.lang, c.word)
			}
			continue
		}
		if !ok || got != c.want {
			t.Errorf("%s: %q color %x want %x (found %v)", c.lang, c.word, got, c.want, ok)
		}
	}
}

func TestLanguageForNamesAndExtensions(t *testing.T) {
	cfg := defaultConfig()
	for path, want := range map[string]string{"A.java": "java", "boot.asm": "asm", "x.S": "asm", "Dockerfile": "dockerfile", "Makefile": "makefile", "main.py": "python", "a.kt": "kotlin"} {
		if got := cfg.languageFor(path); got != want {
			t.Errorf("%s: %s want %s", path, got, want)
		}
	}
	if open, _, ok := commentSyntax("asm"); !ok || open != ";" {
		t.Error("asm comment")
	}
}
