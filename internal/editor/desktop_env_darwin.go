//go:build desktop && darwin

package editor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// platformEnvironment repairs what an app started from Finder lacks: the
// shell's PATH, without which no language server is found, and a UTF-8
// locale, without which pbcopy garbles anything but ASCII.
func platformEnvironment() {
	if os.Getenv("LANG") == "" && os.Getenv("LC_ALL") == "" {
		os.Setenv("LANG", "en_US.UTF-8")
	}
	if os.Getenv("TERM") != "" {
		return // started from a shell
	}
	var dirs []string
	add := func(list ...string) {
		for _, p := range list {
			if filepath.IsAbs(p) && !slices.Contains(dirs, p) {
				dirs = append(dirs, p)
			}
		}
	}
	if shell := os.Getenv("SHELL"); filepath.IsAbs(shell) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		const mark = "__LOOM_PATH__"
		cmd := exec.CommandContext(ctx, shell, "-ilc", `printf '`+mark+`%s`+mark+`' "$PATH"`)
		cmd.WaitDelay = 200 * time.Millisecond
		out, _ := cmd.Output()
		if parts := strings.Split(string(out), mark); len(parts) == 3 {
			add(strings.Split(parts[1], ":")...)
		}
	}
	add(strings.Split(os.Getenv("PATH"), ":")...)
	add("/opt/homebrew/bin", "/usr/local/bin", "/usr/local/go/bin")
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, "go/bin"), filepath.Join(home, ".local/bin"), filepath.Join(home, ".cargo/bin"))
	}
	os.Setenv("PATH", strings.Join(dirs, ":"))
}
