package editor

import (
	"path/filepath"
	"strings"
)

// Material Design icons from Nerd Fonts. Use a Nerd Font Mono variant in the terminal.
const (
	iconFolder     = "\U000f024b"
	iconFolderOpen = "\U000f0770"
	iconFile       = "\U000f09ee"
)

var treeNameIcons = map[string]string{
	"makefile": "\U000f1064", "dockerfile": "\U000f0868", ".gitignore": "\U000f02a2", ".gitattributes": "\U000f02a2",
	".env": "\U000f0493", "go.mod": "\U000f03d6", "go.sum": "\U000f033e", "package.json": "\U000f03d6", "license": "\U000f0fc3",
}

var treeExtIcons = map[string]string{
	".go": "\U000f07d3", ".js": "\U000f031e", ".mjs": "\U000f031e", ".cjs": "\U000f031e", ".jsx": "\U000f031e",
	".ts": "\U000f06e6", ".tsx": "\U000f06e6", ".py": "\U000f0320", ".rs": "\U000f1617",
	".rb": "\U000f0d2d", ".java": "\U000f0b37", ".c": "\U000f0671", ".h": "\U000f0671",
	".cc": "\U000f0671", ".cpp": "\U000f0671", ".hpp": "\U000f0671",
	".md": "\U000f0354", ".markdown": "\U000f0354", ".json": "\U000f0626", ".jsonc": "\U000f0626",
	".yaml": "\U000f0493", ".yml": "\U000f0493", ".toml": "\U000f0493", ".ini": "\U000f0493",
	".conf": "\U000f0493", ".cfg": "\U000f0493",
	".sh": "\U000f018d", ".bash": "\U000f018d", ".zsh": "\U000f018d", ".fish": "\U000f018d",
	".html": "\U000f031d", ".htm": "\U000f031d", ".css": "\U000f031c", ".scss": "\U000f031c", ".less": "\U000f031c",
	".sql": "\U000f01bc", ".lua": "\U000f08b1",
	".png": "\U000f02e9", ".jpg": "\U000f02e9", ".jpeg": "\U000f02e9", ".gif": "\U000f02e9",
	".webp": "\U000f02e9", ".ico": "\U000f02e9", ".svg": "\U000f0721",
	".pdf": "\U000f0226", ".zip": "\U000f05c4", ".tar": "\U000f05c4", ".gz": "\U000f05c4", ".tgz": "\U000f05c4", ".7z": "\U000f05c4",
	".lock": "\U000f033e",
}

func treeIcon(path string, dir, open bool) string {
	if dir {
		if open {
			return iconFolderOpen
		}
		return iconFolder
	}
	name := strings.ToLower(filepath.Base(path))
	if icon, ok := treeNameIcons[name]; ok {
		return icon
	}
	if icon, ok := treeExtIcons[filepath.Ext(name)]; ok {
		return icon
	}
	return iconFile
}
