package brand

import (
	"image"
	"testing"
)

func TestEverySize(t *testing.T) {
	for size := range 8 {
		Icon(size) // too small to show a weave, but must not panic
		Mark(size)
	}
	// The smallest the app asks for is the 14 pt logo at the smallest UI size.
	for _, size := range []int{8, 9, 10, 11, 12, 14, 16, 18, 21, 24, 28, 32, 36, 40, 41, 64, 65, 120, 180} {
		for name, img := range map[string]*image.RGBA{"icon": Icon(size), "mark": Mark(size)} {
			shown := false
			for i := 0; i < len(img.Pix); i += 4 {
				r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
				if r > a || g > a || b > a {
					t.Fatalf("%s at %d: pixel %d is not premultiplied", name, size, i/4)
				}
				shown = shown || a > 0
			}
			if !shown {
				t.Fatalf("%s at %d is blank", name, size)
			}
			if size >= 14 && img.RGBAAt(size-1, 0).A > 60 {
				t.Fatalf("%s at %d fills its transparent corner", name, size)
			}
		}
	}
}

func TestIconsetKeepsTheMoonInTheTile(t *testing.T) {
	for _, size := range []int{128, 256, 512, 1024} {
		img := Icon(size)
		if img.RGBAAt(0, 0).A != 0 || img.RGBAAt(size/2, size/2).A != 255 {
			t.Fatalf("icon at %d: want a solid tile with clear corners", size)
		}
		m := layout(size, true)
		px := func(d vec) vec { return vec{m.origin.x + d.x*m.unit, m.origin.y + d.y*m.unit} }
		r := m.threadR() * m.unit
		for _, d := range []vec{threadFrom, threadTo, {litCentre.x - litR, litCentre.y}, {litCentre.x, litCentre.y + litR}} {
			if tileEdge(px(d), m.size) > -r {
				t.Fatalf("icon at %d: the mark leaves the tile at %v", size, d)
			}
		}
	}
}

func TestTheThreadIsWovenThroughTheMoon(t *testing.T) {
	m := layout(1024, false)
	over, under := false, false
	for k := 0.0; k <= 1; k += 0.001 {
		d := vec{threadFrom.x + (threadTo.x-threadFrom.x)*k, threadFrom.y + (threadTo.y-threadFrom.y)*k}
		if _, on := m.moonAt(d); on {
			if k < threadUnder {
				over = true
			} else {
				under = true
			}
		}
	}
	if !over || !under {
		t.Fatalf("the thread should cross both horns: over %v, under %v", over, under)
	}
}
