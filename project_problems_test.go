package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func publish(t *testing.T, e *editor, path string, severities ...int) {
	t.Helper()
	var diags []map[string]any
	for _, s := range severities {
		diags = append(diags, map[string]any{
			"range":    map[string]any{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 0, "character": 1}},
			"severity": s,
			"message":  "problem",
		})
	}
	params, err := json.Marshal(map[string]any{"uri": fileURI(path), "diagnostics": diags})
	if err != nil {
		t.Fatal(err)
	}
	e.handleNotification(rpcMessage{Method: "textDocument/publishDiagnostics", Params: params})
}

func TestProjectProblemsCountUnopenedFiles(t *testing.T) {
	e := testEditor(t, "vim", "")
	path := fixture(t, e.root, "unopened.go", "package p\n")
	publish(t, e, path, 1, 2)
	awaitUpdate(t, e, func() bool { return len(e.problems) == 1 })
	if got := e.projectProblems(); got != (problemCount{errors: 1, warnings: 1}) {
		t.Fatalf("project problems = %+v", got)
	}
	publish(t, e, path)
	awaitUpdate(t, e, func() bool { return len(e.problems) == 0 })
	if got := e.projectProblems(); got != (problemCount{}) {
		t.Fatalf("cleared problems = %+v", got)
	}
}

func TestProblemsOutsideProjectAreIgnored(t *testing.T) {
	e := testEditor(t, "vim", "")
	outside := filepath.Join(t.TempDir(), "elsewhere.go")
	publish(t, e, outside, 1)
	(<-e.async)()
	if len(e.problems) != 0 {
		t.Fatalf("file outside the project was counted: %v", e.problems)
	}
}

func TestGoplsReportsProjectErrorsForUnopenedFiles(t *testing.T) {
	if os.Getenv("LOOM_TEST_GOPLS") != "1" {
		t.Skip("set LOOM_TEST_GOPLS=1 to run against gopls")
	}
	e := testEditor(t, "vim", "")
	t.Cleanup(e.closeServers)
	fixture(t, e.root, "go.mod", "module demo\n\ngo 1.22\n")
	fixture(t, e.root, "bad.go", "package main\n\nfunc run() {\n\tx := 1\n}\n")
	fixture(t, e.root, "other.go", "package main\n\nfunc helper() int { return \"s\" }\n")
	e.open(fixture(t, e.root, "main.go", "package main\n\nfunc main() {}\n"))
	e.refreshLanguage()
	awaitUpdate(t, e, func() bool { return e.projectProblems().errors >= 2 })
	if got := e.b.diagnostics; len(got) != 0 {
		t.Fatalf("open clean file has diagnostics: %v", got)
	}
	e.showProjectProblems()
	if e.menu == nil || len(e.menu.options) < 2 {
		t.Fatalf("project problems popup missing entries: %v", e.menu)
	}
}
