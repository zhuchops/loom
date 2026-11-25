package editor

import "testing"

func TestFileIcons(t *testing.T) {
	cases := map[string]string{
		"Cargo.lock": "@lock", "/x/package-lock.json": "@lock", "yarn.lock": "@lock", "pnpm-lock.yaml": "@lock",
		"bun.lockb": "@lock", "go.sum": "@lock", "Gemfile.lock": "@lock", "poetry.lock": "@lock", "whatever.lock": "@lock",
		"Cargo.toml": "@crate", "go.mod": "GO", "package.json": "npm", "pyproject.toml": "PY", "Gemfile": "RB",
		"Dockerfile": "@whale", "Dockerfile.dev": "@whale", "api.dockerfile": "@whale", "docker-compose.prod.yml": "@whale",
		".env": "@key", ".env.local": "@key", ".env.production.local": "@key",
		"README": "@book", "README.md": "@book", "readme.rst": "@book", "LICENSE": "@scale", "LICENSE-MIT": "@scale",
		"CHANGELOG.md": "@clock", "Makefile": "#", "GNUmakefile": "#", "justfile": "#", "CMakeLists.txt": "CM",
		"tsconfig.json": "TS", "tsconfig.build.json": "TS", "vite.config.ts": "@bolt", ".eslintrc.cjs": "@check",
		"eslint.config.mjs": "@check", ".prettierrc": "@brush", "types.d.ts": "DTS", "app.test.ts": "@flask",
		"main.go": "GO", "lib.rs": "RS", "App.tsx": "TSX", "index.MJS": "JS", "photo.JPEG": "@image",
		"backup.tar.gz": "@box", "a.b.c.zip": "@box", "server.key": "@key", "cert.pem": "@cert", "id_ed25519": "@key",
		".bashrc": "$", ".zshrc": "$", ".somethingrc": "@gear", ".gitignore": "git", ".gitignore_global": "git",
		".github": "git", "flake.nix": "NIX", "main.tf": "TF", "data.csv": "@table", "notes.txt": "@lines",
		"song.flac": "@note", "clip.mov": "@film", "Inter.woff2": "@font", "schema.graphql": "GQ", "x.proto": "PB",
		".DS_Store": "@apple", "BUILD.bazel": "BZ", "mix.exs": "EX", "build.gradle.kts": "GR", "Package.swift": "SW",
	}
	for path, want := range cases {
		s, ok := fileStyleOf(path)
		if !ok || s.mark != want {
			t.Errorf("%s: mark %q, want %q", path, s.mark, want)
		}
		if ok && s.nerd == 0 {
			t.Errorf("%s has no terminal glyph", path)
		}
	}
	for _, path := range []string{"", ".", "noext", "a.unknownext", "..", "/"} {
		if s, ok := fileStyleOf(path); ok {
			t.Errorf("%q: unexpected mark %q", path, s.mark)
		}
		if treeIcon(path, false, false) != iconFile {
			t.Errorf("%q: terminal should show the plain file glyph", path)
		}
	}
	// Every mark fits the tree's icon column.
	check := func(s fileStyle) {
		if n := len([]rune(s.mark)); n == 0 || s.mark[0] != '@' && n > 3 {
			t.Errorf("mark %q is too long", s.mark)
		}
	}
	for _, s := range fileNames {
		check(s)
	}
	for _, s := range fileExts {
		check(s)
	}
	for _, p := range filePrefixes {
		check(p.style)
	}
}
