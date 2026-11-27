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
	style, ok := fileStyleOf(path)
	if !ok {
		return "", c.dim
	}
	inks := map[byte]color.NRGBA{'r': c.red, 'y': c.yellow, 'g': c.green, 'b': c.blue, 'p': c.purple, 'o': c.orange, 'd': c.dim}
	return style.mark, inks[style.ink]
}

func languageName(id string) string {
	names := map[string]string{
		"": "Plain Text", "go": "Go", "mod": "Go Module", "gomod": "Go Module", "md": "Markdown", "markdown": "Markdown",
		"js": "JavaScript", "javascript": "JavaScript", "ts": "TypeScript", "typescript": "TypeScript", "tsx": "TSX", "jsx": "JSX",
		"py": "Python", "python": "Python", "rs": "Rust", "rust": "Rust", "yml": "YAML", "yaml": "YAML", "json": "JSON",
		"html": "HTML", "css": "CSS", "sh": "Shell", "c": "C", "cpp": "C++", "toml": "TOML", "sql": "SQL",
		"asm": "Assembly", "cmake": "CMake", "csharp": "C#", "dockerfile": "Dockerfile", "graphql": "GraphQL",
		"javascriptreact": "JavaScript JSX", "makefile": "Makefile", "ocaml": "OCaml", "php": "PHP", "proto": "Protocol Buffers",
		"shellscript": "Shell", "dingo": "Dingo", "elixir": "Elixir", "crystal": "Crystal", "nim": "Nim", "cr": "Crystal", "ex": "Elixir", "exs": "Elixir", "heex": "HEEx", "typescriptreact": "TypeScript TSX", "xml": "XML",
	}
	if name, ok := names[strings.ToLower(id)]; ok {
		return name
	}
	return humanize(id)
}
