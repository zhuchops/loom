package editor

import "testing"

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
