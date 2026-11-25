package editor

// Material Design icons from Nerd Fonts. Use a Nerd Font Mono variant in the terminal.
const (
	iconFolder     = "\U000f024b"
	iconFolderOpen = "\U000f0770"
	iconFile       = "\U000f09ee"
)

func treeIcon(path string, dir, open bool) string {
	if dir {
		if open {
			return iconFolderOpen
		}
		return iconFolder
	}
	if style, ok := fileStyleOf(path); ok {
		return string(style.nerd)
	}
	return iconFile
}
