//go:build desktop

package editor

import (
	"image"
	"image/color"
	"path/filepath"
	"strings"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/zhuchops/loom/internal/brand"
)

type iconID int

const (
	iconSearch iconID = iota
	glyphFolder
	glyphFolderOpen
	glyphFile
	iconNewFile
	iconNewFolder
	iconBranch
	iconSidebar
	iconSettings
	iconClose
	iconCommand
	iconTabs
	iconCheck
	iconError
	iconWarning
	iconInfo
	iconPlus
	iconMinus
	iconRefresh
	iconRight
	iconDown
	iconUp
)

func (d *desktop) icon(id iconID, cx, cy int, size float32, c color.NRGBA) {
	p := d.pen(cx, cy, size, c)
	switch id {
	case iconSearch:
		p.circle(7, 7, 4.5)
		p.line(10.2, 10.2, 14, 14)
	case glyphFolder:
		p.line(2, 12.8, 2, 3.5, 6, 3.5, 7.6, 5.3, 14, 5.3, 14, 12.8, 2, 12.8, 2, 11)
	case glyphFolderOpen:
		p.line(2, 12.8, 2, 3.5, 6, 3.5, 7.6, 5.3, 13, 5.3, 13, 7.5)
		p.line(2, 12.8, 4.2, 7.5, 15, 7.5, 12.8, 12.8, 2, 12.8, 2.5, 11.6)
	case glyphFile:
		p.line(9.5, 2, 4, 2, 4, 14, 12, 14, 12, 4.5, 9.5, 2, 9.5, 4.5, 12, 4.5)
	case iconNewFile:
		p.line(8, 14, 3, 14, 3, 2, 8.5, 2, 11, 4.5, 11, 7)
		p.line(12, 9, 12, 15)
		p.line(9, 12, 15, 12)
	case iconNewFolder:
		p.line(8, 12.8, 1.5, 12.8, 1.5, 3.5, 5.5, 3.5, 7, 5.3, 13.5, 5.3, 13.5, 7.5)
		p.line(12.5, 9, 12.5, 15)
		p.line(9.5, 12, 15.5, 12)
	case iconBranch:
		p.circle(4.5, 3.5, 1.8)
		p.circle(4.5, 12.5, 1.8)
		p.circle(11.5, 5.5, 1.8)
		p.line(4.5, 5.3, 4.5, 10.7)
		p.line(11.5, 7.3, 11, 8.6, 9.5, 9.4, 4.5, 9.8)
	case iconSidebar:
		p.line(2, 4, 2, 13, 14, 13, 14, 3, 2, 3, 2, 4)
		p.line(6.3, 3, 6.3, 13)
	case iconSettings:
		p.line(2, 4.5, 8, 4.5)
		p.line(12, 4.5, 14, 4.5)
		p.circle(10, 4.5, 2)
		p.line(2, 11.5, 4, 11.5)
		p.line(8, 11.5, 14, 11.5)
		p.circle(6, 11.5, 2)
	case iconClose:
		p.line(4.2, 4.2, 11.8, 11.8)
		p.line(11.8, 4.2, 4.2, 11.8)
	case iconCommand:
		p.line(2, 4, 2, 13, 14, 13, 14, 3, 2, 3, 2, 4)
		p.line(4.8, 6, 7, 8, 4.8, 10)
		p.line(8.5, 10, 11.2, 10)
	case iconTabs:
		p.line(2, 6, 2, 14, 11, 14, 11, 5, 2, 5, 2, 6)
		p.line(5, 5, 5, 2, 14, 2, 14, 11, 11, 11)
	case iconCheck:
		p.line(3, 8.4, 6.5, 11.8, 13, 4.5)
	case iconError:
		p.circle(8, 8, 6)
		p.line(5.8, 5.8, 10.2, 10.2)
		p.line(10.2, 5.8, 5.8, 10.2)
	case iconWarning:
		p.line(8, 2.5, 14.3, 13.3, 1.7, 13.3, 8, 2.5, 8.6, 3.5)
		p.line(8, 6.2, 8, 9.4)
		p.line(8, 11, 8, 12.2)
	case iconInfo:
		p.circle(8, 8, 6)
		p.line(8, 7.2, 8, 11.5)
		p.line(8, 4.4, 8, 5.7)
	case iconPlus:
		p.line(8, 3.5, 8, 12.5)
		p.line(3.5, 8, 12.5, 8)
	case iconMinus:
		p.line(3.5, 8, 12.5, 8)
	case iconRefresh:
		p.line(13, 8, 12.3, 10.8, 10, 12.6, 7, 12.8, 4.2, 11, 3, 8, 4.2, 5, 7, 3.2, 10, 3.5, 12.6, 5.6)
		p.line(13, 2.5, 13, 6, 9.5, 6)
	case iconRight:
		p.line(6, 4, 10, 8, 6, 12)
	case iconDown:
		p.line(4, 6, 8, 10, 12, 6)
	case iconUp:
		p.line(4, 10, 8, 6, 12, 10)
	}
}

// fileMark is a short coloured monogram for a file type.
func (d *desktop) fileMark(path string) (string, color.NRGBA) {
	c := &d.c
	switch strings.ToLower(filepath.Base(path)) {
	case "makefile", "dockerfile":
		return "#", c.orange
	case "go.mod", "go.sum", "go.work":
		return "GO", c.purple
	case ".gitignore", ".gitattributes", ".gitmodules":
		return "git", c.orange
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "GO", c.blue
	case ".ts", ".tsx":
		return "TS", c.blue
	case ".js", ".jsx", ".mjs", ".cjs":
		return "JS", c.yellow
	case ".json", ".jsonc":
		return "{}", c.yellow
	case ".md", ".markdown":
		return "MD", c.blue
	case ".html", ".xml", ".svg":
		return "<>", c.orange
	case ".css", ".scss":
		return "#", c.purple
	case ".py":
		return "PY", c.yellow
	case ".rs":
		return "RS", c.orange
	case ".rb":
		return "RB", c.red
	case ".java", ".kt":
		return "JV", c.orange
	case ".c", ".h":
		return "C", c.blue
	case ".cpp", ".cc", ".hpp":
		return "C+", c.blue
	case ".sh", ".zsh", ".bash":
		return "$", c.green
	case ".yaml", ".yml":
		return "YM", c.red
	case ".toml", ".ini", ".conf":
		return "TM", c.orange
	case ".sql":
		return "DB", c.purple
	case ".lua":
		return "LU", c.blue
	case ".swift":
		return "SW", c.orange
	}
	return "", c.dim
}

func (d *desktop) fileIcon(cx, cy int, path string) {
	mark, c := d.fileMark(path)
	if mark == "" {
		d.icon(glyphFile, cx, cy, 15, c)
		return
	}
	st := textStyle{size: 9.5, strong: true, mono: true}
	if len(mark) > 2 {
		st.size = 8
	}
	d.textCenter(cx, cy, mark, st, readable(c, d.c.side, 3.5))
}

// logo draws the Loom mark, the same one as on the app icon.
func (d *desktop) logo(cx, cy int, size float32) {
	px := max(1, d.u(size))
	mark, ok := d.marks[px]
	if !ok {
		if d.marks == nil || len(d.marks) > 16 {
			d.marks = map[int]paint.ImageOp{}
		}
		mark = paint.NewImageOp(brand.Mark(px))
		mark.Filter = paint.FilterNearest
		d.marks[px] = mark
	}
	at := image.Pt(cx-px/2, cy-px/2)
	defer op.Offset(at).Push(d.gtx.Ops).Pop()
	defer clip.Rect{Max: image.Pt(px, px)}.Push(d.gtx.Ops).Pop()
	mark.Add(d.gtx.Ops)
	paint.PaintOp{}.Add(d.gtx.Ops)
}
