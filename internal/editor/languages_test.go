package editor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultLanguageLookup(t *testing.T) {
	cfg := defaultConfig()
	for path, want := range map[string]string{
		"main.go": "go", "app.tsx": "typescriptreact", "tool.py": "python",
		"lib.rs": "rust", "style.css": "css", "notes.txt": "txt",
	} {
		if got := cfg.languageFor(path); got != want {
			t.Errorf("languageFor(%q) = %q, want %q", path, got, want)
		}
	}
	if len(cfg.Languages["python"].Server) == 0 || len(cfg.Languages["python"].Format) == 0 {
		t.Fatal("python should have a default server and formatter")
	}
}

func TestLanguageConfigOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{
		"languages": {
			"zig": {"extensions": ["zig"], "server": ["zls"], "format": ["zig", "fmt", "--stdin"]},
			"python": {"extensions": ["py3"]}
		},
		"language_servers": {"ruby": ["solargraph", "stdio"]}
	}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := readConfig(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.languageFor("build.zig"); got != "zig" {
		t.Fatalf("new language = %q", got)
	}
	if got := cfg.languageFor("tool.py3"); got != "python" {
		t.Fatalf("overridden extension = %q", got)
	}
	if got := cfg.languageFor("tool.py"); got == "python" {
		t.Fatal("python kept an extension it no longer owns")
	}
	if len(cfg.Languages["python"].Server) == 0 {
		t.Fatal("override dropped the default python server")
	}
	if got := cfg.Languages["ruby"].Server; len(got) != 2 || got[0] != "solargraph" {
		t.Fatalf("legacy language_servers ignored: %v", got)
	}
}

func TestExternalFormatterRuns(t *testing.T) {
	e := testEditor(t, "vim", "")
	e.cfg.Languages = map[string]languageConfig{"text": {Extensions: []string{"txt"}, Format: []string{"tr", "a-z", "A-Z"}}}
	e.open(fixture(t, e.root, "note.txt", "hello world\n"))
	if err := e.formatBuffer(); err != nil {
		t.Fatal(err)
	}
	if got := string(e.b.text); got != "HELLO WORLD\n" {
		t.Fatalf("formatted text = %q", got)
	}
}

func TestMissingFormatterIsSkipped(t *testing.T) {
	e := testEditor(t, "vim", "")
	e.cfg.Languages = map[string]languageConfig{"text": {Extensions: []string{"txt"}, Format: []string{"loom-no-such-formatter"}}}
	e.open(fixture(t, e.root, "note.txt", "unchanged\n"))
	if err := e.formatBuffer(); err != nil {
		t.Fatalf("missing formatter should not fail: %v", err)
	}
	if got := string(e.b.text); got != "unchanged\n" {
		t.Fatalf("text changed: %q", got)
	}
}

func TestFormatterPreservesCRLF(t *testing.T) {
	e := testEditor(t, "vim", "old\r\n")
	e.replaceFormatted("old\r\n", "new\r\nsecond\n")
	if got := string(e.b.text); got != "new\r\nsecond\r\n" {
		t.Fatalf("formatted text = %q", got)
	}
}
