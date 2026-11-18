// Package jsonrpc is a small JSON-RPC 2.0 client for language servers.
package jsonrpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Message is a JSON-RPC message of any kind.
type Message struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Client is one running JSON-RPC peer.
type Client struct {
	onNotification func(Message)
	process        *exec.Cmd
	in             io.WriteCloser
	sendMu, mu     sync.Mutex
	next           int
	waiters        map[string]chan Message
	done           chan struct{}
}

// Start runs argv in root and speaks JSON-RPC with Content-Length framing over
// its stdin and stdout. onNotification gets every message without an id.
func Start(ctx context.Context, argv []string, root string, onNotification func(Message)) (*Client, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = root
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		in.Close()
		out.Close()
		return nil, err
	}
	c := &Client{onNotification: onNotification, process: cmd, in: in, waiters: make(map[string]chan Message), done: make(chan struct{})}
	go func() {
		c.read(out)
		in.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		close(c.done)
	}()
	return c, nil
}

func (c *Client) write(message map[string]any) error {
	message["jsonrpc"] = "2.0"
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	_, err = fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
	return err
}

func (c *Client) read(reader io.Reader) {
	r := bufio.NewReader(reader)
	for {
		length := -1
		for {
			line, err := r.ReadSlice('\n')
			if err != nil {
				return
			}
			header := strings.TrimSpace(string(line))
			if header == "" {
				break
			}
			key, value, ok := strings.Cut(header, ":")
			if ok && strings.EqualFold(key, "Content-Length") {
				length, err = strconv.Atoi(strings.TrimSpace(value))
				if err != nil {
					return
				}
			}
		}
		if length < 0 || length > 32<<20 {
			return
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		var msg Message
		if json.Unmarshal(body, &msg) != nil {
			continue
		}
		id := string(msg.ID)
		if id == "" || id == "null" {
			if c.onNotification != nil {
				c.onNotification(msg)
			}
			continue
		}
		if msg.Method != "" {
			reply := map[string]any{"id": msg.ID}
			switch msg.Method {
			case "workspace/configuration":
				var p struct{ Items []json.RawMessage }
				_ = json.Unmarshal(msg.Params, &p)
				reply["result"] = make([]any, len(p.Items))
			default:
				reply["error"] = map[string]any{"code": -32601, "message": "Method not supported"}
			}
			_ = c.write(reply)
			continue
		}
		c.mu.Lock()
		ch := c.waiters[id]
		c.mu.Unlock()
		if ch != nil {
			select {
			case ch <- msg:
			default:
			}
		}
	}
}

// Notify sends a notification.
func (c *Client) Notify(method string, params any) error {
	return c.write(map[string]any{"method": method, "params": params})
}

// Request sends a call and decodes its result into dest, which may be nil.
func (c *Client) Request(ctx context.Context, method string, params any, dest any) error {
	c.mu.Lock()
	c.next++
	id := c.next
	key := strconv.Itoa(id)
	ch := make(chan Message, 1)
	c.waiters[key] = ch
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.waiters, key); c.mu.Unlock() }()
	if err := c.write(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return err
	}
	select {
	case msg := <-ch:
		if msg.Error != nil {
			return fmt.Errorf("%s", msg.Error.Message)
		}
		if dest != nil {
			return json.Unmarshal(msg.Result, dest)
		}
		return nil
	case <-c.done:
		return errors.New("language server exited")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Done is closed once the process has exited.
func (c *Client) Done() <-chan struct{} { return c.done }

// Kill stops the process.
func (c *Client) Kill() { _ = c.process.Process.Kill() }
