package editor

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

func runTerminal(e *editor, explicitFile bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	model := &terminalModel{
		editor: e,
		view:   renderer{screen: newTerminalFrame(0, 0)},
		ctx:    ctx, signals: signals, ticks: ticker.C,
	}
	e.restoreSession(explicitFile)
	if !explicitFile {
		e.showDashboard()
	} else {
		e.scheduleLanguage()
		e.refreshGit()
	}
	e.startRecovery()
	_, err := tea.NewProgram(model, tea.WithoutSignalHandler()).Run()
	cancel()
	e.invalidateCompletion()
	e.closeServers()
	e.saveSession()
	if e.recovery != nil {
		if e.discardRecovery {
			_ = os.Remove(e.recovery.path)
		} else {
			_ = e.saveRecovery()
		}
	}
	return err
}

type dragTickMsg struct{}

func dragTick() tea.Cmd {
	return tea.Tick(60*time.Millisecond, func(time.Time) tea.Msg { return dragTickMsg{} })
}

type terminalModel struct {
	dragAt      pointerEvent
	dragTicking bool
	editor      *editor
	view        renderer
	ctx         context.Context
	signals     <-chan os.Signal
	ticks       <-chan time.Time

	signatureTarget signatureTarget
	signatureToken  uint64
	cancelSignature context.CancelFunc
	hoverTarget     hoverTarget
	hoverToken      uint64
	cancelHover     context.CancelFunc
}

type completionReady uint64
type languageReady int
type gitReady struct{}
type editorCallback func()
type diskTick struct{}
type flashExpired struct{}

// The command only waits for events. All editor mutations run in Update on
// the same goroutine as input handling, including async service callbacks.
func (m *terminalModel) waitForEditor() tea.Msg {
	e := m.editor
	select {
	case <-m.ctx.Done():
		return nil
	case <-m.signals:
		return tea.QuitMsg{}
	case <-m.ticks:
		return diskTick{}
	case token := <-e.completionWake:
		return completionReady(token)
	case token := <-e.languageWake:
		return languageReady(token)
	case <-e.gitWake:
		return gitReady{}
	case fn := <-e.async:
		return editorCallback(fn)
	}
}

func (m *terminalModel) Init() tea.Cmd { return m.waitForEditor }

func (m *terminalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	e := m.editor
	flash := e.surroundFlash.until
	var next tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.view.clearDiagnosticHover()
		m.view.screen.Resize(max(0, msg.Width), max(0, msg.Height))
	case tea.KeyPressMsg:
		m.view.clearDiagnosticHover()
		if key := terminalKey(tea.Key(msg)); key != (keyInput{}) {
			e.handle(key)
		}
	case tea.PasteMsg:
		m.view.clearDiagnosticHover()
		if msg.Content != "" {
			e.handle(keyInput{paste: msg.Content})
		}
	case tea.MouseMsg:
		if m.view.screen.Width() >= 4 && m.view.screen.Height() >= 5 {
			p := terminalMouse(msg, m.view.leftDown)
			m.view.mouse(e, p)
			m.dragAt = p
			if m.view.dragging && p.leftDown && !m.dragTicking {
				m.dragTicking = true
				next = dragTick()
			}
		}
	case dragTickMsg:
		next = nil
		if m.view.dragging && m.view.leftDown {
			// Keep scrolling while the pointer rests at the edge.
			m.view.mouse(e, m.dragAt)
			next = dragTick()
		} else {
			m.dragTicking = false
		}
	case completionReady:
		if uint64(msg) == e.completion.token {
			e.requestCompletion(1, e.completion.trigger)
		}
		next = m.waitForEditor
	case signatureReady:
		next = m.requestSignature(uint64(msg))
	case hoverReady:
		next = m.requestHover(uint64(msg))
	case hoverResult:
		if msg.token == m.hoverToken && diagnosticHoverAllowed(e) {
			d := &m.view.diagnosticHover
			d.message = d.diagnostic
			if msg.content != "" {
				if d.message != "" {
					d.message += "\n\nDocumentation:\n"
				}
				d.message += msg.content
			}
		}
	case signatureResult:
		if msg.token == m.signatureToken && msg.session.ctx.Err() == nil {
			e.signature = msg.hint
		}
	case languageReady:
		if int(msg) == e.languageToken {
			e.refreshLanguage()
		}
		next = m.waitForEditor
	case gitReady:
		e.refreshGit()
		next = m.waitForEditor
	case editorCallback:
		msg()
		next = m.waitForEditor
	case diskTick:
		e.checkDisk()
		if err := e.saveRecovery(); err != nil {
			e.notify("Recovery: " + err.Error())
		}
		next = m.waitForEditor
	}
	if e.quit {
		return m, tea.Quit
	}
	if expiry := e.surroundFlash.until; expiry != flash {
		timer := tea.Tick(time.Until(expiry), func(time.Time) tea.Msg { return flashExpired{} })
		next = tea.Batch(next, timer)
	}
	if e.clipboardSet {
		e.clipboardSet = false
		next = tea.Batch(next, tea.SetClipboard(e.clipboardOut))
	}
	return m, tea.Batch(next, m.scheduleSignature(), m.scheduleHover())
}

func (m *terminalModel) View() tea.View {
	f := m.view.screen
	m.view.draw(m.editor)
	view := tea.NewView(f.Render())
	view.AltScreen = true
	view.MouseMode = tea.MouseModeAllMotion
	if f.cursorVisible {
		view.Cursor = tea.NewCursor(f.cursorX, f.cursorY)
		view.Cursor.Blink = false
		if f.cursorShape == cursorBar {
			view.Cursor.Shape = tea.CursorBar
		}
	}
	return view
}

// Match the names used in Loom configs. Text takes priority for shifted
// printable keys so a binding like "space N" still receives capital N.
func terminalKey(k tea.Key) keyInput {
	if k.Mod&(tea.ModAlt|tea.ModMeta|tea.ModHyper|tea.ModSuper) != 0 {
		return keyInput{}
	}
	switch k.Code {
	case tea.KeyEscape:
		return keyInput{key: "esc"}
	case tea.KeyEnter:
		return keyInput{key: "enter"}
	case tea.KeyTab:
		if k.Mod&tea.ModShift != 0 {
			return keyInput{key: "shift+tab"}
		}
		return keyInput{key: "tab"}
	case tea.KeyBackspace:
		return keyInput{key: "backspace"}
	case tea.KeyDelete:
		return keyInput{key: "delete"}
	case tea.KeyPgUp:
		return keyInput{key: "pageup"}
	case tea.KeyPgDown:
		return keyInput{key: "pagedown"}
	case tea.KeyLeft, tea.KeyRight, tea.KeyUp, tea.KeyDown, tea.KeyHome, tea.KeyEnd:
		return keyInput{key: (tea.Key{Code: k.Code}).Keystroke()}
	case tea.KeySpace, 0:
		if k.Mod&tea.ModCtrl != 0 || k.Code == 0 {
			return keyInput{key: "ctrl+space"}
		}
		return keyInput{key: "space"}
	}
	if k.Mod&tea.ModCtrl != 0 {
		if unicode.IsLetter(k.Code) || unicode.IsPunct(k.Code) || unicode.IsSymbol(k.Code) || unicode.IsDigit(k.Code) {
			return keyInput{key: "ctrl+" + string(unicode.ToLower(k.Code))}
		}
		return keyInput{}
	}
	if k.Text != "" {
		if utf8.RuneCountInString(k.Text) > 1 {
			return keyInput{paste: k.Text}
		}
		return keyInput{key: k.Text}
	}
	if k.Mod&tea.ModShift != 0 && k.ShiftedCode != 0 {
		k.Code = k.ShiftedCode
	}
	if unicode.IsPrint(k.Code) {
		return keyInput{key: string(k.Code)}
	}
	return keyInput{}
}

func terminalMouse(msg tea.MouseMsg, leftDown bool) pointerEvent {
	event := msg.Mouse()
	p := pointerEvent{x: event.X, y: event.Y, shift: event.Mod&tea.ModShift != 0, leftDown: leftDown}
	switch msg.(type) {
	case tea.MouseClickMsg:
		switch event.Button {
		case tea.MouseLeft:
			p.leftDown = true
		case tea.MouseRight:
			p.button = mouseRight
		case tea.MouseMiddle:
			p.button = mouseMiddle
		}
	case tea.MouseReleaseMsg:
		if event.Button == tea.MouseLeft || event.Button == tea.MouseNone {
			p.leftDown = false
		}
	case tea.MouseMotionMsg:
		p.leftDown = event.Button == tea.MouseLeft
	case tea.MouseWheelMsg:
		switch event.Button {
		case tea.MouseWheelUp:
			p.button = wheelUp
		case tea.MouseWheelDown:
			p.button = wheelDown
		case tea.MouseWheelLeft:
			p.button = wheelLeft
		case tea.MouseWheelRight:
			p.button = wheelRight
		}
	}
	return p
}
