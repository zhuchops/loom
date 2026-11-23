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
