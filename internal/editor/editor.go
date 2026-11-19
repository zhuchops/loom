package editor

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type command struct {
	title string
	run   func(*editor)
}
type pending struct {
	kind, op string
	around   bool
	old      rune
	target   span
	version  int
	count    int
}
type keyInput struct{ key, paste string }

type editor struct {
	recovery                        *recoveryState
	discardRecovery                 bool
	jumps                           jumpHistory
	directory                       *directoryEditor
	searchVisible                   bool
	bar                             *findBar
	searchOpts                      searchOptions
	searchCache                     searchCache
	gitTimer                        *time.Timer
	gitWake                         chan struct{}
	tree                            fileTree
	layout, activePane              *pane
	zen, relativeNumbers            bool
	scrollLock                      bool
	git                             gitState
	gitToken                        int
	languageTimer                   *time.Timer
	languageWake                    chan int
	languageToken                   int
	languageRunning, languageQueued bool
	change                          changeState

	cfg                         config
	keys                        map[string]map[string]string
	commands                    map[string]command
	buffers                     []*buffer
	b                           *buffer
	mode, prefix, message, root string
	pending                     pending
	count                       int
	register                    string
	viewRows                    int
	regs                        map[rune]register
	regPick                     rune
	history                     []string
	yanking                     bool
	blockInsert                 *blockInsert
	linewise                    bool
	registerTags                []int
	clipboardOut                string
	clipboardSet                bool
	recording                   string
	macros                      map[string][]keyInput
	lastMacro                   string
	replaying                   bool
	menu                        *menu
	notifications               []notification
	notificationDetail          *notificationDetail
	notificationParent          *editor // Directory buffers report messages to the workspace editor.
	query                       string
	quit                        bool
	completion                  completionState
	signature                   signatureHint
	sessions                    map[string]*languageSession
	async                       chan func()
	completionWake              chan uint64
	pluginRunning               bool
	panel                       *sourceControl
	problems                    map[string][]diagnostic
	problemServers              map[string]*languageSession
	diff                        *diffView
	lastFind                    characterFind
	surroundFlash               surroundFlash
	dashboardSelection          int
}

func newEditor(cfg config, b *buffer, root string) (*editor, error) {
	if canonical, err := filepath.EvalSymlinks(root); err == nil {
		root = canonical
	}
	if cfg.TabSize < 1 {
		cfg.TabSize = 4
	}
	tabWidth = cfg.TabSize
	e := &editor{cfg: cfg, b: b, buffers: []*buffer{b}, mode: "normal", root: root, macros: make(map[string][]keyInput),
		commands: make(map[string]command), problems: make(map[string][]diagnostic), sessions: make(map[string]*languageSession), async: make(chan func(), 32), completionWake: make(chan uint64, 1)}
	e.layout = &pane{b: b, cursor: b.cursor, top: b.top, left: b.left}
	e.activePane = e.layout
	e.relativeNumbers = cfg.RelativeNumbers
	e.languageWake = make(chan int, 4)
	e.gitWake = make(chan struct{}, 1)
	e.tree = fileTree{sidebar: true, width: 28, expanded: map[string]bool{root: true}}
	e.refreshTree("")
	e.revealFile()
	e.registerCommands()
	e.registerNativeCommands()
	e.workspaceCommands()
	if err := e.registerPlugins(); err != nil {
		return nil, err
	}
	keys, err := compileKeys(cfg, e.commands)
	e.keys = keys
	return e, err
}

func (e *editor) add(id, title string, fn func(*editor)) { e.commands[id] = command{title, fn} }

func (e *editor) registerCommands() {
	e.add("toggle-comment", "Toggle Line Comment", func(e *editor) { e.toggleComment() })
	e.add("files", "Go to File", func(e *editor) { e.openFiles() })
	e.add("explorer", "Focus Explorer", func(e *editor) { e.showTree(false) })
	e.add("palette", "Command Palette", func(e *editor) { e.openPalette() })
	e.add("theme", "Color Theme", func(e *editor) { e.themeMenu() })
	e.add("buffers", "Switch Buffer", func(e *editor) { e.openBuffers() })
	e.add("save", "Save File", func(e *editor) { e.save() })
	e.add("quit", "Quit", func(e *editor) { e.tryQuit(false) })
	e.add("profile-vim", "Use Vim Profile", func(e *editor) { e.profile("vim") })
	e.add("keybindings", "Show Keybindings", func(e *editor) { e.showKeys() })
	e.add("messages", "Notification History", func(e *editor) { e.openNotifications() })
	e.add("complete", "Trigger Completion", func(e *editor) { e.requestCompletion(1, "") })
	e.add("normal", "Normal Mode", func(e *editor) { e.normal() })
	e.add("select", "Select Mode", func(e *editor) { e.selectMode(false) })
	e.add("insert", "Insert Mode", func(e *editor) { e.mode, e.b.anchor = "insert", -1 })
	e.add("append", "Append", func(e *editor) {
		b := e.b
		b.cursor = min(b.lineEnd(b.rowAt(b.cursor)), b.cursor+1)
		e.mode, e.b.anchor = "insert", -1
	})
	e.add("append-line", "Append at Line End", func(e *editor) { b := e.b; b.cursor = b.lineEnd(b.rowAt(b.cursor)); e.mode, e.b.anchor = "insert", -1 })
	e.add("insert-line", "Insert at Line Start", func(e *editor) { b := e.b; b.cursor = b.lines[b.rowAt(b.cursor)]; e.mode, e.b.anchor = "insert", -1 })
	e.add("open-below", "Open Line Below", func(e *editor) { e.openLine(true) })
	e.add("open-above", "Open Line Above", func(e *editor) { e.openLine(false) })
	e.add("newline", "Insert Newline", func(e *editor) { e.insertNewline() })
	e.add("insert-tab", "Insert Tab", func(e *editor) { e.insertTab() })
	e.add("backspace", "Backspace", func(e *editor) {
		if e.deleteAutoPair() {
			return
		}
		b := e.b
		if b.cursor > 0 {
			start := b.cursor - 1
			if start > 0 && b.text[start] == '\n' && b.text[start-1] == '\r' {
				start--
			}
			b.replace(span{start, b.cursor}, "")
		}
	})
	e.add("delete-forward", "Delete Character", func(e *editor) {
		b := e.b
		end := min(len(b.text), b.cursor+e.takeCount())
		if end > b.cursor {
			if end < len(b.text) && b.text[end-1] == '\r' && b.text[end] == '\n' {
				end++
			}
			b.replace(span{b.cursor, end}, "")
			if e.mode != "insert" {
				e.normal()
			}
		}
	})
	e.add("undo", "Undo", func(e *editor) {
		for i := e.takeCount(); i > 0; i-- {
			e.b.history(false)
		}
		e.normal()
	})
	e.add("redo", "Redo", func(e *editor) {
		for i := e.takeCount(); i > 0; i-- {
			e.b.history(true)
		}
		e.normal()
	})
	e.add("paste-after", "Paste After", func(e *editor) { e.paste(true) })
	e.add("paste-before", "Paste Before", func(e *editor) { e.paste(false) })
	e.add("macro-record", "Record Vim Macro", func(e *editor) { e.pending = pending{kind: "record"} })
	e.add("macro-play", "Play Vim Macro", func(e *editor) { e.pending = pending{kind: "play"} })
	e.add("macro-last", "Play Last Macro", func(e *editor) { e.playMacro(e.lastMacro) })
	e.add("surround-selection", "Add Surrounding Delimiters", func(e *editor) { e.pending = pending{kind: "surround"} })
	e.add("surround-delete", "Delete Surrounding Delimiters", func(e *editor) { e.pending = pending{kind: "unsurround"} })
	e.add("surround-replace", "Replace Surrounding Delimiters", func(e *editor) { e.pending = pending{kind: "replace-old"} })
	e.add("inner-object", "Select Inside Text Object", func(e *editor) { e.pending = pending{kind: "object"} })
	e.add("around-object", "Select Around Text Object", func(e *editor) { e.pending = pending{kind: "object", around: true} })
	e.add("search", "Find in Buffer", func(e *editor) { e.openFindBar(false) })
	e.add("replace", "Find and Replace", func(e *editor) { e.openFindBar(true) })
	e.add("next-match", "Find Next", func(e *editor) { e.find(false) })
	e.add("prev-match", "Find Previous", func(e *editor) { e.find(true) })
	e.add("ex", "", func(e *editor) { e.openPalette() })
	e.vimCommands()
	e.foldCommands()
	for _, op := range []string{"delete", "change", "yank"} {
		e.add(op+"-op", "", func(e *editor) { e.pending = pending{kind: "operator", op: op} })
		e.add(op+"-selection", op+" selection", func(e *editor) { e.operate(op, e.b.selection(), e.b.visualLine) })
	}
	for _, name := range []string{"left", "right", "up", "down", "home", "first-nonblank", "end", "top", "bottom", "word", "back-word", "word-end"} {
		e.add(name, "", func(e *editor) {
			b := e.b
			n := e.count
			for i := e.takeCount(); i > 0; i-- {
				b.cursor = motion(b, name, b.cursor)
			}
			if e.cfg.Profile == "vim" && (name == "top" || name == "bottom") {
				row := b.rowAt(b.cursor)
				last := len(b.lines) - 1
				if last > 0 && b.lines[last] == len(b.text) {
					last--
				}
				if n > 0 {
					row = n - 1
				}
				b.cursor = motion(b, "first-nonblank", b.lines[min(row, last)])
			}

		})
	}
	for _, name := range []string{"left", "right", "up", "down"} {
		e.add("insert-"+name, "", func(e *editor) { e.insertMove(name) })
	}
}

func (e *editor) takeCount() int { n := max(1, e.count); e.count = 0; return n }
func (e *editor) normal() {
	if e.mode == "select" {
		e.b.markSelection()
	}
	if e.mode == "insert" && e.blockInsert != nil {
		e.finishBlockInsert()
	}
	e.regPick = 0
	if e.mode == "insert" {
		b := e.b
		if b.cursor > b.lines[b.rowAt(b.cursor)] {
			b.cursor--
		}
	}
	e.mode, e.b.anchor, e.count, e.prefix, e.pending = "normal", -1, 0, "", pending{}
	e.b.visualLine, e.b.visualBlock = false, false
	e.clampModalCursor()
}
func (e *editor) clampModalCursor() {
	if e.mode == "insert" {
		return
	}
	b := e.b
	row := b.rowAt(b.cursor)
	b.cursor = min(b.cursor, max(b.lines[row], b.lineEnd(row)-1))
}

func (e *editor) profile(name string) {
	e.normal()
	e.cfg.Profile = name
	e.notify("Profile: " + name)
}

func (e *editor) insert(s string) { e.b.replace(span{e.b.cursor, e.b.cursor}, s) }
func (e *editor) insertNewline() {
	b := e.b
	start := b.lines[b.rowAt(b.cursor)]
	end := start
	for end < b.cursor && (b.text[end] == ' ' || b.text[end] == '\t') {
		end++
	}
	at := b.cursor
	indent := string(b.text[start:end])
	newline := b.newline()
	if b.tags == nil && at > start && at < len(b.text) && b.text[at-1] == '{' && b.text[at] == '}' {
		inner := newline + indent + e.indentUnit(indent)
		b.replaceAll([]edit{{span{at, at}, inner + newline + indent}}, at+len(inner))
		return
	}
	tag := 0
	if b.tags != nil && at < len(b.tags) && b.text[at] == '\n' {
		tag = b.tags[at]
	}
	e.insert(newline + indent)
	if tag != 0 {
		b.tags[at], b.tags[b.cursor] = tag, 0
	}
}
func (e *editor) openLine(below bool) {
	b := e.b
	row := b.rowAt(b.cursor)
	start := b.lines[row]
	end := start
	for end < b.lineEnd(row) && (b.text[end] == ' ' || b.text[end] == '\t') {
		end++
	}
	e.mode = "insert"
	indent := string(b.text[start:end])
	if below {
		if b.tags != nil && row+1 < len(b.lines) {
			b.cursor = b.lines[row+1]
			at := b.cursor
			e.insert("\n")
			b.cursor = at
			return
		}
		b.cursor = b.lineEnd(row)
		e.insert(b.newline() + indent)
		return
	}
	b.cursor = start
	e.insert(indent + b.newline())
	b.cursor = start + end - start
}
func (e *editor) insertMove(name string) {
	b := e.b
	row := b.rowAt(b.cursor)
	col := b.cursor - b.lines[row]
	switch name {
	case "left":
		b.cursor = max(0, b.cursor-1)
		if b.cursor > 0 && b.cursor < len(b.text) && b.text[b.cursor] == '\n' && b.text[b.cursor-1] == '\r' {
			b.cursor--
		}
	case "right":
		b.cursor = min(len(b.text), b.cursor+1)
		if b.cursor > 0 && b.cursor < len(b.text) && b.text[b.cursor] == '\n' && b.text[b.cursor-1] == '\r' {
			b.cursor++
		}
	case "up":
		row = max(0, row-1)
		b.cursor = min(b.lines[row]+col, b.lineEnd(row))
	case "down":
		row = min(len(b.lines)-1, row+1)
		b.cursor = min(b.lines[row]+col, b.lineEnd(row))
	}
}

func (e *editor) operate(op string, s span, linewise bool) {
	if e.mode == "select" && e.b.visualBlock && e.b.anchor >= 0 && op != "indent" && op != "outdent" {
		e.blockOperate(op)
		return
	}
	e.yanking = op == "yank"
	if op == "indent" || op == "outdent" {
		e.shiftRows(op, s, 1)
		return
	}
	if e.b.tags != nil && linewise && op == "change" && s.end > s.start && e.b.text[s.end-1] == '\n' {
		s.end--
		linewise = false
	}
	text := string(e.b.text[s.start:s.end])
	if linewise && !strings.HasSuffix(text, "\n") {
		text += e.b.newline()
	}
	var tags []int
	if e.b.tags != nil && linewise {
		tags = slices.Clone(e.b.tags[s.start:s.end])
	}
	e.setRegister(text, linewise, tags)
	if op != "yank" && (s.start != s.end || op == "change") {
		value := ""
		if linewise && op == "change" && s.end > s.start && e.b.text[s.end-1] == '\n' {
			value = e.b.newline()
		}
		e.b.replace(s, value)
	}
	e.b.cursor = min(s.start, len(e.b.text))
	e.normal()
	if op == "change" {
		e.mode = "insert"
		e.b.cursor = s.start
	}
}

func (e *editor) paste(after bool) {
	if e.regPick == 0 || e.regPick == '"' {
		e.syncRegister()
	}
	e.withRegister(func() { e.pasteRegister(after) })
}

func (e *editor) pasteRegister(after bool) {
	if e.register == "" {
		return
	}
	b := e.b
	row := b.rowAt(b.cursor)
	at := b.cursor
	n := e.takeCount()
	value := e.register
	if e.linewise {
		at = b.lines[row]
		newline := b.newline()
		if !strings.HasSuffix(value, "\n") {
			value += newline
		}
		value = strings.Repeat(value, n)
		if after && row+1 < len(b.lines) {
			at = b.lines[row+1]
		} else if after {
			at = len(b.text)
			value = newline + strings.TrimSuffix(value, newline)
		}
	} else {
		value = strings.Repeat(value, n)
		if after {
			at = min(b.lineEnd(row), at+1)
		}
	}
	b.replace(span{at, at}, value)
	if b.tags != nil && e.linewise && len(e.registerTags) > 0 && len([]rune(value)) == len(e.registerTags)*n && strings.HasSuffix(value, "\n") {
		for i := range n {
			copy(b.tags[at+i*len(e.registerTags):], e.registerTags)
		}
	}
	b.cursor = at
	if e.linewise && after && at > 0 && b.text[at-1] != '\n' {
		b.cursor += len([]rune(b.newline()))
	}
	e.normal()
}

func inputRune(key string) (rune, bool) {
	if key == "space" {
		return ' ', true
	}
	r, n := utf8.DecodeRuneInString(key)
	return r, n > 0 && n == len(key)
}

func (e *editor) handle(in keyInput) {
	if e.notificationDetail != nil {
		e.notificationInput(in.key)
		return
	}
	if e.menu == nil && e.diff != nil {
		e.diffInput(in)
		return
	}
	if e.menu == nil && e.panel != nil {
		e.sourceControlInput(in)
		return
	}
	if e.menu == nil && e.directory != nil {
		e.directoryInput(in)
		return
	}
	if e.menu == nil && e.bar != nil && e.directory == nil && e.findInput(in) {
		return
	}
	if e.menu == nil && e.tree.focused && e.tree.filtering {
		e.filterInput(in.key)
		return
	}
	if e.menu == nil && e.tree.focused {
		if e.prefix == "" && in.key == ":" {
			e.openPalette()
			return
		}
		if e.prefix == "" && in.key == "space" {
			e.prefix = "space"
			return
		}
		if e.prefix == "" {
			e.tree.manualScroll = false
			if e.treeInput(in.key) {
				return
			}
			// Keep typing scoped to the tree until focus returns to the editor.
			if !strings.HasPrefix(in.key, "ctrl+") {
				return
			}
		}
	}
	e.scrollLock = false

	key := in.key
	if key == "" && in.paste == "" {
		return
	}
	keys := e.keys[e.cfg.Profile+"."+e.mode]
	if e.menu == nil && e.recording != "" && !e.replaying && e.mode != "insert" && e.pending.kind == "" &&
		(keys[key] == "macro-record" || keys[key] == "macro-toggle") && e.prefix == "" {
		e.lastMacro, e.recording = e.recording, ""
		e.notify("Macro recorded")
		return
	}
	if e.recording != "" && !e.replaying {
		record := e.macros[e.recording]
		if len(record) < 10000 {
			e.macros[e.recording] = append(record, in)
		}
	}
	if e.menu != nil {
		e.menuInput(in)
		return
	}
	if e.b.dashboard {
		if in.paste == "" && (e.prefix != "" || !e.dashboardInput(key)) {
			e.dispatchKey(keys, key)
		}
		return
	}
	b, version, cursor := e.b, e.b.version, e.b.cursor
	before, undoLen := e.mode, len(b.undo)
	now := time.Now()
	b.mergeUndo = before == "insert" && !e.replaying && !e.change.repeating && !b.lastInsert.IsZero() && now.Sub(b.lastInsert) < 650*time.Millisecond
	defer func() { b.mergeUndo = false }()
	defer func() {
		e.trackChange(in, b, version, before, undoLen, now)
		if e.b == b && version != b.version {
			e.scheduleLanguage()
			e.scheduleGit()
		}
	}()
	previousPrefix := e.prefix
	idle := e.prefix == "" && e.pending.kind == "" && in.paste == ""
	vertical := idle && (keys[key] == "up" || keys[key] == "down")
	counting := idle && len(key) == 1 && key[0] >= '0' && key[0] <= '9' && (key != "0" || e.count > 0)
	endKey := idle && keys[key] == "end"
	if vertical && !(b.wantSet && b.wantAt == b.cursor) {
		b.wantCol, b.wantSet = b.cursor-b.lines[b.rowAt(b.cursor)], true
	}
	if in.paste != "" {
		e.pending, e.prefix, e.count = pending{}, "", 0
		if e.mode != "insert" {
			e.mode = "insert"
			b.anchor = -1
			b.visualLine = false
		}
		e.insert(strings.ReplaceAll(strings.ReplaceAll(in.paste, "\r\n", "\n"), "\n", b.newline()))
	} else if e.completionInput(key) {
		return
	} else if e.pending.kind != "" {
		e.pendingInput(key)
	} else {
		e.dispatchKey(keys, key)
	}
	switch {
	case vertical || counting:
		b.wantAt = b.cursor
	case endKey:
		b.wantCol, b.wantSet, b.wantAt = wantEnd, true, b.cursor
	default:
		b.wantSet = false
	}
	refreshed := e.b == b && version != b.version && e.mode == "insert" && e.refreshCompletion(key, cursor)
	if !refreshed && (e.b != b || version != b.version || cursor != b.cursor || e.mode != "insert" || previousPrefix != e.prefix) {
		e.invalidateCompletion()
	}
	if !refreshed && e.b == b && version != b.version && e.mode == "insert" && e.cfg.AutoComplete && e.menu == nil && !e.replaying {
		if !e.pathCompletion() {
			e.scheduleCompletion(key)
		}
	}
}

func (e *editor) dispatchKey(keys map[string]string, key string) {
	if e.prefix == "" && e.mode != "insert" && len(key) == 1 && key[0] >= '0' && key[0] <= '9' && (key != "0" || e.count > 0) {
		e.count = min(9999, 10*e.count+int(key[0]-'0'))
		return
	}
	sequence := key
	if e.prefix != "" {
		sequence = e.prefix + " " + key
	}
	if id := keys[sequence]; id != "" {
		e.prefix = ""
		e.runCommand(id)
		if e.pending.kind == "" {
			e.count = 0
		}
		return
	}
	if key != "esc" {
		for candidate := range keys {
			if strings.HasPrefix(candidate, sequence+" ") {
				e.prefix = sequence
				return
			}
		}
	}
	if e.prefix != "" {
		e.prefix, e.count = "", 0
		if key == "esc" {
			e.normal()
		}
		return
	}
	if r, ok := inputRune(key); ok {
		if e.mode == "insert" {
			e.typeRune(r)
		} else if e.mode == "select" && e.cfg.AutoPairs {
			e.wrapSelection(r)
		}
	}
	e.count = 0
}

func (e *editor) pendingInput(key string) {
	p := e.pending
	e.pending = pending{}
	if key == "esc" {
		e.count = 0
		return
	}
	r, ok := inputRune(key)
	if p.kind == "operator" {
		e.operatorKey(p, key)
		return
	}
	if p.kind == "operator-g" {
		count := e.count
		n := e.takeCount()
		e.operatorMotion(p.op, e.keys[e.cfg.Profile+".normal"]["g "+key], count, n)
		return
	}
	if !ok {
		e.count = 0
		return
	}
	b := e.b
	switch p.kind {
	case "find-character":
		e.lastFind = characterFind{r, strings.ContainsAny(p.op[:1], "FT"), strings.ContainsAny(p.op[:1], "tT")}
		e.findCharacter(p.op[1:], e.lastFind, false, false)
	case "replace-character":
		end := b.cursor + e.takeCount()
		if end <= b.lineEnd(b.rowAt(b.cursor)) {
			b.replaceAll([]edit{{span{b.cursor, end}, strings.Repeat(string(r), end-b.cursor)}}, end-1)
		}
	case "mini-add", "mini-delete", "mini-replace-old", "mini-replace-new", "mini-find", "mini-find-back", "mini-highlight":
		e.miniSurround(p, r)
		return
	case "register":
		if validRegister(r) {
			e.regPick = r
		}
		return
	case "mark-set", "mark-line", "mark-exact":
		e.markInput(p.kind, r)
		return
	case "record":
		if r >= 'a' && r <= 'z' {
			e.startMacro(key)
		}
	case "play":
		if key == "@" {
			key = e.lastMacro
		}
		e.playMacro(key)
	case "object", "op-object", "surround-object":
		if s, ok := textObject(b, r, p.around); ok {
			if p.kind == "op-object" {
				e.applyMotionOperator(p.op, s, false)
				return
			} else {
				if s.start == s.end {
					e.notify("Empty text object")
					break
				}
				e.mode, b.anchor, b.cursor, b.visualLine = "select", s.start, max(s.start, s.end-1), false
				if p.kind == "surround-object" {
					e.pending = pending{kind: "surround"}
					return
				}
			}
		} else {
			e.notify("Text object not found")
		}
	case "surround":
		open, close := delimiters(r)
		s := b.selection()
		if p.op == "line" {
			s = span{b.lines[b.rowAt(b.cursor)], b.lineEnd(b.rowAt(b.cursor))}
		} else if b.visualLine {
			s = b.withoutFinalNewline(s)
		}
		b.wrap(s, string(open), string(close), s.start)
		b.cursor = s.start
		e.normal()
	case "surround-op":
		switch key {
		case "i", "a":
			e.pending = pending{kind: "surround-object", around: key == "a"}
		case "s":
			e.pending = pending{kind: "surround", op: "line"}
		default:
			name := e.keys[e.cfg.Profile+".normal"][key]
			switch name {
			case "left", "right", "word", "back-word", "word-end", "home", "end":
				to := motion(b, name, b.cursor)
				if name == "word" && to > b.cursor {
					to--
				}
				b.anchor, b.cursor, e.mode = b.cursor, to, "select"
				e.pending = pending{kind: "surround"}
			}
		}
		return
	case "replace-old":
		e.pending = pending{kind: "replace-new", old: r}
		return
	case "unsurround", "replace-new":
		old := r
		if p.kind == "replace-new" {
			old = p.old
		}
		s, ok := enclosing(b, old, b.selection())
		if !ok {
			e.notify("Surrounding delimiters not found")
			return
		}
		value := string(b.text[s.start+1 : s.end-1])
		if p.kind == "replace-new" {
			open, close := delimiters(r)
			value = string(open) + value + string(close)
		}
		b.replace(s, value)
		b.cursor = s.start
		e.normal()
	}
	e.count = 0
}

func (e *editor) operatorKey(p pending, key string) {
	op := p.op
	if key == "s" && op != "mini-add" {
		kind := map[string]string{"delete": "unsurround", "change": "replace-old", "yank": "surround-op"}
		e.pending = pending{kind: kind[op]}
		return
	}
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' && (key != "0" || p.count > 0) {
		if p.count == 0 {
			p.count = max(1, e.takeCount())
		}
		e.count = min(9999, e.count*10+int(key[0]-'0'))
		e.pending = p
		return
	}
	if p.count != 0 {
		e.count = min(9999, e.count*p.count)
	}
	if key == "i" || key == "a" {
		e.pending = pending{kind: "op-object", op: op, around: key == "a"}
		return
	}
	b := e.b
	if e.cfg.Profile == "vim" {
		switch key {
		case "%":
			e.matchBracket(op)
			return
		case "f", "F", "t", "T":
			e.pending = pending{kind: "find-character", op: key + op}
			return
		case ";", ",":
			e.findCharacter(op, e.lastFind, key == ",", true)
			return
		}
	}
	count := e.count
	n := e.takeCount()
	repeat := map[string]string{"delete": "d", "change": "c", "yank": "y", "indent": ">", "outdent": "<"}
	if key != "" && (key == repeat[op] || op == "mini-add" && key == "_") {
		row := b.rowAt(b.cursor)
		last := min(len(b.lines), row+n)
		e.applyMotionOperator(op, b.lineSpan(row, last-1), true)
		return
	}
	name := e.keys[e.cfg.Profile+".normal"][key]
	if key == "g" && e.cfg.Profile == "vim" {
		e.pending = pending{kind: "operator-g", op: op}
		e.count = count
		return
	}
	e.operatorMotion(op, name, count, n)
}

func (e *editor) operatorMotion(op, name string, count, n int) {
	b := e.b
	switch name {
	case "up", "down", "top", "bottom":
		to := b.cursor
		for range n {
			to = motion(b, name, to)
		}
		if (name == "top" || name == "bottom") && count > 0 {
			to = b.lines[min(count-1, len(b.lines)-1)]
		}
		first, last := b.rowAt(min(b.cursor, to)), b.rowAt(max(b.cursor, to))
		e.applyMotionOperator(op, b.lineSpan(first, last), true)
	case "left", "right", "word", "back-word", "word-end", "home", "first-nonblank", "end":
		to := b.cursor
		if op == "change" && name == "word" && to < len(b.text) && wordClass(b.text[to]) != 0 {
			name = "word-end"
		}
		for range n {
			to = motion(b, name, to)
		}
		if name == "right" {
			to = min(b.lineEnd(b.rowAt(b.cursor)), b.cursor+n)
		}
		s := span{min(b.cursor, to), max(b.cursor, to)}
		if name == "word-end" || name == "end" {
			s.end = min(b.lineEnd(b.rowAt(s.end)), s.end+1)
		}
		if s.start != s.end || op != "mini-add" {
			e.applyMotionOperator(op, s, false)
		}
	}
}

func (e *editor) startMacro(name string) {
	if e.replaying {
		return
	}
	e.recording = name
	e.macros[name] = nil
	e.notify("Recording macro " + name)
}
func (e *editor) playMacro(name string) {
	n := min(e.takeCount(), 100)
	if e.replaying {
		return
	}
	keys, ok := e.macros[name]
	if !ok {
		e.notify("Macro register is empty")
		return
	}
	e.lastMacro, e.replaying = name, true
	b, version := e.b, e.b.version
	initial := b.snapshot()
	history := slices.Clone(b.undo)
	defer func() {
		e.replaying = false
		if e.b == b && b.version != version {
			b.undo = pushHistory(history, initial)
			e.scheduleLanguage()
		}
	}()
	for range n {
		for _, key := range keys {
			if e.quit {
				return
			}
			e.handle(key)
		}
	}
}

func (e *editor) open(path string) bool {
	b, err := loadBuffer(path)
	if err != nil {
		e.notify(err.Error())
		return false
	}
	e.openLoaded(b)
	return true
}

func (e *editor) openLoaded(b *buffer) {
	for _, existing := range e.buffers {
		if !existing.dashboard && existing.path == b.path {
			e.switchBuffer(existing)
			return
		}
	}
	if ds := e.problems[b.path]; len(ds) > 0 {
		b.diagnostics = slices.Clone(ds)
	}
	if len(e.buffers) == 1 && e.b.path == "" && !e.b.dashboard && !e.b.dirty() && e.layout == e.activePane {
		e.buffers[0] = b
	} else {
		e.buffers = append(e.buffers, b)
	}
	e.switchBuffer(b)
}

// The active leaf projects its view onto e.b for existing modal commands.
func (e *editor) switchBuffer(b *buffer) {
	if e.directory != nil && !e.directory.doc.b.dirty() {
		e.directory = nil
	}
	e.invalidateCompletion()
	e.normal()
	e.rememberPane()
	e.b = b
	if e.activePane != nil {
		p := e.activePane
		previous := p.dashboardPrevious
		if b.dashboard && !p.b.dashboard && p.b.path != "" {
			fileView := *p
			p.dashboardPrevious = &fileView
		} else if !b.dashboard {
			p.dashboardPrevious = nil
			if previous != nil && b == previous.b {
				b.cursor, b.top, b.left = min(previous.cursor, len(b.text)), previous.top, previous.left
			}
		}
		p.b, p.cursor, p.top, p.left = b, b.cursor, b.top, b.left
	}
	e.afterBufferSwitch()
}

func (e *editor) afterBufferSwitch() {
	e.invalidateCompletion()
	e.clampModalCursor()
	e.scrollLock = false
	e.tree.focused, e.tree.popup = false, false
	e.tree.manualScroll = false
	e.change.inputs, e.change.changed = nil, false
	e.revealFile()
	e.revealTreeSelection()
	e.scheduleLanguage()
	e.refreshGit()
}
func (e *editor) save() bool {
	if e.directory != nil {
		e.writeDirectory()
		return false
	}
	return e.saveAgainst(nil, false)
}
func (e *editor) tryQuit(force bool) {
	if e.directory != nil {
		e.closeDirectory(force)
		return
	}
	if !force {
		for _, b := range e.buffers {
			if b.dirty() {
				e.notify("Unsaved buffer. Save it or use :q!")
				return
			}
		}
	}
	e.discardRecovery = force
	e.quit = true
}
func (e *editor) ex(line string) {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ":"))
	if e.directoryEx(line) || e.exAddress(line) || e.runExRange(line) {
		return
	}
	id, arg, _ := strings.Cut(strings.TrimSpace(line), " ")
	arg = strings.TrimSpace(arg)
	switch id {
	case "cd":
		if arg == "" {
			e.directoryPicker()
		} else {
			e.goDirectory(arg)
		}
	case "split", "sp", "vsplit", "vs":
		e.splitPane(id == "vsplit" || id == "vs", arg)
	case "only":
		e.closePane(true)
	case "close":
		switch arg {
		case "tab":
			e.closeBuffer(e.b)
		case "", "pane":
			e.closePane(false)
		default:
			e.notify("Use :close, :close pane, or :close tab")
		}
	case "ExploreClose":
		e.hideExplorer()
	case "tabnext", "tabn", "tabprev", "tabp":
		if arg == "" {
			delta := 1
			if id == "tabprev" || id == "tabp" {
				delta = -1
			}
			e.cycleTab(delta)
		} else if n, err := strconv.Atoi(arg); err == nil {
			e.tabNumber(n)
		} else {
			e.notify("Expected a tab number")
		}
	case "tabfirst":
		e.tabNumber(1)
	case "tablast":
		e.tabNumber(len(e.buffers))
	case "tabclose":
		e.closeBuffer(e.b)
	case "tabmove":
		index := len(e.buffers) - 1
		if arg != "" {
			n, err := strconv.Atoi(arg)
			if err != nil {
				e.notify("Use :tabmove [+/-]N")
				return
			}
			index = n
			if strings.HasPrefix(arg, "+") || strings.HasPrefix(arg, "-") {
				index = slices.Index(e.buffers, e.b) + n
			}
		}
		e.moveTab(index)
	case "set":
		switch arg {
		case "rnu", "relativenumber":
			e.relativeNumbers = true
		case "nornu", "norelativenumber":
			e.relativeNumbers = false
		case "rnu!", "relativenumber!":
			e.relativeNumbers = !e.relativeNumbers
		default:
			e.notify("Supported options: relativenumber, norelativenumber, relativenumber!")
		}
		if e.directory != nil {
			e.directory.doc.relativeNumbers = e.relativeNumbers
		}
	case "w", "write":
		if e.b.dashboard {
			e.notify("Open a file before saving")
			return
		}
		if arg == "" {
			e.save()
			return
		}
		path, err := filepath.Abs(arg)
		if err != nil {
			e.notify(err.Error())
			return
		}
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		for _, b := range e.buffers {
			if b != e.b && b.path == path {
				e.notify("Path is already open in another buffer")
				return
			}
		}
		old := e.b.path
		if path == old {
			e.save()
			return
		}
		d, err := readDisk(path)
		if err != nil {
			e.notify(err.Error())
			return
		}
		if d.exists {
			e.notify("Save-as destination already exists")
			return
		}
		e.b.path = path
		if !e.saveAgainst(&d, false) {
			e.b.path = old
			e.menu = nil
			return
		}
		e.invalidateCompletion()
		if old != path {
			e.closeDocument(old)
			e.revealFile()
			e.scheduleLanguage()
		}
	case "wq":
		if e.saveAgainst(nil, true) {
			e.tryQuit(false)
		}
	case "q", "quit":
		if e.layout != nil && e.layout != e.activePane {
			e.closePane(false)
		} else {
			e.tryQuit(false)
		}
	case "qa", "qall":
		e.tryQuit(false)
	case "qa!", "qall!":
		e.tryQuit(true)
	case "q!", "quit!":
		e.tryQuit(true)
	case "noh", "nohlsearch":
		e.searchVisible = false
	case "e", "edit":
		if arg != "" {
			e.open(arg)
		} else {
			e.openFiles()
		}
	case "vim":
		e.profile(id)
	default:
		if c, ok := e.commands[id]; ok && arg == "" {
			if e.directory != nil && directoryTextCommand(id) {
				c.run(e.directory.doc)
			} else {
				e.runCommand(id)
			}
		} else {
			e.notify(fmt.Sprintf("Unknown command %q", id))
		}
	}
}
