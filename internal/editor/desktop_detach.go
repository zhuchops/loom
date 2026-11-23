//go:build desktop && unix

package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"
)

// foreground is set by -foreground: keep the terminal attached.
var foreground bool

const detachedMark = "LOOM_DETACHED"

// detachFromTerminal reruns the program in the background when it was started
// from a terminal, so the shell gets its prompt back and Loom's log lines go
// to a file instead of the screen. It reports that this process has nothing
// more to do. Started from Finder or a launcher there is no terminal, and the
// program just runs.
func detachFromTerminal() (done bool, err error) {
	if foreground || os.Getenv(detachedMark) != "" {
		return false, nil
	}
	if !term.IsTerminal(os.Stdin.Fd()) && !term.IsTerminal(os.Stdout.Fd()) && !term.IsTerminal(os.Stderr.Fd()) {
		return false, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return false, nil // run here instead
	}
	logPath := desktopLogPath()
	var out *os.File
	if logPath != "" && os.MkdirAll(filepath.Dir(logPath), 0o700) == nil {
		out, _ = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	}
	if out == nil {
		if out, err = os.OpenFile(os.DevNull, os.O_WRONLY, 0); err != nil {
			return false, nil
		}
	}
	defer out.Close()
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = append(os.Environ(), detachedMark+"=1")
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return false, nil
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		// It stopped at once: a bad flag or a config it could not use.
		if err == nil {
			return true, nil
		}
		if text, readErr := os.ReadFile(logPath); readErr == nil && len(text) > 0 {
			fmt.Fprint(os.Stderr, string(text))
		}
		os.Exit(1)
	case <-time.After(800 * time.Millisecond):
	}
	return true, nil
}

func desktopLogPath() string {
	root := stateRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "desktop.log")
}
