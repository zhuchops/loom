//go:build desktop

package editor

import (
	"image/color"
	"path/filepath"
	"runtime"
	"strings"
)

// shortcutText shows the platform's own spelling of a window shortcut.
func shortcutText(mac, other string) string {
	if runtime.GOOS == "darwin" {
		return mac
	}
	return other
}

func bufferName(b *buffer) string {
	switch {
	case b.dashboard:
		return "Welcome"
	case b.path == "":
		return "Untitled"
	}
	return filepath.Base(b.path)
}

// fileMark is a short coloured monogram for a file type.
func fileMarkOf(c *uiColors, path string) (string, color.NRGBA) {
	switch strings.ToLower(filepath.Base(path)) {
	case "makefile", "dockerfile":
		return "#", c.orange
	case "mix.exs", "mix.lock":
		return "EX", c.purple
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
	case ".dingo":
		return "DG", c.orange
	case ".ex", ".exs":
		return "EX", c.purple
	case ".heex", ".eex", ".leex":
		return "<>", c.purple
	}
	return "", c.dim
}

func languageName(id string) string {
	names := map[string]string{
		"": "Plain Text", "go": "Go", "mod": "Go Module", "gomod": "Go Module", "md": "Markdown", "markdown": "Markdown",
		"js": "JavaScript", "javascript": "JavaScript", "ts": "TypeScript", "typescript": "TypeScript", "tsx": "TSX", "jsx": "JSX",
		"py": "Python", "python": "Python", "rs": "Rust", "rust": "Rust", "yml": "YAML", "yaml": "YAML", "json": "JSON",
		"html": "HTML", "css": "CSS", "sh": "Shell", "c": "C", "cpp": "C++", "toml": "TOML", "sql": "SQL",
		"asm": "Assembly", "cmake": "CMake", "csharp": "C#", "dockerfile": "Dockerfile", "graphql": "GraphQL",
		"javascriptreact": "JavaScript JSX", "makefile": "Makefile", "ocaml": "OCaml", "php": "PHP", "proto": "Protocol Buffers",
		"shellscript": "Shell", "dingo": "Dingo", "elixir": "Elixir", "ex": "Elixir", "exs": "Elixir", "heex": "HEEx", "typescriptreact": "TypeScript TSX", "xml": "XML",
	}
	if name, ok := names[strings.ToLower(id)]; ok {
		return name
	}
	return humanize(id)
}
