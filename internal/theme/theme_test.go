package theme

import "testing"

func TestThemesAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, th := range All {
		if seen[th.Name] {
			t.Fatalf("duplicate theme %s", th.Name)
		}
		seen[th.Name] = true
		if (th.BG == th.FG && !th.Terminal) || th.Red == th.BG || th.Accent == th.BG {
			t.Fatalf("%s has unreadable colors", th.Name)
		}
	}
	for _, name := range []string{"everforest", "gruvbox", "nord", "dracula", "tokyo-night", "catppuccin", "one-dark", "monokai", "solarized-dark", "solarized-light", "terminal"} {
		if !seen[name] {
			t.Fatalf("missing theme %s", name)
		}
	}
	if _, ok := Find("NORD"); !ok {
		t.Fatal("Find should ignore case")
	}
}
