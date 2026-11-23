package editor

import (
	"strings"
	"testing"
)

func TestCompletionOnlyAutoOpensAfterLetters(t *testing.T) {
	e := testEditor(t, "vim", "")
	e.b.path = "/tmp/x.go"
	press(e, "i")
	press(e, "{ ,")
	e.scheduleCompletion(",")
	if autoTriggerAt(e.b) {
		t.Fatal("punctuation counted as an automatic trigger")
	}
	press(e, "f")
	if !autoTriggerAt(e.b) {
		t.Fatal("letter did not count as an automatic trigger")
	}
}

func TestCompletionFiltersWhileTyping(t *testing.T) {
	e := testEditor(t, "vim", "")
	press(e, "i")
	e.completion.all = []completionItem{{Label: "Println"}, {Label: "Printf"}, {Label: "Sprint"}, {Label: "Fprint"}}
	e.completion.start = 0
	e.completion.items = filterCompletion(e.completion.all, "")
	press(e, "p")
	if got := labels(e.completion.items); got != "Printf,Println,Sprint,Fprint" {
		t.Fatalf("after p: %s", got)
	}
	if e.completion.token == 0 {
		t.Fatal("refresh did not bump the token")
	}
	press(e, "r")
	if got := labels(e.completion.items); got != "Printf,Println,Sprint,Fprint" {
		t.Fatalf("after pr: %s", got)
	}
	press(e, "x")
	if len(e.completion.items) != 0 {
		t.Fatalf("expected no items after prx, got %s", labels(e.completion.items))
	}
}

func TestCompletionAcceptReplacesWholeWord(t *testing.T) {
	e := testEditor(t, "vim", "fo")
	press(e, "A")
	item := completionItem{Label: "foobar", TextEdit: &textEdit{Range: &lspRange{Start: position{0, 0}, End: position{0, 2}}, NewText: "foobar"}}
	if err := applyCompletion(e.b, item); err != nil {
		t.Fatal(err)
	}
	if got := string(e.b.text); got != "foobar" {
		t.Fatalf("accept: %q", got)
	}
}

func labels(items []completionItem) string {
	out := ""
	for i, it := range items {
		if i > 0 {
			out += ","
		}
		out += it.Label
	}
	return out
}

func TestCompletionFunctionGetsParens(t *testing.T) {
	b := &buffer{anchor: -1, text: []rune("fmt.Prin"), cursor: 8}
	b.indexLines()
	if err := applyCompletion(b, completionItem{Label: "Println", Kind: 3}); err != nil {
		t.Fatal(err)
	}
	if got := string(b.text); got != "fmt.Println()" || b.cursor != 12 {
		t.Fatalf("got %q cursor %d", got, b.cursor)
	}
	b = &buffer{anchor: -1, text: []rune("Prin(x)"), cursor: 4}
	b.indexLines()
	applyCompletion(b, completionItem{Label: "Println", Kind: 3})
	if got := string(b.text); got != "Println(x)" {
		t.Fatalf("got %q", got)
	}
	b = &buffer{anchor: -1, text: []rune("Na"), cursor: 2}
	b.indexLines()
	applyCompletion(b, completionItem{Label: "Name", Kind: 5})
	if got := string(b.text); got != "Name" {
		t.Fatalf("got %q", got)
	}
}

func TestCompletionParensCanBeTypedOver(t *testing.T) {
	e := testEditor(t, "vim", "fmt.Prin")
	e.mode = "insert"
	e.b.cursor = 8
	applyCompletion(e.b, completionItem{Label: "Println", Kind: 3})
	e.typeRune(')')
	if got := string(e.b.text); got != "fmt.Println()" || e.b.cursor != 13 {
		t.Fatalf("got %q cursor %d", got, e.b.cursor)
	}
}

func TestCompletionVimKeys(t *testing.T) {
	e := testEditor(t, "vim", "Pr")
	e.mode = "insert"
	e.b.cursor = 2
	e.completion.items = []completionItem{{Label: "Print"}, {Label: "Printf"}, {Label: "Println"}}
	e.completion.start = 0
	e.handle(keyInput{key: "ctrl+n"})
	e.handle(keyInput{key: "ctrl+n"})
	e.handle(keyInput{key: "ctrl+p"})
	if e.completion.selected != 1 {
		t.Fatalf("selected %d", e.completion.selected)
	}
	e.handle(keyInput{key: "ctrl+y"})
	if string(e.b.text) != "Printf" {
		t.Fatalf("got %q", e.b.text)
	}
}

func TestCompletionNoArgumentsPutsCursorAfterBrackets(t *testing.T) {
	cases := []struct {
		detail string
		cursor int // in "run()"
	}{
		{"func() string", 5},
		{"func(name string) string", 4},
		{"(method) Foo.run(): void", 5},
		{"(method) Foo.run(x: number): void", 4},
		{"", 4}, // no signature: assume arguments
	}
	for _, c := range cases {
		b := &buffer{anchor: -1, text: []rune("run"), cursor: 3}
		b.indexLines()
		if err := applyCompletion(b, completionItem{Label: "run", Kind: 3, Detail: c.detail}); err != nil {
			t.Fatal(err)
		}
		if string(b.text) != "run()" || b.cursor != c.cursor {
			t.Errorf("%q: got %q cursor %d, want %d", c.detail, b.text, b.cursor, c.cursor)
		}
	}
}

func TestCompletionOpensAfterDot(t *testing.T) {
	e := testEditor(t, "vim", "")
	e.b.path = "/tmp/x.ex"
	press(e, "i")
	press(e, "I O . ")
	e.b.cursor = len(e.b.text) // "IO."
	if !autoTriggerAt(e.b) {
		t.Fatalf("a dot after a name should trigger completion: %q", string(e.b.text))
	}
	for _, text := range []string{"1.", "..", ". ", " ."} {
		e.b.text, e.b.cursor = []rune(text), len([]rune(text))
		if autoTriggerAt(e.b) && text != ". " {
			t.Errorf("%q should not trigger completion", text)
		}
	}
}

func TestSnippetCallsBecomePlainCompletions(t *testing.T) {
	items := plainCompletions([]completionItem{
		{Label: "puts(item)", Kind: 3, InsertTextFormat: 2, TextEdit: &textEdit{NewText: "puts(${1:item})"}},
		{Label: "for", Kind: 15, InsertTextFormat: 2, InsertText: "for ${1:x} in ${2:y} do\n$0\nend"},
		{Label: "plain", Kind: 1},
	})
	if got := labels(items); got != "puts(item),plain" {
		t.Fatalf("items = %s", got)
	}
	if items[0].TextEdit.NewText != "puts" || items[0].InsertTextFormat != 1 {
		t.Fatalf("call not flattened: %+v", items[0].TextEdit)
	}
	if takesArguments(completionItem{Label: "run()", Detail: "(function)"}) {
		t.Error("run() takes no arguments")
	}
}

func TestCompletionRangeOffTheCursorLineIsIgnoredAndImportMoved(t *testing.T) {
	e := testEditor(t, "vim", "")
	b := e.b
	b.path = "/tmp/main.dingo"
	text := "package main\n\nfunc main() {\n    fmt.Pr\n}\n"
	b.text = []rune(text)
	b.indexLines()
	b.cursor = strings.Index(text, "Pr") + 2
	item := completionItem{
		Label: "Println", Kind: 3,
		// A range from the top of the file, like positions of the generated Go file.
		TextEdit:   &textEdit{Range: &lspRange{Start: position{0, 0}, End: position{0, 5}}, NewText: "Println"},
		Additional: []textEdit{{Range: &lspRange{Start: position{0, 0}, End: position{0, 12}}, NewText: "import \"fmt\"\n"}},
		Detail:     "func(a ...any)",
	}
	if err := applyCompletionAt(b, item, wordStartAt(b, b.cursor)); err != nil {
		t.Fatal(err)
	}
	if got, want := string(b.text), "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println()\n}\n"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}
