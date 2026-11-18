package editor

import (
	"testing"

	"github.com/zhuchops/loom/internal/theme"
)

func TestTerminalThemeUsesPalette(t *testing.T) {
	defer applyTheme(theme.All[0])
	applyTheme(theme.Terminal())
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
	defer applyTheme(theme.All[0])
	e := testEditor(t, "vim", "x")
	nord, _ := theme.Find("nord")
	e.setTheme(nord)
	if semanticColor("keyword") != int32(terminalColor(nord.Red)) || baseStyle.GetBackground() != terminalColor(nord.BG) {
		t.Fatal("theme not applied")
	}
}
