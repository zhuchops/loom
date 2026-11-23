//go:build desktop

package editor

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// playDemo runs the script named by LOOM_DEMO once the window is up, so the
// front end can be checked without touching the keyboard. Each line is
// "keys a b ctrl+p", "type text", "run command-id", "open path" or "sleep 300".
func (b *Bridge) playDemo() {
	path := os.Getenv("LOOM_DEMO")
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	go func() {
		time.Sleep(600 * time.Millisecond)
		for _, line := range strings.Split(string(data), "\n") {
			verb, rest, _ := strings.Cut(strings.TrimSpace(line), " ")
			switch verb {
			case "keys":
				for _, k := range strings.Fields(rest) {
					b.call(func() {
						if b.settingsHaveKeys() {
							b.settingsKey(keyInput{key: k})
						} else {
							b.m.editor.handle(keyInput{key: k})
						}
					})
				}
			case "type":
				b.call(func() { b.m.editor.handle(keyInput{paste: rest}) })
			case "run":
				b.call(func() {
					if c, ok := b.m.editor.commands[rest]; ok {
						c.run(b.m.editor)
					}
				})
			case "open":
				b.call(func() { b.m.editor.open(rest) })
			case "sleep":
				ms, _ := strconv.Atoi(rest)
				time.Sleep(time.Duration(ms) * time.Millisecond)
			}
			time.Sleep(40 * time.Millisecond)
		}
	}()
}
