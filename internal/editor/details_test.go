package editor

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestListsOpenFullDetailsAndReturn(t *testing.T) {
	full := "actual message\n\n" + strings.Repeat("long details 界 ", 100) + "\nFINAL"
	for _, kind := range []string{"hover", "message"} {
		t.Run(kind, func(t *testing.T) {
			e := testEditor(t, "vim", "")
			switch kind {
			case "hover":
				raw, _ := json.Marshal(map[string]string{"contents": full})
				e.showHover(raw, "")
			case "message":
				e.notify(full)
				e.openNotifications()
			}
			if kind == "hover" {
				e.menu.input = "Documentation"
			} else {
				e.menu.input = "actual"
			}
			e.menu.filter()
			parent := e.menu
			filter := parent.input
			v := renderer{screen: newTerminalFrame(52, 14)}
			v.draw(e)
			v.mouse(e, pointerEvent{x: v.menuBox.x, y: v.menuBox.y})
			if !parent.hovering || parent.selectedPreview().message == "" {
				t.Fatal("hover did not expose the message preview")
			}
			press(e, "enter")
			if e.notificationDetail == nil || !strings.Contains(e.notificationDetail.text, full) {
				t.Fatal("Enter did not open the full details")
			}
			v.draw(e)
			press(e, "end")
			v.draw(e)
			if !strings.Contains(v.screen.String(), "FINAL") {
				t.Fatal("full details could not scroll to the final line")
			}
			press(e, "esc")
			if e.notificationDetail != nil || e.menu != parent || parent.input != filter {
				t.Fatal("Escape did not restore the filtered list")
			}
			v.draw(e)
			v.mouse(e, pointerEvent{x: v.menuBox.x, y: v.menuBox.y, leftDown: true})
			if e.notificationDetail == nil {
				t.Fatal("click did not open details")
			}
		})
	}
}
