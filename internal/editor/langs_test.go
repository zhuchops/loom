package editor

import "testing"

func TestLanguageForNamesAndExtensions(t *testing.T) {
	cfg := defaultConfig()
	for path, want := range map[string]string{"A.java": "java", "boot.asm": "asm", "x.S": "asm", "Dockerfile": "dockerfile", "Makefile": "makefile", "main.py": "python", "a.kt": "kotlin"} {
		if got := cfg.languageFor(path); got != want {
			t.Errorf("%s: %s want %s", path, got, want)
		}
	}
}

func TestSyntaxTokensTakeThemeColors(t *testing.T) {
	b := &buffer{text: []rune("public class Foo {}")}
	got := highlightSyntax(b, "java")
	if len(got) == 0 || got[0].style.GetForeground() != th.red {
		t.Fatalf("keyword not colored by theme: %+v", got)
	}
}
