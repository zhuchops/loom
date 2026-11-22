//go:build ignore

// icon.go writes the Loom iconset. It leaves out 16 and 32 points on purpose:
// given those, macOS 27 sets the icon on a black plate at small sizes. Without
// them it scales the larger artwork down and keeps the shape.
// Usage: go run packaging/macos/icon.go out.iconset
package main

import (
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/zhuchops/loom/internal/brand"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: icon out.iconset")
	}
	for _, points := range []int{128, 256, 512} {
		for scale := 1; scale <= 2; scale++ {
			name := fmt.Sprintf("icon_%dx%d", points, points)
			if scale == 2 {
				name += "@2x"
			}
			if err := write(filepath.Join(os.Args[1], name+".png"), points*scale); err != nil {
				log.Fatal(err)
			}
		}
	}
}

func write(path string, size int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, brand.Icon(size)); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
