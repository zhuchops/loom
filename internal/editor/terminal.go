package editor

import (
	"context"
	"os"
	"os/signal"
	"slices"
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
	model.attach()
	e.begin(explicitFile)
	_, err := tea.NewProgram(model, tea.WithoutSignalHandler()).Run()
	cancel()
	e.hub.finish()
	return err
}

// begin restores the previous session and starts background services.
func (e *editor) begin(explicitFile bool) {
	e.loadRecent()
	e.restoreSession(explicitFile)
	if !e.noProject && !explicitFile {
		rememberProject(e.root)
	}
	if !explicitFile {
		e.showDashboard()
	} else {
		e.scheduleLanguage()
		e.refreshGit()
	}
	e.startRecovery()
}

// finish stops background services and saves state for the next start.
func (e *editor) finish() {
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
}

type dragTickMsg struct{}

type animTickMsg struct{}

func dragTick() tea.Cmd {
	return tea.Tick(60*time.Millisecond, func(time.Time) tea.Msg { return dragTickMsg{} })
}

type terminalModel struct {
	dragAt      pointerEvent
	dragTicking bool
	animTicking bool
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

	waiting    map[*editor]bool // sessions whose events are being waited for
	switched   bool
	switchedAt time.Time
}

type completionReady uint64
type languageReady int
type gitReady struct{}
type editorCallback func()
type diskTick struct{}
type flashExpired struct{}

// sessionEvent is work reported by a session, which may be out of sight.
type sessionEvent struct {
	e   *editor
	msg tea.Msg
}

// attach has the model follow the session that is shown.
func (m *terminalModel) attach() {
	m.editor.hub.onShow = func(e *editor) {
		m.editor, m.switched, m.switchedAt = e, true, time.Now()
	}
}

// The commands only wait for events. All editor mutations run in Update on
// the same goroutine as input handling, including async service callbacks.
func (m *terminalModel) waitForSystem() tea.Msg {
	select {
	case <-m.ctx.Done():
		return nil
	case <-m.signals:
		return tea.QuitMsg{}
	case <-m.ticks:
		return diskTick{}
	}
}

func (m *terminalModel) waitForSession(e *editor) tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		var msg tea.Msg
		select {
		case <-ctx.Done():
			return nil
		case <-e.closed:
			return nil
		case token := <-e.completionWake:
			msg = completionReady(token)
		case token := <-e.languageWake:
			msg = languageReady(token)
		case <-e.gitWake:
			msg = gitReady{}
		case fn := <-e.async:
			msg = editorCallback(fn)
		}
		return sessionEvent{e, msg}
	}
}

// waitForNewSessions starts listening to sessions that have just started.
func (m *terminalModel) waitForNewSessions() tea.Cmd {
	if m.ctx == nil || m.editor.hub == nil {
		return nil
	}
	if m.waiting == nil {
		m.waiting = map[*editor]bool{}
	}
	var cmds []tea.Cmd
	for _, e := range m.editor.hub.live {
		if !m.waiting[e] {
			m.waiting[e] = true
			cmds = append(cmds, m.waitForSession(e))
		}
	}
	for e := range m.waiting {
		if !slices.Contains(m.editor.hub.live, e) {
			delete(m.waiting, e)
		}
	}
	return tea.Batch(cmds...)
}

// settle finishes a switch once the input that caused it has been handled.
func (m *terminalModel) settle() tea.Cmd {
	if !m.switched {
		return nil
	}
	m.switched = false
	old := &m.view
	m.view = renderer{screen: old.screen, native: old.native, leftDown: old.leftDown}
	if m.cancelSignature != nil {
		m.cancelSignature()
	}
	if m.cancelHover != nil {
		m.cancelHover()
	}
	m.signatureToken++
	m.hoverToken++
	m.signatureTarget, m.hoverTarget = signatureTarget{}, hoverTarget{}
	return m.waitForNewSessions()
}

func (m *terminalModel) Init() tea.Cmd {
	return tea.Batch(m.waitForSystem, m.waitForNewSessions())
}

// serve handles what one session reported, shown or not.
func (m *terminalModel) serve(ev sessionEvent) tea.Cmd {
	e := ev.e
	select {
	case <-e.closed:
		return nil
	default:
	}
	switch msg := ev.msg.(type) {
	case completionReady:
		if uint64(msg) == e.completion.token {
			e.requestCompletion(1, e.completion.trigger)
		}
	case languageReady:
		if int(msg) == e.languageToken {
			e.refreshLanguage()
		}
	case gitReady:
		e.refreshGit()
	case editorCallback:
		msg()
	}
	return m.waitForSession(e)
}

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
	case animTickMsg:
		m.animTicking = false
	case dragTickMsg:
		next = nil
		if m.view.dragging && m.view.leftDown {
			// Keep scrolling while the pointer rests at the edge.
			m.view.mouse(e, m.dragAt)
			next = dragTick()
		} else {
			m.dragTicking = false
		}
	case sessionEvent:
		next = m.serve(msg)
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
	case diskTick:
		e.checkDisk()
		e.watchTree()
		e.watchGit()
		if e.hub != nil {
			e.hub.saveRecovery()
		}
		next = m.waitForSystem
	}
	if m.switched {
		next = tea.Batch(next, m.settle())
		e, flash = m.editor, m.editor.surroundFlash.until
	}
	if e.quit {
		return m, tea.Quit
	}
	if expiry := e.surroundFlash.until; expiry != flash {
		timer := tea.Tick(time.Until(expiry), func(time.Time) tea.Msg { return flashExpired{} })
		next = tea.Batch(next, timer)
	}
	if e.animating() && !m.animTicking {
		m.animTicking = true
		next = tea.Batch(next, tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg { return animTickMsg{} }))
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
	if k.Mod&tea.ModSuper != 0 && k.Mod&^tea.ModSuper == 0 && (k.Code == tea.KeyBackspace || k.Code == tea.KeyDelete) {
		// Command+Delete in the desktop app clears to the start or end of a field.
		if k.Code == tea.KeyBackspace {
			return keyInput{key: "super+backspace"}
		}
		return keyInput{key: "super+delete"}
	}
	if k.Mod&(tea.ModMeta|tea.ModHyper|tea.ModSuper) != 0 {
		return keyInput{}
	}
	if k.Mod&tea.ModAlt != 0 {
		// Alt arrives as an escape prefix. Esc then a key is the same bytes, so
		// only keys that mean something on their own are bound.
		k.Mod &^= tea.ModAlt
		if inner := terminalKey(k); inner.key != "" && inner.key != "esc" && inner.paste == "" {
			return keyInput{key: "alt+" + inner.key, shift: inner.shift}
		}
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
		if k.Mod&tea.ModCtrl != 0 {
			return keyInput{key: "ctrl+backspace"}
		}
		return keyInput{key: "backspace"}
	case tea.KeyDelete:
		if k.Mod&tea.ModCtrl != 0 {
			return keyInput{key: "ctrl+delete"}
		}
		return keyInput{key: "delete"}
	case tea.KeyPgUp:
		return keyInput{key: "pageup"}
	case tea.KeyPgDown:
		return keyInput{key: "pagedown"}
	case tea.KeyLeft, tea.KeyRight, tea.KeyUp, tea.KeyDown, tea.KeyHome, tea.KeyEnd:
		name := (tea.Key{Code: k.Code}).Keystroke()
		if k.Mod&tea.ModCtrl != 0 {
			name = "ctrl+" + name
		}
		return keyInput{key: name, shift: k.Mod&tea.ModShift != 0}
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
