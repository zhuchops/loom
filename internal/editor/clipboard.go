package editor

import (
	"bytes"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// The register is mirrored to the system clipboard, like Vim's
// clipboard=unnamedplus. These are variables so tests never touch the real one.
var (
	clipboardCopy  = systemCopy
	clipboardPaste = systemPaste
)

type clipboardTool struct{ copy, paste []string }

func clipboardTools() []clipboardTool {
	switch runtime.GOOS {
	case "darwin":
		return []clipboardTool{{[]string{"pbcopy"}, []string{"pbpaste"}}}
	case "windows":
		return []clipboardTool{{[]string{"clip.exe"}, []string{"powershell.exe", "-NoProfile", "-Command", "Get-Clipboard -Raw"}}}
	}
	return []clipboardTool{
		{[]string{"wl-copy"}, []string{"wl-paste", "--no-newline"}},
		{[]string{"xclip", "-selection", "clipboard"}, []string{"xclip", "-selection", "clipboard", "-o"}},
		{[]string{"xsel", "--clipboard", "--input"}, []string{"xsel", "--clipboard", "--output"}},
	}
}

// systemCopy reports whether a native tool took the text.
func systemCopy(text string) bool {
	for _, t := range clipboardTools() {
		if _, err := exec.LookPath(t.copy[0]); err != nil {
			continue
		}
		cmd := exec.Command(t.copy[0], t.copy[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if cmd.Run() == nil {
			return true
		}
	}
	return false
}

func systemPaste() (string, bool) {
	for _, t := range clipboardTools() {
		if _, err := exec.LookPath(t.paste[0]); err != nil {
			continue
		}
		cmd := exec.Command(t.paste[0], t.paste[1:]...)
		var out bytes.Buffer
		cmd.Stdout = &out
		done := make(chan error, 1)
		go func() { done <- cmd.Run() }()
		select {
		case err := <-done:
			if err == nil {
				return out.String(), true
			}
		case <-time.After(500 * time.Millisecond):
			_ = cmd.Process.Kill()
		}
	}
	return "", false
}

// setRegister stores yanked text and mirrors it to the system clipboard. The
// OSC52 copy goes out too, which is what reaches the local clipboard over SSH.
func (e *editor) setRegister(text string, linewise bool, tags []int) {
	e.register, e.linewise, e.registerTags = text, linewise, tags
	e.clipboardOut, e.clipboardSet = text, true
	clipboardCopy(text)
}

// syncRegister makes the register follow the system clipboard. Text copied in
// another application replaces it as plain characters; text we copied ourselves
// keeps its linewise flag.
func (e *editor) syncRegister() {
	text, ok := clipboardPaste()
	if !ok || text == "" || text == e.register {
		return
	}
	e.register, e.linewise, e.registerTags = text, false, nil
}
