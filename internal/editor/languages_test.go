package editor

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestElixirSupport(t *testing.T) {
	cfg := defaultConfig()
	for path, want := range map[string]string{"lib/app.ex": "elixir", "mix.exs": "elixir", "page.html.heex": "heex", "a.eex": "heex"} {
		if got := cfg.languageFor(path); got != want {
			t.Errorf("languageFor(%q) = %q, want %q", path, got, want)
		}
	}
	if got := cfg.Languages["elixir"].Server; len(got) != 2 || got[0] != "expert" || got[1] != "--stdio" {
		t.Fatalf("elixir server = %v, want expert --stdio", got)
	}
	if got := cfg.Languages["heex"].Server; len(got) == 0 || got[0] != "expert" {
		t.Fatalf("heex server = %v", got)
	}
	if len(cfg.Languages["elixir"].Format) == 0 {
		t.Fatal("elixir should format with mix format")
	}
	if got := scopeLabel("  defp valid?(user) do"); got != "valid?" {
		t.Errorf("scope label = %q", got)
	}
	if got := scopeLabel("defmodule MyApp.Accounts do"); got != "MyApp.Accounts" {
		t.Errorf("module label = %q", got)
	}
	e := testEditor(t, "vim", "")
	e.open(fixture(t, e.root, "a.ex", ""))
	for before, want := range map[string]bool{
		"def run(x) do": true, "  fn x ->": true, "case y do": true, "else": true, "x = 1": false, "  foo(1) # do": false,
	} {
		if got := e.indentAfter(before) != ""; got != want {
			t.Errorf("indentAfter(%q) indents = %v, want %v", before, got, want)
		}
	}
}

func TestCrystalSupport(t *testing.T) {
	cfg := defaultConfig()
	if got := cfg.languageFor("src/app.cr"); got != "crystal" {
		t.Fatalf("languageFor = %q", got)
	}
	c := cfg.Languages["crystal"]
	if len(c.Server) != 1 || c.Server[0] != "crystalline" {
		t.Fatalf("crystal server = %v", c.Server)
	}
	if strings.Join(c.Format, " ") != "crystal tool format -" || !slices.Contains(c.Roots, "shard.yml") {
		t.Fatalf("crystal format = %v, roots = %v", c.Format, c.Roots)
	}
	if got := scopeLabel("  macro define_getter(name)"); got != "define_getter" {
		t.Errorf("macro label = %q", got)
	}
	e := testEditor(t, "vim", "")
	e.open(fixture(t, e.root, "a.cr", ""))
	for before, want := range map[string]bool{
		"def run(x : Int32) : Int32": true, "  private def helper": true, "class Foo < Bar": true, "struct Point": true,
		"  if x > 1": true, "else": true, "  when 1": true, "items.each do |item|": true, "spawn do": true,
		"x = if y": true, "lib LibC": true, "x = 1": false, "return if done": false, "def x; 1; end": false,
		"abstract def area : Float64": false, "  foo(1) # do": false, "end": false,
	} {
		if got := e.indentAfter(before) != ""; got != want {
			t.Errorf("indentAfter(%q) indents = %v, want %v", before, got, want)
		}
	}
}

func TestNimSupport(t *testing.T) {
	cfg := defaultConfig()
	for _, path := range []string{"src/app.nim", "config.nims", "app.nimble"} {
		if got := cfg.languageFor(path); got != "nim" {
			t.Errorf("languageFor(%q) = %q", path, got)
		}
	}
	n := cfg.Languages["nim"]
	if len(n.Server) != 1 || n.Server[0] != "nimlangserver" || strings.Join(n.Format, " ") != "nph -" {
		t.Fatalf("nim server = %v, format = %v", n.Server, n.Format)
	}
	// A Nimble package is rooted at the folder of its <name>.nimble file.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.nimble"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := workspaceRoot(filepath.Join(root, "src", "app", "x.nim"), "nim", n.Roots); got != root {
		t.Errorf("workspaceRoot = %q, want %q", got, root)
	}
	if got := scopeLabel("proc parse*(s: string): Node ="); got != "parse" {
		t.Errorf("proc label = %q", got)
	}
	e := testEditor(t, "vim", "")
	e.open(fixture(t, e.root, "a.nim", ""))
	for before, want := range map[string]bool{
		"proc add(a, b: int): int =": true, "type": true, "  Point = object": true, "Node = ref object of RootObj": true,
		"  if x > 1:": true, "  of 1:": true, "for i in 0..<n:": true, "Color = enum": true, "let": true,
		"x = 1": false, "if a == b": false, "echo x # note:": false, "discard": false,
	} {
		if got := e.indentAfter(before) != ""; got != want {
			t.Errorf("indentAfter(%q) indents = %v, want %v", before, got, want)
		}
	}
}

func TestDingoSupport(t *testing.T) {
	cfg := defaultConfig()
	if got := cfg.languageFor("cmd/main.dingo"); got != "dingo" {
		t.Fatalf("languageFor = %q", got)
	}
	if got := cfg.languageFor("cmd/main.go"); got != "go" {
		t.Fatalf("go files changed: %q", got)
	}
	if got := cfg.Languages["dingo"].Server; len(got) != 1 || got[0] != "dingo-lsp" {
		t.Fatalf("server = %v", got)
	}
}
