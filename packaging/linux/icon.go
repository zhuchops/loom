//go:build ignore

// icon.go writes the Loom icon as one PNG, for the Linux desktop entry.
// Usage: go run packaging/linux/icon.go out.png [size]
package main

import (
	"image/png"
	"log"
	"os"
	"strconv"

	"github.com/zhuchops/loom/internal/brand"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: icon out.png [size]")
	}
	size := 256
	if len(os.Args) > 2 {
		n, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		size = n
	}
	f, err := os.Create(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, brand.Icon(size)); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}
