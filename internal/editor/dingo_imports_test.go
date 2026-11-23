package editor

import (
	"os/exec"
	"strings"
	"testing"
)

func TestPackageBeforeDot(t *testing.T) {
	for text, want := range map[string]string{
		"    fmt.Pr": "fmt", "x := strings.": "strings", "a.b.c": "", "Fmt.Pr": "", "fmt": "", "(fmt.Pr": "fmt",
	} {
		got, ok := packageBeforeDot([]rune(text), len([]rune(text)))
		if got != want || ok != (want != "") {
			t.Errorf("%q = %q, %v; want %q", text, got, ok, want)
		}
	}
}

func TestImportEdit(t *testing.T) {
	apply := func(src string) string {
		b := &buffer{text: []rune(src)}
		b.indexLines()
		e, ok := importEdit(b, "fmt")
		if !ok {
			return "(none)"
		}
		return string(b.text[:e.start]) + e.text + string(b.text[e.end:])
	}
	for name, c := range map[string]struct{ src, want string }{
		"block":   {"package main\n\nimport (\n\t\"os\"\n)\n", "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n"},
		"single":  {"package main\n\nimport \"os\"\n\nfunc f() {}\n", "package main\n\nimport \"os\"\nimport \"fmt\"\n\nfunc f() {}\n"},
		"none":    {"package main\n\nfunc f() {}\n", "package main\n\nimport \"fmt\"\n\nfunc f() {}\n"},
		"no pkg":  {"func f() {}\n", "import \"fmt\"\n\nfunc f() {}\n"},
		"already": {"import \"fmt\"\n", "(none)"},
	} {
		if got := apply(c.src); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", name, got, c.want)
		}
	}
}

func TestUnimportedCompletionAddsTheImport(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command")
	}
	src := "package main\n\nfunc main() {\n    fmt.Pr\n}\n"
	b := &buffer{text: []rune(src), path: "/tmp/main.dingo"}
	b.indexLines()
	b.cursor = strings.Index(src, "Pr") + 2
	items := unimportedCompletions(b.text, b.cursor)
	var chosen completionItem
	for _, item := range items {
		if item.Label == "Println" {
			chosen = item
		}
	}
	if chosen.Label == "" || chosen.importPath != "fmt" {
		t.Fatalf("no fmt.Println among %d items", len(items))
	}
	if err := applyCompletionAt(b, chosen, wordStartAt(b, b.cursor)); err != nil {
		t.Fatal(err)
	}
	if want := "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println()\n}\n"; string(b.text) != want {
		t.Fatalf("text = %q, want %q", string(b.text), want)
	}
	if got := string(b.text[b.cursor-1]); got != ")" && string(b.text[b.cursor]) != ")" {
		t.Errorf("cursor not in the call: %q", string(b.text[b.cursor-2:b.cursor+2]))
	}
	// An imported package is the server's business.
	if got := unimportedCompletions([]rune("import \"fmt\"\nfmt.P"), 17); len(got) != 0 {
		t.Errorf("offered %d items for an imported package", len(got))
	}
}

func TestServerImportEditIsMovedIntoTheFile(t *testing.T) {
	for text, want := range map[string]string{
		"import \"fmt\"\n": "fmt", "\t\"net/http\"\n": "net/http", "import (\n\t\"fmt\"\n)\n": "fmt",
		"import str \"strings\"\n": "strings", "foo()\n": "", "": "",
	} {
		if got := importedPath(text); got != want {
			t.Errorf("importedPath(%q) = %q, want %q", text, got, want)
		}
	}
	src := "package main\n\nfunc main() {\n    fmt.prin\n}\n"
	b := &buffer{text: []rune(src), path: "/tmp/main.dingo"}
	b.indexLines()
	b.cursor = strings.Index(src, "prin") + 4
	// What gopls sends for an unimported package: positions of the Go file.
	item := completionItem{
		Label: "Println", Kind: 3, Detail: "func(a ...any)",
		TextEdit:   &textEdit{Range: &lspRange{Start: position{9, 4}, End: position{9, 8}}, NewText: "Println"},
		Additional: []textEdit{{Range: &lspRange{Start: position{2, 0}, End: position{2, 0}}, NewText: "import \"fmt\"\n"}},
	}
	if err := applyCompletionAt(b, item, wordStartAt(b, b.cursor)); err != nil {
		t.Fatal(err)
	}
	if want := "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println()\n}\n"; string(b.text) != want {
		t.Fatalf("text = %q, want %q", string(b.text), want)
	}
}

func TestDingoBareWordCompletions(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command")
	}
	src := "package main\n\nfunc main() {\n    pac\n}\n"
	cursor := strings.Index(src, "pac") + 3
	items := bareWordCompletions([]rune(src), cursor, nil)
	labels := map[string]completionItem{}
	for _, item := range items {
		labels[item.Label+"|"+item.Detail] = item
	}
	if _, ok := labels["package|keyword"]; !ok {
		t.Error("the keyword package is missing")
	}
	// A package whose name starts with the prefix, with its import path.
	src = "package main\n\nfunc main() {\n    strin\n}\n"
	items = bareWordCompletions([]rune(src), strings.Index(src, "strin")+5, nil)
	found := false
	for _, item := range items {
		if item.Label == "strings" && item.importPath == "strings" && item.Kind == 9 {
			found = true
		}
	}
	if !found {
		t.Errorf("strings package not offered: %d items", len(items))
	}
	// Not after a dot, not for what is already known, not for what is imported.
	if got := bareWordCompletions([]rune("x.pac"), 5, nil); got != nil {
		t.Errorf("offered %d items after a dot", len(got))
	}
	if got := bareWordCompletions([]rune("import \"strings\"\nstrin"), 20, nil); len(got) != 0 {
		for _, item := range got {
			if item.Label == "strings" {
				t.Error("an imported package was offered")
			}
		}
	}
	// Accepting adds the import.
	b := &buffer{text: []rune(src), path: "/tmp/m.dingo"}
	b.indexLines()
	b.cursor = strings.Index(src, "strin") + 5
	for _, item := range items {
		if item.Label == "strings" {
			if err := applyCompletionAt(b, item, wordStartAt(b, b.cursor)); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if want := "package main\n\nimport \"strings\"\n\nfunc main() {\n    strings\n}\n"; string(b.text) != want {
		t.Fatalf("text = %q", string(b.text))
	}
}
