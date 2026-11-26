//go:build desktop && !darwin

package editor

import (
	"os"
	"runtime"
	"unsafe"
)

// platformEnvironment turns off WebKitGTK's DMA-BUF renderer, which makes
// the window flicker on many Linux drivers and compositors. Setting the
// variable yourself wins.
func platformEnvironment() {
	if runtime.GOOS == "linux" {
		if _, set := os.LookupEnv("WEBKIT_DISABLE_DMABUF_RENDERER"); !set {
			_ = os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
		}
	}
}

func raiseApp() {}

func setAppearance(unsafe.Pointer, bool) {}

func installWindow(unsafe.Pointer, bool) {}

func installDock() {}

func quietTyping() {}
