package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFoldHidesBlockAndJSkipsIt(t *testing.T) {
	e := testEditor(t, "vim", "func a() {\n\tx\n\ty\n}\nnext")
	press(e, "z c")
	if hidden := e.b.hiddenRows(); hidden == nil || !hidden[1] || !hidden[3] || hidden[0] || hidden[4] {
		t.Fatalf("closed fold hid the wrong rows: %v", hidden)
	}
	press(e, "j")
	if row := e.b.rowAt(e.b.cursor); row != 4 {
		t.Fatalf("j should skip the fold, landed on row %d", row)
	}
	press(e, "k")
	if row := e.b.rowAt(e.b.cursor); row != 0 {
		t.Fatalf("k should land on the fold header, landed on row %d", row)
	}
	press(e, "z o")
	if e.b.hiddenRows() != nil {
		t.Fatal("z o did not open the fold")
	}
}

func TestFoldToggleAndCloseAll(t *testing.T) {
	e := testEditor(t, "vim", "a\n  b\n  c\nd\n  e")
	press(e, "j z a")
	if !e.b.folds.closed[0] && !e.b.folds.closed[1] {
		t.Fatal("z a did not close the fold under the cursor")
	}
	press(e, "z R")
	if e.b.hiddenRows() != nil {
		t.Fatal("z R should open all folds")
	}
	press(e, "z M")
	if hidden := e.b.hiddenRows(); hidden == nil || !hidden[1] || !hidden[4] {
		t.Fatalf("z M should close every fold: %v", hidden)
	}
}

func TestFoldDropsWhenLinesChange(t *testing.T) {
	e := testEditor(t, "vim", "f {\n\tx\n}")
	press(e, "z c")
	press(e, "o")
	press(e, "esc")
	if e.b.hiddenRows() != nil {
		t.Fatal("folds should reset when a line is added")
	}
}

func TestPathCompletionListsDirectory(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "lib"), 0o755)
	os.WriteFile(filepath.Join(dir, "main.go"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, ".hidden"), nil, 0o644)
	e := testEditor(t, "vim", "")
	e.b.path = filepath.Join(dir, "main.go")
	e.b.text = []rune(`import "./`)
	e.b.indexLines()
	e.b.cursor = len(e.b.text)
	e.mode = "insert"
	if !e.pathCompletion() {
		t.Fatal("cursor in ./ string should use path completion")
	}
	if got := labels(e.completion.items); got != "lib/,main.go" {
		t.Fatalf("entries: %s", got)
	}
	e.b.text = []rune(`import "./li`)
	e.b.indexLines()
	e.b.cursor = len(e.b.text)
	e.pathCompletion()
	if got := labels(e.completion.items); got != "lib/" {
		t.Fatalf("filtered entries: %s", got)
	}
	if err := applyCompletionAt(e.b, e.completion.items[0], e.completion.start); err != nil {
		t.Fatal(err)
	}
	if got := string(e.b.text); got != `import "./lib/` {
		t.Fatalf("accept: %q", got)
	}
}

func TestPathCompletionIgnoresPlainStrings(t *testing.T) {
	e := testEditor(t, "vim", "")
	e.b.path = filepath.Join(t.TempDir(), "x.go")
	e.b.text = []rune(`fmt.Println("hello`)
	e.b.indexLines()
	e.b.cursor = len(e.b.text)
	e.mode = "insert"
	if e.pathCompletion() {
		t.Fatal("plain text string should not use path completion")
	}
}

func TestFlattenSymbolsOutline(t *testing.T) {
	raw := `[{"name":"Server","kind":23,"range":{"start":{"line":2,"character":0},"end":{"line":9,"character":1}},"selectionRange":{"start":{"line":2,"character":5},"end":{"line":2,"character":11}},"children":[{"name":"Start","kind":6,"range":{"start":{"line":4,"character":1},"end":{"line":6,"character":2}},"selectionRange":{"start":{"line":4,"character":1},"end":{"line":4,"character":6}}}]}]`
	var syms []symbolInfo
	if err := json.Unmarshal([]byte(raw), &syms); err != nil {
		t.Fatal(err)
	}
	flat := flattenSymbols(syms, "/x/server.go", 0, nil)
	if len(flat) != 2 || flat[0].name != "Server" || flat[0].kind != "struct" || flat[0].line != 2 || flat[0].char != 5 {
		t.Fatalf("top level: %+v", flat)
	}
	if flat[1].name != "Start" || flat[1].depth != 1 || flat[1].kind != "method" {
		t.Fatalf("child: %+v", flat[1])
	}
	if !strings.HasSuffix(flat[1].path, "server.go") {
		t.Fatal("fallback path not used")
	}
}

func TestFoldMarkersShowFoldableRows(t *testing.T) {
	e := testEditor(t, "vim", "func a() {\n\tx\n}\nplain\nmore\nif y {\n  z\n}")
	ends := e.b.foldEnds()
	if end, ok := ends[0]; !ok || end != 2 {
		t.Fatalf("func header should fold through its closing brace: %v", ends)
	}
	if _, ok := ends[3]; ok {
		t.Fatal("plain line with no block below should not be foldable")
	}
	if _, ok := ends[5]; !ok {
		t.Fatal("if header should be foldable")
	}
}

func TestDollarKeepsEndOfLineAcrossEmptyLine(t *testing.T) {
	e := testEditor(t, "vim", "hello\n\nworld!")
	press(e, "$ j")
	if e.b.cursor != 6 {
		t.Fatalf("empty line: cursor %d", e.b.cursor)
	}
	press(e, "k")
	if e.b.cursor != 4 {
		t.Fatalf("k from empty line should return to end of 'hello', got %d", e.b.cursor)
	}
	press(e, "j j")
	if e.b.cursor != 12 {
		t.Fatalf("should stay at end of 'world!', got %d", e.b.cursor)
	}
}

func TestColumnMemoryAcrossShortLine(t *testing.T) {
	e := testEditor(t, "vim", "abcdef\nab\nabcdef")
	press(e, "l l l l j")
	if e.b.cursor != 8 {
		t.Fatalf("short line clamp: %d", e.b.cursor)
	}
	press(e, "j")
	if col := e.b.cursor - e.b.lines[2]; col != 4 {
		t.Fatalf("column should return to 4, got %d", col)
	}
	press(e, "h k")
	if col := e.b.cursor - e.b.lines[1]; col != 1 {
		t.Fatalf("after h the old column must be forgotten, got %d", col)
	}
}
