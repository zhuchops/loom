package main

import "testing"

func TestThemesAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, th := range themes {
		if seen[th.name] {
			t.Fatalf("duplicate theme %s", th.name)
		}
		seen[th.name] = true
		if (th.bg == th.fg && !th.terminal) || th.red == th.bg || th.accent == th.bg {
			t.Fatalf("%s has unreadable colors", th.name)
		}
	}
	for _, name := range []string{"everforest", "gruvbox", "nord", "dracula", "tokyo-night", "catppuccin", "one-dark", "monokai", "solarized-dark", "solarized-light", "terminal"} {
		if !seen[name] {
			t.Fatalf("missing theme %s", name)
		}
	}
}

func TestTerminalThemeUsesPalette(t *testing.T) {
	defer applyTheme(themes[0])
	applyTheme(terminalTheme())
	style := baseStyle.Foreground(th.red).uvStyle()
	if style.Bg != nil || style.Fg == nil || semanticColor("keyword") != int32(ansiColor(1)) {
		t.Fatalf("terminal theme should use default background and palette colors: %+v", style)
	}
	f := newTerminalFrame(4, 1)
	f.Put(0, 0, "x", 1, baseStyle.Foreground(th.red))
	_, got, _ := f.Get(0, 0)
	if got.fg != ansiColor(1) || got.bg != defaultFlag {
		t.Fatalf("round trip %x %x", got.fg, got.bg)
	}
}

func TestSetThemeRefreshesColors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	defer applyTheme(themes[0])
	e := testEditor(t, "vim", "x")
	nord, _ := findTheme("nord")
	e.setTheme(nord)
	if semanticColor("keyword") != int32(nord.red) || baseStyle.GetBackground() != nord.bg {
		t.Fatal("theme not applied")
	}
}
