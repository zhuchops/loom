package editor

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

type register struct {
	text     string
	linewise bool
	tags     []int
}

const historyLimit = 30

func (e *editor) registerRegisterCommands() {
	e.add("register-prefix", "Use Register", func(e *editor) { e.pending = pending{kind: "register"} })
	e.add("registers", "Registers and Clipboard History", func(e *editor) { e.registersMenu() })
}

func validRegister(r rune) bool {
	return unicode.IsLetter(r) && r < 128 || unicode.IsDigit(r) || strings.ContainsRune(`"+*_-`, r)
}

// storeRegister saves yanked text under the register chosen with "x, if any.
// "_ discards it, "+ and "* use the system clipboard, a-z keep it to themselves
// and A-Z append.
func (e *editor) storeRegister(text string, linewise bool, tags []int, yank bool) {
	pick := e.regPick
	e.regPick = 0
	if pick == '_' {
		return
	}
	if e.regs == nil {
		e.regs = map[rune]register{}
	}
	if unicode.IsUpper(pick) {
		pick = unicode.ToLower(pick)
		old := e.regs[pick]
		text, linewise = old.text+text, linewise || old.linewise
	}
	switch {
	case unicode.IsLower(pick):
		e.regs[pick] = register{text, linewise, tags}
	case yank:
		e.regs['0'] = register{text, linewise, tags}
	default:
		for i := '9'; i > '1'; i-- {
			e.regs[i] = e.regs[i-1]
		}
		e.regs['1'] = register{text, linewise, tags}
	}
	e.register, e.linewise, e.registerTags = text, linewise, tags
	e.remember(text)
	if pick == 0 || pick == '"' || pick == '+' || pick == '*' {
		e.clipboardOut, e.clipboardSet = text, true
		clipboardCopy(text)
	}
}

func (e *editor) remember(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	e.history = slices.DeleteFunc(e.history, func(s string) bool { return s == text })
	e.history = append([]string{text}, e.history...)
	if len(e.history) > historyLimit {
		e.history = e.history[:historyLimit]
	}
}

// withRegister runs fn with the chosen register as the paste source, leaving
// the default register as it was.
func (e *editor) withRegister(fn func()) {
	pick := e.regPick
	e.regPick = 0
	if pick == 0 || pick == '"' || pick == '_' {
		if pick == '_' {
			return
		}
		fn()
		return
	}
	saved := register{e.register, e.linewise, e.registerTags}
	defer func() { e.register, e.linewise, e.registerTags = saved.text, saved.linewise, saved.tags }()
	switch {
	case pick == '+' || pick == '*':
		text, ok := clipboardPaste()
		if !ok || text == "" {
			e.notify("The system clipboard is empty")
			return
		}
		e.register, e.linewise, e.registerTags = text, false, nil
	default:
		r, ok := e.regs[unicode.ToLower(pick)]
		if !ok || r.text == "" {
			e.notify(fmt.Sprintf("Register %c is empty", pick))
			return
		}
		e.register, e.linewise, e.registerTags = r.text, r.linewise, r.tags
	}
	fn()
}

func (e *editor) registersMenu() {
	m := &menu{title: "Registers and Clipboard History", hidePreview: true, empty: "Nothing has been copied yet"}
	paste := func(text string, linewise bool) func() {
		return func() {
			e.regPick = 0
			e.register, e.linewise, e.registerTags = text, linewise, nil
			e.paste(true)
		}
	}
	show := func(s string) string {
		s = strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", "⏎")
		if r := []rune(s); len(r) > 60 {
			s = string(r[:60]) + "…"
		}
		return s
	}
	for _, name := range []rune("0123456789abcdefghijklmnopqrstuvwxyz") {
		if r, ok := e.regs[name]; ok && r.text != "" {
			m.options = append(m.options, option{show(r.text), `"` + string(name), paste(r.text, r.linewise), nil})
		}
	}
	for i, text := range e.history {
		m.options = append(m.options, option{show(text), fmt.Sprintf("history %d", i+1), paste(text, strings.HasSuffix(text, "\n")), nil})
	}
	e.openMenu(m)
}
