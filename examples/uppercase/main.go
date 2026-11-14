package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf16"
)

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type selection struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

func byteOffset(text string, pos position) (int, bool) {
	line, start := 0, 0
	for line < pos.Line {
		next := strings.IndexByte(text[start:], '\n')
		if next < 0 {
			return 0, false
		}
		start += next + 1
		line++
	}
	column := 0
	for i, r := range text[start:] {
		if column == pos.Character {
			return start + i, true
		}
		if r == '\n' || r == '\r' {
			return 0, false
		}
		column += utf16.RuneLen(r)
	}
	return len(text), column == pos.Character
}

func run() error {
	var request struct {
		Version   int        `json:"version"`
		Text      string     `json:"text"`
		Selection *selection `json:"selection"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		return err
	}
	if request.Version != 1 {
		return fmt.Errorf("unsupported protocol version")
	}
	out := json.NewEncoder(os.Stdout)
	if request.Selection == nil {
		return out.Encode(map[string]string{"message": "Select text first"})
	}
	start, ok := byteOffset(request.Text, request.Selection.Start)
	end, valid := byteOffset(request.Text, request.Selection.End)
	if !ok || !valid || end < start {
		return fmt.Errorf("invalid selection")
	}
	return out.Encode(map[string]any{
		"message": "Uppercased selection",
		"edits":   []map[string]any{{"range": request.Selection, "newText": strings.ToUpper(request.Text[start:end])}},
	})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
