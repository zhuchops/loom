// Package theme holds the built-in color themes as plain data.
package theme

import "strings"

// Color is a 24-bit RGB value, or with bit 24 set an ANSI palette index, or
// with bit 25 set the terminal's default foreground or background.
type Color uint32

const (
	ANSI    Color = 1 << 24
	Default Color = 1 << 25
)

// Theme names every color the editor draws with.
type Theme struct {
	Name                                             string
	Terminal                                         bool
	BG, FG, Muted, Accent                            Color
	Panel, Menu, Active, Line, Border                Color
	Sel, Match, Flash, Badge                         Color
	Red, Yellow, Aqua, Type, Orange, Purple, Comment Color
}

func hex(name string, c ...int32) Theme {
	t := Theme{Name: name}
	for i, p := range []*Color{&t.BG, &t.FG, &t.Muted, &t.Accent, &t.Panel, &t.Menu, &t.Active, &t.Line, &t.Border,
		&t.Sel, &t.Match, &t.Flash, &t.Badge, &t.Red, &t.Yellow, &t.Aqua, &t.Type, &t.Orange, &t.Purple, &t.Comment} {
		*p = Color(c[i])
	}
	return t
}

func ansiColor(n int) Color { return ANSI | Color(n) }

// Terminal uses the terminal's own background and 16 ANSI colors.
func Terminal() Theme {
	def := Default
	return Theme{
		Name: "terminal", Terminal: true,
		BG: def, FG: def, Muted: ansiColor(8), Accent: ansiColor(2),
		Panel: def, Menu: def, Active: ansiColor(8), Line: ansiColor(0), Border: ansiColor(8),
		Sel: ansiColor(4), Match: ansiColor(3), Flash: ansiColor(11), Badge: ansiColor(0),
		Red: ansiColor(1), Yellow: ansiColor(3), Aqua: ansiColor(6), Type: ansiColor(4),
		Orange: ansiColor(9), Purple: ansiColor(5), Comment: ansiColor(8),
	}
}

// Order: bg fg muted accent panel menu active line border sel match flash badge red yellow aqua typ orange purple comment.
var All = []Theme{
	hex("nvchad", 0x1e222a, 0xabb2bf, 0x6f737b, 0x61afef, 0x1b1f27, 0x252931, 0x353b45, 0x2a2e36, 0x353b45,
		0x3e4451, 0x4a4527, 0x665c30, 0x1e222a, 0xe06c75, 0xe7c787, 0x56b6c2, 0xe5c07b, 0xd19a66, 0xc882e7, 0x565c64),
	hex("everforest", 0x2d353b, 0xd3c6aa, 0x89968b, 0xa7c080, 0x272e33, 0x343f44, 0x4b5558, 0x343f44, 0x475258,
		0x543a48, 0x514d39, 0x696146, 0x232a2e, 0xe67e80, 0xdbbc7f, 0x7fbbb3, 0x83c092, 0xe69875, 0xd699b6, 0x9da9a0),
	hex("gruvbox", 0x282828, 0xebdbb2, 0x928374, 0xb8bb26, 0x1d2021, 0x32302f, 0x665c54, 0x3c3836, 0x504945,
		0x504945, 0x4a4326, 0x6b5d2f, 0x282828, 0xfb4934, 0xfabd2f, 0x8ec07c, 0x83a598, 0xfe8019, 0xd3869b, 0x928374),
	hex("nord", 0x2e3440, 0xd8dee9, 0x7b88a1, 0xa3be8c, 0x272c36, 0x3b4252, 0x4c566a, 0x3b4252, 0x434c5e,
		0x434c5e, 0x5e5a3f, 0x7a7249, 0x2e3440, 0xbf616a, 0xebcb8b, 0x8fbcbb, 0x81a1c1, 0xd08770, 0xb48ead, 0x616e88),
	hex("dracula", 0x282a36, 0xf8f8f2, 0x6272a4, 0x50fa7b, 0x21222c, 0x343746, 0x565a75, 0x343746, 0x44475a,
		0x44475a, 0x4f4a2a, 0x6e6636, 0x282a36, 0xff5555, 0xf1fa8c, 0x8be9fd, 0x8be9fd, 0xffb86c, 0xbd93f9, 0x6272a4),
	hex("tokyo-night", 0x1a1b26, 0xc0caf5, 0x565f89, 0x9ece6a, 0x16161e, 0x24283b, 0x33467c, 0x24283b, 0x3b4261,
		0x283457, 0x4a4426, 0x665c30, 0x1a1b26, 0xf7768e, 0xe0af68, 0x7dcfff, 0x2ac3de, 0xff9e64, 0xbb9af7, 0x636da6),
	hex("catppuccin", 0x1e1e2e, 0xcdd6f4, 0x7f849c, 0xa6e3a1, 0x181825, 0x313244, 0x45475a, 0x313244, 0x45475a,
		0x3b3f5c, 0x4a4533, 0x665d3e, 0x1e1e2e, 0xf38ba8, 0xf9e2af, 0x94e2d5, 0x89b4fa, 0xfab387, 0xcba6f7, 0x6c7086),
	hex("one-dark", 0x282c34, 0xabb2bf, 0x6b7280, 0x98c379, 0x21252b, 0x2c313a, 0x3e4451, 0x2c313a, 0x3e4451,
		0x3a4258, 0x4a4527, 0x665c30, 0x282c34, 0xe06c75, 0xe5c07b, 0x56b6c2, 0x61afef, 0xd19a66, 0xc678dd, 0x7f848e),
	hex("monokai", 0x272822, 0xf8f8f2, 0x75715e, 0xa6e22e, 0x1e1f1c, 0x3e3d32, 0x5a5a3e, 0x3e3d32, 0x49483e,
		0x49483e, 0x5a5a2a, 0x75752f, 0x272822, 0xf92672, 0xe6db74, 0x66d9ef, 0x66d9ef, 0xfd971f, 0xae81ff, 0x88846f),
	hex("solarized-dark", 0x002b36, 0x839496, 0x586e75, 0x859900, 0x00212b, 0x073642, 0x0f4a5a, 0x073642, 0x0f4a5a,
		0x124b5a, 0x3d4300, 0x5c6400, 0x002b36, 0xdc322f, 0xb58900, 0x2aa198, 0x268bd2, 0xcb4b16, 0x6c71c4, 0x657b83),
	hex("solarized-light", 0xfdf6e3, 0x586e75, 0x93a1a1, 0x859900, 0xeee8d5, 0xeee8d5, 0xddd6c1, 0xeee8d5, 0xd3cbb7,
		0xd9e2c8, 0xf0e08a, 0xe8d160, 0xfdf6e3, 0xdc322f, 0xb58900, 0x2aa198, 0x268bd2, 0xcb4b16, 0x6c71c4, 0x93a1a1),
	Terminal(),
}

// Find returns the theme with the given name, ignoring case.
func Find(name string) (Theme, bool) {
	for _, t := range All {
		if strings.EqualFold(t.Name, name) {
			return t, true
		}
	}
	return Theme{}, false
}
