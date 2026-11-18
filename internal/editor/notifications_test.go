package editor

import (
	"slices"
	"strings"
	"testing"
)

func TestNotificationHistory(t *testing.T) {
	e := testEditor(t, "vim", "")
	e.ex(":messages")
	if e.menu == nil || len(e.menu.options) != 0 {
		t.Fatal("could not open empty history")
	}
	press(e, "esc")
	e.notify("earlier")
	e.openDirectory(e.root)
	e.directory.doc.notify("directory message")
	e.closeDirectory(true)
	full := "newest\n\n" + strings.Repeat("details 界\n", 20) + "last line"
	e.notify(full)
	press(e, "space N")
	if e.menu == nil || len(e.menu.options) != 3 || e.menu.options[1].label != "directory message" {
		t.Fatal("history lost a notification")
	}
	press(e, "l a s t space l i n e")
	history := e.menu
	press(e, "enter")
	if e.notificationDetail == nil || e.notificationDetail.text != full {
		t.Fatal("history did not open the full message")
	}
	screen := newTerminalFrame(35, 10)
	v := renderer{screen: screen}
	v.draw(e)
	v.mouse(e, pointerEvent{x: 10, y: 4, button: wheelDown})
	if e.notificationDetail.scroll == 0 {
		t.Fatal("notification detail did not scroll")
	}
	press(e, "end")
	v.draw(e)
	var row strings.Builder
	for x := 1; x < 34; x++ {
		s, _, _ := screen.Get(x, 6)
		row.WriteString(s)
	}
	if !strings.Contains(row.String(), "last line") {
		t.Fatalf("could not read the last line: %q", row.String())
	}
	press(e, "esc")
	if e.menu != history || e.menu.input != "last line" || e.notificationDetail != nil {
		t.Fatal("returning to history lost the filter")
	}
	press(e, "esc")
	e.panel = &sourceControl{}
	v.mouse(e, pointerEvent{x: 2, y: 9, leftDown: true})
	v.mouse(e, pointerEvent{x: 2, y: 9})
	v.draw(e)
	v.mouse(e, pointerEvent{x: v.menuBox.x, y: v.menuBox.y, leftDown: true})
	if e.notificationDetail == nil || e.notificationDetail.text != full {
		t.Fatal("could not open history by mouse over source control")
	}
}

func TestNotificationLimitAndWrapping(t *testing.T) {
	e := testEditor(t, "vim", "")
	e.notify("discarded")
	for range notificationLimit {
		e.notify("kept")
	}
	if len(e.notifications) != notificationLimit || e.notifications[0].text != "kept" {
		t.Fatal("history did not discard the oldest entry")
	}
	if rows := wrapNotification("a界界\r\n\nlast", 3); !slices.Equal(rows, []string{"a界", "界", "", "las", "t"}) {
		t.Fatalf("unexpected wrapped lines: %q", rows)
	}
}
