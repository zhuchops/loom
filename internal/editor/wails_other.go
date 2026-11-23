//go:build wails && !darwin

package editor

// platformEnvironment has nothing to repair outside macOS.
func platformEnvironment() {}
