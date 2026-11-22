//go:build desktop

package editor

import (
	"cmp"
	"encoding/binary"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/text"
	otfont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
)

// The names the chosen fonts are registered under, so that a lookup can't
// land on some other installed face of the same family.
const (
	faceCode     font.Typeface = "LoomCode"
	faceFallback font.Typeface = "LoomFallback"
	faceUI       font.Typeface = "LoomUI"
)

// fontFile is one face inside a font file.
type fontFile struct {
	path   string
	index  int
	weight int
	italic bool
	narrow bool // condensed or expanded
	// The weight axis of a variable font.
	minWeight, maxWeight int
}

func (f fontFile) variable() bool { return f.maxWeight > f.minWeight }

type fontFamily struct {
	name  string // shown and saved
	query string // what the system lookup knows it as
	// strikes are embedded bitmaps, which hide the outlines from Gio.
	strikes bool
	faces   []fontFile
}

type fontBook struct {
	families []*fontFamily // sorted by name
	byName   map[string]*fontFamily
}

func fontFolders() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{"/System/Library/Fonts", "/Library/Fonts", filepath.Join(home, "Library/Fonts")}
	case "windows":
		return []string{filepath.Join(os.Getenv("WINDIR"), "Fonts"), filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Windows", "Fonts")}
	}
	return []string{"/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".local/share/fonts"), filepath.Join(home, ".fonts")}
}

// Apple's own fonts have hidden names.
var appleFamilies = map[string]string{"System Font": "SF Pro", ".SF NS": "SF Pro", ".SF NS Mono": "SF Mono"}

// scanFonts reads just the headers of every installed font.
func scanFonts(folders []string) *fontBook {
	book := &fontBook{byName: map[string]*fontFamily{}}
	var buf []byte
	for _, folder := range folders {
		_ = filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".ttf", ".otf", ".ttc", ".otc":
			default:
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			loaders, err := ot.NewLoaders(f)
			if err != nil {
				return nil
			}
			for i, ld := range loaders {
				var about otfont.Description
				about, buf = otfont.Describe(ld, buf)
				name := about.Family
				if shown, ok := appleFamilies[name]; ok {
					name = shown
				}
				if name == "" || strings.HasPrefix(name, ".") || name == "LastResort" {
					continue
				}
				face := fontFile{
					path: path, index: i,
					weight: int(about.Aspect.Weight),
					italic: about.Aspect.Style == otfont.StyleItalic,
					narrow: about.Aspect.Stretch != 0 && about.Aspect.Stretch != otfont.StretchNormal,
				}
				if face.weight == 0 {
					face.weight = 400
				}
				face.minWeight, face.maxWeight = weightAxis(ld)
				family := book.byName[strings.ToLower(name)]
				if family == nil {
					family = &fontFamily{name: name, query: about.Family}
					book.byName[strings.ToLower(name)] = family
					book.families = append(book.families, family)
				}
				family.strikes = family.strikes || hasTable(ld, "EBDT") || hasTable(ld, "bdat")
				family.faces = append(family.faces, face)
			}
			return nil
		})
	}
	slices.SortFunc(book.families, func(a, b *fontFamily) int {
		return cmp.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})
	return book
}

type fontKey struct {
	path  string
	index int
}

type fontTraits struct{ usable, mono bool }

// Reading character maps is most of the work of a scan, so it waits until a
// font is offered or chosen.
var knownTraits sync.Map // fontKey to fontTraits

func (f *fontFamily) traits() fontTraits {
	file, ok := f.face(400, false)
	if !ok {
		return fontTraits{}
	}
	key := fontKey{file.path, file.index}
	if known, ok := knownTraits.Load(key); ok {
		return known.(fontTraits)
	}
	var t fontTraits
	if r, err := os.Open(file.path); err == nil {
		defer r.Close()
		if loaders, err := ot.NewLoaders(r); err == nil && file.index < len(loaders) {
			t.usable, t.mono = latinCoverage(loaders[file.index])
		}
	}
	knownTraits.Store(key, t)
	return t
}

func hasTable(ld *ot.Loader, tag string) bool {
	_, err := ld.RawTable(ot.MustNewTag(tag))
	return err == nil
}

// latinCoverage reports whether a face can draw ASCII as shapes and whether
// those all have one width. The flag fonts carry for that is often wrong.
func latinCoverage(ld *ot.Loader) (usable, mono bool) {
	if !hasTable(ld, "glyf") && !hasTable(ld, "CFF ") && !hasTable(ld, "CFF2") {
		return false, false // bitmaps only
	}
	raw, err := ld.RawTable(ot.MustNewTag("cmap"))
	if err != nil {
		return false, false
	}
	table, _, err := tables.ParseCmap(raw)
	if err != nil {
		return false, false
	}
	cmap, _, err := otfont.ProcessCmap(table, 0)
	if err != nil {
		return false, false
	}
	hhea, err := ld.RawTable(ot.MustNewTag("hhea"))
	if err != nil || len(hhea) < 36 {
		return false, false
	}
	hmtx, err := ld.RawTable(ot.MustNewTag("hmtx"))
	n := int(binary.BigEndian.Uint16(hhea[34:]))
	if err != nil || n == 0 || len(hmtx) < 4*n {
		return false, false
	}
	width := -1
	mono = true
	for r := rune('!'); r <= '~'; r++ {
		glyph, ok := cmap.Lookup(r)
		if !ok {
			return false, false
		}
		w := int(binary.BigEndian.Uint16(hmtx[4*min(int(glyph), n-1):]))
		if width < 0 {
			width = w
		}
		mono = mono && absInt(w-width)*33 <= max(w, width)
	}
	return true, mono && width > 0
}

// weightAxis is the range of a variable font's weight, or zeros.
func weightAxis(ld *ot.Loader) (int, int) {
	fvar, err := ld.RawTable(ot.MustNewTag("fvar"))
	if err != nil || len(fvar) < 16 {
		return 0, 0
	}
	at := int(binary.BigEndian.Uint16(fvar[4:]))
	count, size := int(binary.BigEndian.Uint16(fvar[8:])), int(binary.BigEndian.Uint16(fvar[10:]))
	for i := range count {
		axis := at + i*size
		if axis+16 > len(fvar) {
			break
		}
		if string(fvar[axis:axis+4]) == "wght" {
			fixed := func(off int) int { return int(int32(binary.BigEndian.Uint32(fvar[axis+off:])) >> 16) }
			return fixed(4), fixed(12)
		}
	}
	return 0, 0
}

func (b *fontBook) find(name string) *fontFamily {
	if name == "" {
		return nil
	}
	return b.byName[strings.ToLower(name)]
}

// weights are the upright weights the family can be set in.
func (f *fontFamily) weights() []int {
	var out []int
	for _, face := range f.faces {
		switch {
		case face.italic || face.narrow:
		case face.variable():
			for w := (face.minWeight + 99) / 100 * 100; w <= face.maxWeight; w += 100 {
				if w >= 100 && w <= 900 {
					out = append(out, w)
				}
			}
		default:
			out = append(out, face.weight)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// face is the one closest to a weight and style.
func (f *fontFamily) face(weight int, italic bool) (fontFile, bool) {
	best, bestScore, found := fontFile{}, 0, false
	for _, face := range f.faces {
		if face.italic != italic {
			continue
		}
		away := absInt(face.weight - weight)
		if face.variable() {
			away = max(0, face.minWeight-weight, weight-face.maxWeight)
		}
		score := 2 * away
		if face.weight < weight && weight >= 600 {
			score++ // of two as close, bold takes the heavier
		}
		if face.narrow {
			score += 5000
		}
		if !found || score < bestScore {
			best, bestScore, found = face, score, true
		}
	}
	return best, found
}

// weighted sets a variable font to one weight.
type weighted struct {
	inner  font.Face
	weight int
}

func (w weighted) Face() *otfont.Face {
	face := w.inner.Face()
	face.SetVariations([]otfont.Variation{{Tag: ot.MustNewTag("wght"), Value: float32(w.weight)}})
	return face
}

func (f fontFile) load(weight int, as font.Font) (font.FontFace, bool) {
	raw, err := os.ReadFile(f.path)
	if err != nil {
		return font.FontFace{}, false
	}
	faces, err := opentype.ParseCollection(withoutStrikes(raw))
	if err != nil || f.index >= len(faces) {
		return font.FontFace{}, false
	}
	face := faces[f.index].Face
	if f.variable() {
		face = weighted{face, min(f.maxWeight, max(f.minWeight, weight))}
	}
	return font.FontFace{Font: as, Face: face}, true
}

// withoutStrikes hides a font's bitmap tables. Gio prefers them to the
// outlines and then can't draw them, which leaves the text blank.
func withoutStrikes(raw []byte) []byte {
	starts := []int{0}
	if len(raw) >= 12 && string(raw[:4]) == "ttcf" {
		starts = starts[:0]
		for i := range int(binary.BigEndian.Uint32(raw[8:])) {
			if 16+4*i > len(raw) {
				return raw
			}
			starts = append(starts, int(binary.BigEndian.Uint32(raw[12+4*i:])))
		}
	}
	for _, at := range starts {
		if at < 0 || at+12 > len(raw) {
			continue
		}
		var outlines bool
		var strikes []int
		for i := range int(binary.BigEndian.Uint16(raw[at+4:])) {
			record := at + 12 + 16*i
			if record+16 > len(raw) {
				break
			}
			switch string(raw[record : record+4]) {
			case "glyf", "CFF ", "CFF2":
				outlines = true
			case "EBDT", "EBLC", "bdat", "bloc":
				strikes = append(strikes, record)
			}
		}
		if outlines {
			for _, record := range strikes {
				raw[record] = 'z' // a tag nothing looks for
			}
		}
	}
	return raw
}

var installedFonts struct {
	sync.Mutex
	book *fontBook
}

func fonts() *fontBook {
	installedFonts.Lock()
	defer installedFonts.Unlock()
	if installedFonts.book == nil {
		installedFonts.book = scanFonts(fontFolders())
	}
	return installedFonts.book
}

// rescanFonts picks up fonts installed while Loom was running.
func rescanFonts() {
	book := scanFonts(fontFolders())
	installedFonts.Lock()
	installedFonts.book = book
	installedFonts.Unlock()
	// Have the picker's list ready by the time it is opened.
	go func() {
		for _, family := range book.families {
			family.traits()
		}
	}()
}

// fontLook is every setting that changes glyphs or their size.
type fontLook struct {
	family, fallback, ui         string
	weight, bold, uiWeight       int
	size, height, spacing, uiPts float32
	ligatures                    bool
}

func lookOf(o options) fontLook {
	return fontLook{
		family: o.FontFamily, fallback: o.FontFallback, ui: o.UIFontFamily,
		weight: o.FontWeight, bold: o.FontBoldWeight, uiWeight: o.UIFontWeight,
		size: float32(o.FontSize), height: float32(o.LineHeight), spacing: float32(o.LetterSpacing), uiPts: float32(o.UIFontSize),
		ligatures: o.FontLigatures,
	}
}

func (l fontLook) sameFaces(o fontLook) bool {
	return l.family == o.family && l.fallback == o.fallback && l.ui == o.ui && l.weight == o.weight && l.bold == o.bold && l.uiWeight == o.uiWeight
}

func pickFamily(book *fontBook, chosen string, defaults ...string) *fontFamily {
	for _, name := range append([]string{chosen}, defaults...) {
		if family := book.find(name); family != nil {
			return family
		}
	}
	return nil
}

var (
	codeDefaults = []string{"Menlo", "DejaVu Sans Mono", "Cascadia Mono", "Consolas"}
	uiDefaults   = []string{"Avenir Next", "Inter", "DejaVu Sans", "Segoe UI"}
)

// loadFonts builds the shaper for d.look.
func (d *desktop) loadFonts() {
	book, l := fonts(), d.look
	var faces []font.FontFace
	add := func(family *fontFamily, alias font.Typeface, weight int, as font.Weight, italic bool) bool {
		if family == nil {
			return false
		}
		file, ok := family.face(weight, italic)
		if !ok {
			return false
		}
		style := font.Regular
		if italic {
			style = font.Italic
		}
		face, ok := file.load(weight, font.Font{Typeface: alias, Weight: as, Style: style})
		if ok {
			faces = append(faces, face)
		}
		return ok
	}

	chosen := l.family
	if family := book.find(chosen); family != nil && !family.traits().mono {
		chosen = "" // code can't sit on a grid in a proportional font
	}
	d.codeFamily = pickFamily(book, chosen, codeDefaults...)
	d.hasItalic = false
	if path := os.Getenv("LOOM_FONT"); path != "" && l.family == "" {
		custom := scanFonts([]string{path})
		if len(custom.families) > 0 {
			d.codeFamily = custom.families[0]
		}
	}
	d.codeFont = font.Font{Typeface: "Go Mono"}
	if add(d.codeFamily, faceCode, l.weight, font.Normal, false) {
		d.codeFont = font.Font{Typeface: faceCode}
		add(d.codeFamily, faceCode, l.bold, font.Bold, false)
		d.hasItalic = add(d.codeFamily, faceCode, l.weight, font.Normal, true)
		if d.hasItalic {
			add(d.codeFamily, faceCode, l.bold, font.Bold, true)
		}
	}

	// A lookup only tries the families it names before turning to the system,
	// so the fallback is named after each font.
	var then font.Typeface
	if fallback := book.find(l.fallback); fallback != nil {
		if add(fallback, faceFallback, l.weight, font.Normal, false) {
			add(fallback, faceFallback, l.bold, font.Bold, false)
			then = ", " + faceFallback
			if d.codeFont.Typeface == faceCode {
				d.codeFont.Typeface += then
			}
		}
	}

	d.uiFamily = pickFamily(book, l.ui, uiDefaults...)
	d.uiFont = font.Font{Typeface: "Go", Weight: font.Medium}
	d.uiStrong = font.Font{Typeface: "Go", Weight: font.Bold}
	if add(d.uiFamily, faceUI, l.uiWeight, font.Normal, false) {
		d.uiFont = font.Font{Typeface: faceUI + then}
		d.uiStrong = d.uiFont
		if add(d.uiFamily, faceUI, min(900, l.uiWeight+100), font.Bold, false) {
			d.uiStrong.Weight = font.Bold
		}
	}

	// Key symbols such as ⌘.
	for _, path := range []string{"/System/Library/Fonts/SFNS.ttf", "/System/Library/Fonts/Apple Symbols.ttf"} {
		faces = append(faces, readFaces(path)...)
	}
	faces = append(faces, gofont.Collection()...)
	d.shaper = text.NewShaper(text.WithCollection(faces))
}

// typeface names an installed family for the system lookup.
func (f *fontFamily) typeface() font.Typeface {
	if f.strikes || strings.HasPrefix(f.query, ".") || strings.ContainsAny(f.query, `"\`) {
		return ""
	}
	return font.Typeface(`"` + f.query + `"`)
}

// ligatureBreak reports whether a run has to be split between a and b to
// keep the font from joining them.
func ligatureBreak(a, b byte) bool {
	plain := func(c byte) bool { return 'a' <= c|0x20 && c|0x20 <= 'z' || '0' <= c && c <= '9' }
	switch {
	case !plain(a) && !plain(b):
		return true
	case b == 'x' && '0' <= a && a <= '9', a == 'x' && '0' <= b && b <= '9': // 0xFF, 2x4
		return true
	}
	return a == 'w' && b == 'w' // www
}
