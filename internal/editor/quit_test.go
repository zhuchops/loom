//go:build desktop

package editor

import (
	"os"
	"testing"
	"time"
)

// closingHost is a window that closes when its editor quits.
type closingHost struct {
	b      *Bridge
	closed chan struct{}
}

func (h closingHost) emit(any)                                 {}
func (h closingHost) quit()                                    { close(h.closed) }
func (h closingHost) pick(pickKind, string, func(string)) bool { return false }
func (h closingHost) appearance(bool)                          {}

func TestQuittingWithUnsavedChanges(t *testing.T) {
	for _, choice := range []struct {
		name  string
		index int
		saved string
	}{{"Save All", 0, "package a\nx"}, {"Don't Save", 1, "package a\n"}} {
		t.Run(choice.name, func(t *testing.T) {
			b, e := testBridge(t, "")
			path := fixture(t, e.root, "a.go", "package a\n")
			e.cfg.FormatOnSave = false
			e.open(path)
			e.b.text = append(e.b.text, 'x')
			e.b.indexLines()
			host := closingHost{b, make(chan struct{})}
			b.host = host
			go b.loop()

			// The window's close button: the dialog asks first.
			asked := make(chan bool)
			b.call(func() {
				b.requestQuit()
				asked <- b.dialog != nil
			})
			if !<-asked {
				t.Fatal("closed with unsaved changes")
			}
			b.Answer(choice.index)
			select {
			case <-host.closed:
			case <-time.After(3 * time.Second):
				t.Fatal("the window never closed")
			}
			if data, _ := os.ReadFile(path); string(data) != choice.saved {
				t.Fatalf("file holds %q, want %q", data, choice.saved)
			}
		})
	}
}
