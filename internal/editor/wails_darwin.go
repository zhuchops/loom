//go:build wails && darwin

package editor

// Wails' file dialogs use UTType, which lives in its own framework; link it
// here so a plain go build works without CGO_LDFLAGS.

// #cgo LDFLAGS: -framework UniformTypeIdentifiers
import "C"
