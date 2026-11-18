package editor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// A plugin is an executable that reads one JSON request and writes one reply.
type pluginRequest struct {
	Version   int       `json:"version"`
	Path      string    `json:"path"`
	Text      string    `json:"text"`
	Cursor    position  `json:"cursor"`
	Selection *lspRange `json:"selection,omitempty"`
}

type pluginReply struct {
	Message string     `json:"message"`
	Edits   []textEdit `json:"edits"`
}

func (e *editor) registerPlugins() error {
	for _, plugin := range e.cfg.Plugins {
		if plugin.ID == "" || strings.ContainsAny(plugin.ID, " \t\r\n:") || len(plugin.Command) == 0 || plugin.Command[0] == "" {
			return fmt.Errorf("plugin requires an id and command")
		}
		id := "plugin." + plugin.ID
		if _, ok := e.commands[id]; ok {
			return fmt.Errorf("duplicate plugin %s", id)
		}
		title := plugin.Title
		if title == "" {
			title = id
		}
		e.add(id, title, func(e *editor) { e.runPlugin(plugin) })
	}
	return nil
}

func (e *editor) runPlugin(plugin pluginConfig) {
	if e.pluginRunning {
		e.notify("A plugin is already running")
		return
	}
	e.pluginRunning = true
	e.notify("Running " + plugin.ID)
	b, version, path := e.b, e.b.version, e.b.path
	request := pluginRequest{Version: 1, Path: path, Text: string(b.text), Cursor: b.lspPosition(b.cursor)}
	if b.anchor >= 0 {
		s := b.selection()
		request.Selection = &lspRange{b.lspPosition(s.start), b.lspPosition(s.end)}
	}
	root := e.root
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		payload, _ := json.Marshal(request)
		argv := plugin.Command
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Dir = root
		cmd.Stdin = bytes.NewReader(payload)
		out, err := cmd.Output()
		var reply pluginReply
		if err == nil {
			err = json.Unmarshal(out, &reply)
		}
		e.async <- func() {
			e.pluginRunning = false
			if err != nil {
				e.notify("Plugin " + plugin.ID + ": " + err.Error())
				return
			}
			if e.b != b || b.version != version || b.path != path {
				e.notify("Plugin result discarded after buffer change")
				return
			}
			if err := applyPlugin(b, reply.Edits); err != nil {
				e.notify(err.Error())
				return
			}
			if len(reply.Edits) > 0 {
				e.invalidateCompletion()
				e.clampModalCursor()
				e.scheduleLanguage()
				e.scheduleGit()
			}
			message := reply.Message
			if message == "" {
				message = "Finished " + plugin.ID
			}
			e.notify(message)
		}
	}()
}

func applyPlugin(b *buffer, changes []textEdit) error {
	if len(changes) == 0 {
		return nil
	}
	var edits []edit
	for _, change := range changes {
		if change.Range == nil {
			return fmt.Errorf("plugin edit is missing a range")
		}
		start, ok := b.offset(change.Range.Start)
		end, valid := b.offset(change.Range.End)
		if !ok || !valid || end < start {
			return fmt.Errorf("plugin returned an invalid UTF-16 range")
		}
		edits = append(edits, edit{span{start, end}, change.NewText})
	}
	slices.SortStableFunc(edits, func(a, b edit) int { return a.start - b.start })
	cursor := b.cursor
	for i, change := range edits {
		if i > 0 && (edits[i-1].end > change.start || edits[i-1].start == change.start) {
			return fmt.Errorf("plugin edits overlap")
		}
		if change.start <= b.cursor {
			cursor += len([]rune(change.text)) - (min(b.cursor, change.end) - change.start)
		}
	}
	b.replaceAll(edits, cursor)
	return nil
}
