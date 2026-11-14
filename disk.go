package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type diskState struct {
	exists bool
	text   string
}

var errDiskChanged = errors.New("file changed on disk")

func readDisk(path string) (diskState, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return diskState{}, nil
	}
	if err != nil {
		return diskState{}, err
	}
	if !info.Mode().IsRegular() {
		return diskState{}, fmt.Errorf("expected a regular file: %s", path)
	}
	raw, err := os.ReadFile(path)
	return diskState{true, string(raw)}, err
}

func (b *buffer) diskMatches(d diskState) bool { return b.savedExists == d.exists && b.saved == d.text }

func (e *editor) checkDisk() {
	for _, b := range e.buffers {
		if b.path == "" {
			continue
		}
		info, err := os.Lstat(b.path)
		if err == nil && b.observed != nil && os.SameFile(b.observed, info) && b.observed.ModTime() == info.ModTime() && b.observed.Size() == info.Size() && b.observed.Mode() == info.Mode() {
			continue
		}
		if err == nil {
			b.observed = info
		} else {
			b.observed = nil
		}
		d, err := readDisk(b.path)
		if err == nil && b.diskMatches(d) {
			b.external = false
			continue
		}
		if err == nil && d.exists && !b.dirty() && utf8.ValidString(d.text) && !strings.ContainsRune(d.text, 0) {
			e.reloadBuffer(b, d)
		} else if !b.external {
			b.external = true
			if b == e.b {
				e.notify("File changed on disk")
			}
		}
	}
}

func (e *editor) reloadBuffer(b *buffer, d diskState) {
	row := b.rowAt(b.cursor)
	col := b.cursor - b.lines[row]
	b.mergeUndo = false
	b.lastInsert = time.Time{}
	b.restore(snapshot{text: []rune(d.text), cursor: 0})
	b.saved, b.savedExists, b.external = d.text, d.exists, false
	b.undo, b.redo = nil, nil
	row = min(row, len(b.lines)-1)
	b.cursor = min(b.lines[row]+col, b.lineEnd(row))
	b.top = min(b.top, len(b.lines)-1)
	if b == e.b {
		e.clampModalCursor()
		e.invalidateCompletion()
		e.scheduleLanguage()
		e.refreshGit()
		e.notify("Reloaded " + filepath.Base(b.path))
	}
}

func (e *editor) reviewDisk(b *buffer, d diskState, quit bool) {
	if !d.exists {
		e.notify("File was removed on disk; review before recreating")
	}
	m := &menu{title: "Disk Changes · " + filepath.Base(b.path)}
	m.submit = func(string) {
		if len(m.matches) > 0 && m.selected < len(m.matches) {
			if run := m.matches[m.selected].run; run != nil {
				run()
			} else {
				e.openMenu(m)
			}
		}
	}
	m.options = []option{
		{"Keep Editing", "leave disk untouched", func() { e.notify("Kept unsaved buffer") }},
		{"Overwrite Disk", "save the buffer version", func() {
			if e.b != b {
				return
			}
			if e.saveAgainst(&d, quit) && quit {
				e.tryQuit(false)
			}
		}},
	}
	if d.exists && utf8.ValidString(d.text) && !strings.ContainsRune(d.text, 0) {
		m.options = append(m.options, option{"Reload from Disk", "discard editor changes", func() {
			current, err := readDisk(b.path)
			if err != nil || current != d {
				e.notify("Disk changed again; reopen the review")
				return
			}
			e.reloadBuffer(b, d)
		}})
	}
	old, next := strings.Split(d.text, "\n"), strings.Split(string(b.text), "\n")
	first := 0
	for first < min(len(old), len(next)) && old[first] == next[first] {
		first++
	}
	lastOld, lastNext := len(old), len(next)
	for lastOld > first && lastNext > first && old[lastOld-1] == next[lastNext-1] {
		lastOld--
		lastNext--
	}
	for i, line := range old[first:lastOld] {
		if i == 150 {
			break
		}
		m.options = append(m.options, option{fmt.Sprintf("− Disk %d", first+i+1), line, nil})
	}
	for i, line := range next[first:lastNext] {
		if i == 150 {
			break
		}
		m.options = append(m.options, option{fmt.Sprintf("+ Editor %d", first+i+1), line, nil})
	}
	e.openMenu(m)
}

func (e *editor) saveAgainst(expected *diskState, quit bool) bool {
	b := e.b
	if b.dashboard {
		e.notify("Open a file before saving")
		return false
	}
	var formatErr error
	if b.path != "" {
		d, err := readDisk(b.path)
		if err != nil {
			e.notify(err.Error())
			return false
		}
		if expected != nil && d != *expected {
			e.notify("Disk changed again; review the new version")
			e.reviewDisk(b, d, quit)
			return false
		}
		if expected == nil && !b.diskMatches(d) {
			b.external = true
			if d.exists && !b.dirty() && utf8.ValidString(d.text) && !strings.ContainsRune(d.text, 0) {
				e.reloadBuffer(b, d)
				return false
			}
			e.reviewDisk(b, d, quit)
			return false
		}
		expected = &d
		if e.cfg.FormatOnSave || e.cfg.OrganizeImports {
			formatErr = e.formatBuffer()
		}
	}
	if err := b.save(expected); err != nil {
		e.notify(err.Error())
		if errors.Is(err, errDiskChanged) {
			if d, readErr := readDisk(b.path); readErr == nil {
				e.reviewDisk(b, d, quit)
			}
		}
		return false
	}
	message := "Saved " + filepath.Base(b.path)
	if formatErr != nil {
		message += " · " + formatErr.Error()
	}
	e.notify(message)
	e.didSave(b)
	e.refreshTree(b.path)
	e.scheduleLanguage()
	e.refreshGit()
	if e.recovery != nil {
		_ = e.saveRecovery()
	}
	return true
}

func (e *editor) addRecoveredBuffer(b *buffer) {
	if e.b.path == "" && !e.b.dashboard && !e.b.dirty() && len(e.buffers) == 1 && e.layout == e.activePane {
		e.buffers = []*buffer{b}
	} else if !slices.Contains(e.buffers, b) {
		e.buffers = append(e.buffers, b)
	}
	e.switchBuffer(b)
}
