package jsonrpc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// With LOOM_RPC_ECHO set, the test binary acts as a server that answers every
// request with {"echo": method} and announces itself with a notification.
func TestMain(m *testing.M) {
	if os.Getenv("LOOM_RPC_ECHO") != "" {
		serve()
		return
	}
	os.Exit(m.Run())
}

func serve() {
	send := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Printf("Content-Length: %d\r\n\r\n%s", len(b), b)
	}
	send(map[string]any{"jsonrpc": "2.0", "method": "hello", "params": map[string]any{}})
	r := bufio.NewReader(os.Stdin)
	for {
		var n int
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if strings.TrimSpace(line) == "" {
				break
			}
			fmt.Sscanf(line, "Content-Length: %d", &n)
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		var msg Message
		json.Unmarshal(body, &msg)
		if len(msg.ID) > 0 {
			send(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": map[string]string{"echo": msg.Method}})
		}
	}
}

func TestRequestAndNotification(t *testing.T) {
	t.Setenv("LOOM_RPC_ECHO", "1")
	hello := make(chan string, 1)
	c, err := Start(context.Background(), []string{os.Args[0]}, ".", func(m Message) { hello <- m.Method })
	if err != nil {
		t.Fatal(err)
	}
	defer c.Kill()
	select {
	case got := <-hello:
		if got != "hello" {
			t.Fatalf("notification %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no notification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var out struct{ Echo string }
	if err := c.Request(ctx, "textDocument/hover", nil, &out); err != nil || out.Echo != "textDocument/hover" {
		t.Fatalf("request: %v %+v", err, out)
	}
	c.Kill()
	select {
	case <-c.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Done never closed")
	}
}
