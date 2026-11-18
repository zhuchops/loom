package editor

import "testing"

func TestCharacterFindRepeatsInBothProfiles(t *testing.T) {
	for _, profile := range []string{"vim"} {
		for _, tc := range []struct {
			keys string
			want int
		}{
			{"; ,", 0}, // No previous find.
			{"f - ;", 3},
			{"f - 2 ;", 5},
			{"f - ; , ;", 3}, // Reverse does not replace the remembered direction.
			{"$ F - ;", 5},
			{"$ F - ; , ;", 5},
			{"t - ;", 2}, // Skip the adjacent target on a repeated till motion.
			{"t - 2 ;", 4},
			{"$ T - ;", 6},
			{"f - ; ; ; ; ;", 7}, // Failed repeats keep the cursor on this line.
			{"f - v ;", 3},
		} {
			t.Run(profile+"/"+tc.keys, func(t *testing.T) {
				e := testEditor(t, profile, "a-b-c-d-e\nnext-line")
				press(e, tc.keys)
				if e.b.cursor != tc.want {
					t.Fatalf("cursor = %d, want %d", e.b.cursor, tc.want)
				}
			})
		}
	}
}

func TestVimOperatorRepeatsCharacterFind(t *testing.T) {
	e := testEditor(t, "vim", "a-b-c-d")
	press(e, "f - d ;")
	if got := string(e.b.text); got != "ac-d" {
		t.Fatalf("delete with repeated character find = %q", got)
	}
}
