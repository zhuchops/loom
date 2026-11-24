//go:build desktop && !darwin

package editor

// platformEnvironment has nothing to repair outside macOS.
func platformEnvironment() {}

func raiseWindow() {}

func setAppearance(bool) {}

func installWindow() {}

func quietTyping() {}
