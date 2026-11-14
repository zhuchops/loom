package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

type recoveredTab struct {
	Path, Base, Text string
	Exists           bool
	Cursor           int
}

type recoveryState struct{ root, path string }

func (e *editor) startRecovery() {
	path := e.sessionPath()
	if path == "" {
		return
	}
	root := strings.TrimSuffix(path, ".json") + "-recovery"
	e.recovery = &recoveryState{root, filepath.Join(root, fmt.Sprintf("%d-%d.json", os.Getpid(), time.Now().UnixNano()))}
	e.offerRecovery()
}

func (e *editor) saveRecovery() error {
	r := e.recovery
	if r == nil {
		return nil
	}
	var tabs []recoveredTab
	for _, b := range e.buffers {
		if b.dirty() {
			tabs = append(tabs, recoveredTab{b.path, b.saved, string(b.text), b.savedExists, b.cursor})
		}
	}
	if len(tabs) == 0 {
		err := os.Remove(r.path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(r.root, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(r.root, ".recovery-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(tabs)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), r.path)
}

func (e *editor) offerRecovery() {
	if e.recovery == nil {
		return
	}
	type candidate struct {
		path string
		tabs []recoveredTab
	}
	var candidates []candidate
	files, _ := os.ReadDir(e.recovery.root)
	for _, file := range files {
		pid, _, ok := strings.Cut(file.Name(), "-")
		if !ok || file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		n, err := strconv.Atoi(pid)
		if err != nil || n <= 0 {
			continue
		}
		if err := syscall.Kill(n, 0); err != syscall.ESRCH {
			continue
		}
		path := filepath.Join(e.recovery.root, file.Name())
		raw, err := os.ReadFile(path)
		var tabs []recoveredTab
		if err == nil && json.Unmarshal(raw, &tabs) == nil && len(tabs) > 0 {
			candidates = append(candidates, candidate{path, tabs})
		}
	}
	if len(candidates) == 0 {
		return
	}
	m := &menu{title: "Recover Unsaved Buffers"}
	for _, c := range candidates {
		name := filepath.Base(c.tabs[0].Path)
		if c.tabs[0].Path == "" {
			name = "Untitled"
		}
		label := fmt.Sprintf("%s · %d buffers", name, len(c.tabs))
		m.options = append(m.options, option{"Restore " + label, "", func() { e.restoreRecovery(c.path, c.tabs) }})
		m.options = append(m.options, option{"Discard " + label, "", func() {
			e.openMenu(&menu{title: "Discard This Recovery Snapshot?", confirmation: true, submit: func(string) {
				if err := os.Remove(c.path); err != nil {
					e.notify(err.Error())
					return
				}
				e.offerRecovery()
			}})
		}})
	}
	e.openMenu(m)
}

func (e *editor) restoreRecovery(path string, tabs []recoveredTab) {
	restored := 0
	for _, tab := range tabs {
		if !utf8.ValidString(tab.Text) || tab.Path != "" && !filepath.IsAbs(tab.Path) {
			continue
		}
		d := diskState{}
		var err error
		if tab.Path != "" {
			d, err = readDisk(tab.Path)
		}
		if err == nil && d.exists && d.text == tab.Text {
			continue
		}
		var b *buffer
		for _, existing := range e.buffers {
			if tab.Path != "" && existing.path == tab.Path {
				b = existing
				break
			}
		}
		if b != nil && b.dirty() {
			b, _ = loadBuffer("")
		} else if b == nil {
			b, _ = loadBuffer("")
			b.path = tab.Path
		}
		b.saved, b.savedExists = tab.Base, tab.Exists
		b.external = err != nil || !b.diskMatches(d)
		b.restore(snapshot{text: []rune(tab.Text), cursor: max(0, tab.Cursor)})
		e.addRecoveredBuffer(b)
		restored++
	}
	if err := e.saveRecovery(); err != nil {
		e.notify("Recovery restored in memory: " + err.Error())
		return
	}
	if err := os.Remove(path); err != nil {
		e.notify(err.Error())
		return
	}
	e.notify(fmt.Sprintf("Restored %d unsaved buffers", restored))
}
