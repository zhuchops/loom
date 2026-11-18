package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zhuchops/loom/internal/theme"
)

// palette is the active theme in the form the renderer draws with.
type palette struct {
	name                                            string
	terminal                                        bool
	bg, fg, muted, accent                           terminalColor
	panel, menu, active, line, border               terminalColor
	sel, match, flash, badge                        terminalColor
	red, yellow, aqua, typ, orange, purple, comment terminalColor
}

var th palette

// ANSI colors follow the terminal's own palette. Bit 24 marks a palette index,
// bit 25 the terminal's default foreground or background.
const (
	ansiFlag    = terminalColor(theme.ANSI)
	defaultFlag = terminalColor(theme.Default)
)

func ansiColor(n int) terminalColor { return ansiFlag | terminalColor(n) }

func newPalette(t theme.Theme) palette {
	c := func(v theme.Color) terminalColor { return terminalColor(v) }
	return palette{
		name: t.Name, terminal: t.Terminal,
		bg: c(t.BG), fg: c(t.FG), muted: c(t.Muted), accent: c(t.Accent),
		panel: c(t.Panel), menu: c(t.Menu), active: c(t.Active), line: c(t.Line), border: c(t.Border),
		sel: c(t.Sel), match: c(t.Match), flash: c(t.Flash), badge: c(t.Badge),
		red: c(t.Red), yellow: c(t.Yellow), aqua: c(t.Aqua), typ: c(t.Type), orange: c(t.Orange), purple: c(t.Purple), comment: c(t.Comment),
	}
}

func init() { applyTheme(theme.All[0]) }

func applyTheme(t theme.Theme) {
	th = newPalette(t)
	p := th
	baseStyle = cellStyle{}.Background(p.bg).Foreground(p.fg)
	mutedStyle = baseStyle.Foreground(p.muted)
	headingStyle = baseStyle.Foreground(p.accent).Bold(true)
	menuStyle = baseStyle.Background(p.menu).Foreground(p.fg)
	activeStyle = menuStyle.Background(p.active).Bold(true)
	selectStyle = p.sel
	panelStyle = baseStyle.Background(p.panel)
	lineStyle = p.line
	borderStyle = panelStyle.Foreground(p.border)
}

func themeStatePath() string { return filepath.Join(filepath.Dir(configPath()), "theme") }

// loadTheme picks the configured theme, else the one chosen last time.
func loadTheme(cfg config) error {
	if cfg.Theme != "" {
		t, ok := theme.Find(cfg.Theme)
		if !ok {
			return fmt.Errorf("unknown theme %q", cfg.Theme)
		}
		applyTheme(t)
		return nil
	}
	raw, _ := os.ReadFile(themeStatePath())
	if t, ok := theme.Find(strings.TrimSpace(string(raw))); ok {
		applyTheme(t)
	}
	return nil
}

func (e *editor) setTheme(t theme.Theme) {
	applyTheme(t)
	for _, b := range e.buffers {
		// Highlights carry colors, so rebuild them for the new palette.
		b.semantic, b.lexical = nil, nil
		b.semanticVersion, b.highlightVersion = 0, 0
	}
	if err := os.MkdirAll(filepath.Dir(themeStatePath()), 0755); err == nil {
		os.WriteFile(themeStatePath(), []byte(t.Name+"\n"), 0644)
	}
	e.notify("Theme: " + t.Name)
}

func (e *editor) themeMenu() {
	m := &menu{title: "Color Theme", hidePreview: true}
	for _, t := range theme.All {
		hint := ""
		if t.Name == th.name {
			hint = "current"
		}
		if t.Terminal {
			hint = strings.TrimSpace(hint + " uses terminal colors")
		}
		m.options = append(m.options, option{t.Name, hint, func() { e.setTheme(t) }, nil})
	}
	e.openMenu(m)
}
