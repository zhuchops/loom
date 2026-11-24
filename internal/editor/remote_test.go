package editor

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"
)

// TestRemoteSession drives an editor the way a window on another computer
// does: over a connection, with calls one way and frames the other.
func TestRemoteSession(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	copyWas, pasteWas := clipboardCopy, clipboardPaste
	t.Cleanup(func() { clipboardCopy, clipboardPaste = copyWas, pasteWas })

	e := testEditor(t, "vim", "")
	server, window := net.Pipe()
	served := make(chan error, 1)
	go func() { served <- serveRemote(server, e, false, "build-box") }()

	messages := make(chan map[string]json.RawMessage, 64)
	go func() {
		dec := json.NewDecoder(window)
		for {
			var m map[string]json.RawMessage
			if dec.Decode(&m) != nil {
				close(messages)
				return
			}
			messages <- m
		}
	}()
	enc := json.NewEncoder(window)
	call := func(name string, args ...any) {
		raw := make([]json.RawMessage, len(args))
		for i, a := range args {
			raw[i], _ = json.Marshal(a)
		}
		if err := enc.Encode(remoteCall{Call: name, Args: raw}); err != nil {
			t.Fatal(err)
		}
	}
	// await reads messages until one satisfies ok.
	await := func(what string, ok func(map[string]json.RawMessage) bool) map[string]json.RawMessage {
		t.Helper()
		timeout := time.After(10 * time.Second)
		for {
			select {
			case m, open := <-messages:
				if !open {
					t.Fatalf("connection closed while waiting for %s", what)
				}
				if ok(m) {
					return m
				}
			case <-timeout:
				t.Fatalf("no %s", what)
			}
		}
	}
	has := func(key, text string) func(map[string]json.RawMessage) bool {
		return func(m map[string]json.RawMessage) bool { return strings.Contains(string(m[key]), text) }
	}

	call("Start", 80, 24)
	await("a frame naming the host", has("frame", `"host":"build-box"`))

	call("Command", "new-file") // starts in Insert mode
	call("Text", "hello remote")
	await("the typed text", has("frame", "hello remote"))

	call("Key", KeyEvent{Key: "Escape"})
	call("Text", "y")
	call("Text", "y")
	await("the yanked line on this computer's clipboard", has("clipboard", "hello remote"))

	// Pasting takes what the window's computer has on its clipboard.
	call("Clipboard", "from the laptop")
	call("Command", "paste")
	await("the pasted text", has("frame", "from the laptop"))

	// Closing asks about the unsaved buffer; Don't Save quits.
	call("Close")
	await("the question", has("frame", "Save your changes"))
	call("Answer", 1)
	await("the end of the session", func(m map[string]json.RawMessage) bool { return string(m["quit"]) == "true" })
	select {
	case err := <-served:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the remote editor did not stop")
	}
}

func TestRemoteSocketNeedsAnSSHSession(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("LOOM_REMOTE_SOCKET", "")
	if s := remoteSocket(); s != "" {
		t.Fatalf("found %q outside SSH", s)
	}
}
