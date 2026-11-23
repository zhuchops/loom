//go:build desktop

package editor

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
)

// The desktop front end shares the editor, its commands and its renderer with
// the terminal build, but only the text panes live on a character grid. The
// title bar, sidebar, tabs, status bar, pickers, panels and popovers are laid
// out and hit-tested in pixels.

// MainDesktop runs Loom in a native window and exits the process on failure.
func MainDesktop() {
	// Started from Finder, the working directory is "/"; begin in the home folder.
	if wd, err := os.Getwd(); err == nil && wd == "/" {
		if home, err := os.UserHomeDir(); err == nil {
			_ = os.Chdir(home)
		}
	}
	platformEnvironment()
	e, explicit, done, err := prepare(true)
	if err == nil && !done {
		go func() {
			if err := runDesktop(e, explicit); err != nil {
				fmt.Fprintln(os.Stderr, "loom:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}()
		app.Main()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "loom:", err)
		os.Exit(1)
	}
}

const defaultUISize = 13 // the size the interface is designed at

type desktop struct {
	w     *app.Window
	m     *terminalModel
	shown *editor // the session the state below belongs to
	inbox chan tea.Msg

	gtx     layout.Context
	scratch op.Ops
	shaper  *text.Shaper
	sizes   map[measured]image.Point
	wraps   map[wrapped][]string

	uiFont, uiStrong, codeFont font.Font
	codeFamily, uiFamily       *fontFamily
	hasItalic                  bool
	look                       fontLook
	clip                       rect

	marks map[int]paint.ImageOp // the logo, drawn once per size

	c    uiColors
	inks map[[2]color.NRGBA]color.NRGBA

	// The code grid.
	metric     unit.Metric
	scale      float32
	code       textStyle
	cellW      float64
	cellH      int
	lift       int
	cols, rows int

	geo geometry

	// Pointer handling works on the zones painted in the previous frame.
	zones      []zone
	drags      []rect // parts of the title bar that move the window
	layer      int
	hoverLayer int
	grab       *zone
	mx, my     int
	wheelX     float32
	wheelY     float32
	focused    bool // has asked for keyboard focus
	active     bool // the window is in front

	tabs      []tabSlot
	tabScroll int
	tabShown  *buffer

	list    listState
	toast   toast
	dialog  *dialog
	preview previewCache

	settings *settingsView

	scroll scrollAnim
	cur    cursorAnim
}

func runDesktop(e *editor, explicit bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	w := new(app.Window)
	options := append([]app.Option{
		app.Title("Loom"),
		app.Size(1280, 820),
		app.MinSize(640, 420),
	}, platformOptions()...)
	w.Option(options...)
	d := newDesktop(w, e, ctx, ticker.C)
	d.platformStart()
	d.serveLaunches()
	e.begin(explicit)
	defer e.hub.finish()
	d.askAboutConfig()
	d.run(d.m.Init())
	var ops op.Ops
	for {
		switch ev := w.Event().(type) {
		case app.DestroyEvent:
			return ev.Err
		case *app.ClosingEvent:
			if d.m.editor.quit || !d.unsaved() {
				// Leave now: a DestroyEvent only comes once AppKit frees the view.
				return nil
			}
			ev.Abort()
			d.requestQuit()
			w.Invalidate()
		case app.ConfigEvent:
			d.active = ev.Config.Focused
		case app.FrameEvent:
			gtx := app.NewContext(&ops, ev)
			d.frame(gtx)
			ev.Frame(gtx.Ops)
			if d.m.editor.quit {
				return nil
			}
		default:
			d.platformEvent(ev)
		}
	}
}

// newDesktop builds the front end around an editor; tests use it without a
// window loop.
func newDesktop(w *app.Window, e *editor, ctx context.Context, ticks <-chan time.Time) *desktop {
	d := &desktop{
		w: w, inbox: make(chan tea.Msg, 256), active: true, look: lookOf(e.cfg.options),
		sizes: map[measured]image.Point{}, wraps: map[wrapped][]string{}, inks: map[[2]color.NRGBA]color.NRGBA{},
	}
	d.m = &terminalModel{editor: e, view: renderer{screen: newTerminalFrame(0, 0), native: true}, ctx: ctx, ticks: ticks}
	d.m.attach()
	d.shown = e
	d.loadFonts()
	e.settingsPage = d.toggleSettings
	return d
}

// post hands msg to the event loop from any goroutine.
func (d *desktop) post(msg tea.Msg) {
	d.inbox <- msg
	d.w.Invalidate()
}

// uiCallback runs on the event loop.
type uiCallback func()

// run executes a Bubble Tea command in the background and queues its result.
func (d *desktop) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		if msg := cmd(); msg != nil {
			d.post(msg)
		}
	}()
}

func (d *desktop) send(msg tea.Msg) {
	switch msg := msg.(type) {
	case uiCallback:
		msg()
		return
	case tea.BatchMsg:
		for _, cmd := range msg {
			d.run(cmd)
		}
		return
	case tea.QuitMsg:
		d.m.editor.quit = true
		return
	case tea.KeyPressMsg:
		if d.settingsHaveKeys() {
			d.settingsKey(terminalKey(tea.Key(msg)))
			return
		}
	case tea.PasteMsg:
		if d.settingsHaveKeys() {
			d.settingsKey(keyInput{paste: msg.Content})
			return
		}
	}
	_, cmd := d.m.Update(msg)
	d.run(cmd)
}

func (d *desktop) drain() {
	for {
		select {
		case msg := <-d.inbox:
			d.send(msg)
		default:
			return
		}
	}
}

// measure sizes the code grid. A cell can be a fraction of a pixel wide:
// rounding it would make neighbouring font sizes look the same.
func (d *desktop) measure() {
	gtx := d.gtx
	if d.cellW > 0 && gtx.Metric == d.metric {
		return
	}
	d.metric = gtx.Metric
	d.scale = gtx.Metric.PxPerDp * d.look.uiPts / defaultUISize
	clear(d.sizes)
	clear(d.wraps)

	sample := strings.Repeat("M", 500)
	d.code = textStyle{size: d.look.size, mono: true, code: true}
	size := d.measureText(sample, d.code)
	d.cellW = max(1, float64(size.X)/float64(len(sample))+float64(d.look.spacing*gtx.Metric.PxPerDp))
	d.cellH = max(1, d.px(d.look.size*d.look.height))
	d.lift = (d.cellH - size.Y) / 2
}

// cellX is the pixel offset of a column.
func (d *desktop) cellX(x int) int { return int(math.Round(float64(x) * d.cellW)) }

// syncLook follows the font settings.
func (d *desktop) syncLook() {
	look := lookOf(d.m.editor.cfg.options)
	if look == d.look {
		return
	}
	reload := !look.sameFaces(d.look)
	d.look = look
	if reload {
		d.loadFonts()
	}
	clear(d.sizes)
	clear(d.wraps)
	d.cellW = 0
	d.scroll, d.cur = scrollAnim{}, cursorAnim{}
	d.measure()
}

// zoom changes the saved font size. Zero goes back to the default.
func (d *desktop) zoom(delta float32) {
	e := d.m.editor
	var err error
	if delta == 0 {
		err = e.resetSetting("font_size")
	} else {
		s := settingByKey["font_size"]
		err = e.setSetting(s.key, max(s.min, min(s.max, e.cfg.FontSize+float64(delta))))
	}
	if err != nil {
		e.notify(err.Error())
	}
}

func (d *desktop) frame(gtx layout.Context) {
	d.gtx = gtx
	d.syncLook()
	d.measure()
	d.drain()
	d.input()
	d.run(d.m.settle()) // a click can switch sessions without going through Update
	e := d.m.editor
	if e != d.shown {
		d.follow(e)
	}
	if c := deriveColors(); c != d.c {
		d.c = c
		clear(d.inks)
		d.platformTheme()
	}
	d.syncLook() // settings may have changed with this frame's input
	d.geo = d.arrange(e, gtx.Constraints.Max)
	cols, rows := max(4, int(float64(d.geo.grid.Dx())/d.cellW)), max(5, d.geo.grid.Dy()/d.cellH)
	if cols != d.cols || rows != d.rows {
		d.cols, d.rows = cols, rows
		d.send(tea.WindowSizeMsg{Width: cols, Height: rows})
	}
	d.m.view.draw(e)
	d.paint(e)
	d.inputArea()
	if e.animating() {
		gtx.Execute(op.InvalidateCmd{})
	}
}

// follow drops what was on screen for the session that was shown before.
func (d *desktop) follow(e *editor) {
	d.shown = e
	d.settings = nil
	d.scroll, d.cur = scrollAnim{}, cursorAnim{}
	d.list, d.preview = listState{}, previewCache{}
	d.tabScroll, d.tabShown = 0, nil
	d.grab = nil
	// Its older messages aren't news.
	d.toast = toast{seen: d.m.switchedAt}
}

// geometry is where the parts of the window are.
type geometry struct {
	window, title, sidebar, tabs, crumbs, editor, status rect
	dirCard                                              rect
	grid                                                 rect
}

func (d *desktop) sidebarShown(e *editor, width int) bool {
	if e.zen || e.noProject || width < d.u(520) {
		return false
	}
	return e.tree.sidebar || e.tree.popup || e.tree.focused
}

func (d *desktop) arrange(e *editor, size image.Point) geometry {
	w, h := size.X, size.Y
	g := geometry{window: image.Rectangle{Max: size}}
	top, bottom, left := max(d.px(titleHeight), d.u(titleHeight)), h, 0
	g.title = image.Rect(0, 0, w, top)
	if !e.zen && e.cfg.StatusBar {
		bottom -= d.u(27)
		g.status = image.Rect(0, bottom, w, h)
	}
	if d.sidebarShown(e, w) {
		left = min(d.u(float32(max(18, e.tree.width)*9)), w*2/5)
		g.sidebar = image.Rect(0, top, left, bottom)
	}
	y := top
	if !e.zen && (len(e.buffers) > 1 || !e.b.dashboard) {
		g.tabs = image.Rect(left, y, w, y+d.u(36))
		y = g.tabs.Max.Y
		if !e.b.dashboard && e.b.path != "" && e.cfg.PathBar {
			g.crumbs = image.Rect(left, y, w, y+d.u(36))
			y = g.crumbs.Max.Y
		}
	}
	g.editor = image.Rect(left, y, w, bottom)
	inner := g.editor
	if e.directory != nil {
		cw, ch := min(d.u(680), w-d.u(40)), min(d.u(500), bottom-top-d.u(40))
		x, cy := (w-cw)/2, top+(bottom-top-ch)/2
		g.dirCard = image.Rect(x, cy, x+cw, cy+ch)
		inner = image.Rect(x+d.u(6), cy+d.u(46), x+cw-d.u(6), cy+ch-d.u(38))
	}
	g.grid = image.Rect(inner.Min.X+d.u(4), inner.Min.Y+d.u(4), inner.Max.X-d.u(6), inner.Max.Y)
	return g
}

const (
	layerBase = iota
	layerPopover
	layerModal
	layerToast
	layerDialog
)

func (d *desktop) paint(e *editor) {
	d.zones, d.drags, d.tabs = d.zones[:0], d.drags[:0], d.tabs[:0]
	g := &d.geo

	d.layer = layerBase
	d.fill(g.window, d.c.bg)
	if e.directory == nil {
		d.paintEditor(e, g.editor, d.c.bg)
	}
	d.paintTitle(e)
	d.paintSidebar(e)
	d.paintTabs(e)
	d.paintCrumbs(e)
	d.paintStatus(e)

	d.layer = layerPopover
	if e.menu == nil && e.panel == nil && e.diff == nil && e.directory == nil && d.settings == nil {
		d.paintFind(e)
		d.paintCompletion(e)
		d.paintHover(e)
	}

	d.layer = layerModal
	if d.settings != nil {
		d.paintSettings(e)
	}
	switch {
	case e.notificationDetail != nil:
		d.paintDetail(e)
	case e.menu != nil:
		d.paintPicker(e)
	case e.diff != nil:
		d.paintDiff(e)
	case e.panel != nil:
		d.paintGit(e)
	case e.directory != nil:
		d.paintDirectory(e)
	}
	if e.menu == nil && e.panel == nil && e.diff == nil && e.notificationDetail == nil && d.settings == nil {
		d.paintWhichKey(e)
	}

	d.layer = layerToast
	d.paintToast(e)

	d.layer = layerDialog
	d.paintDialog()
}
