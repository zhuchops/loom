package editor

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Remote sessions. `loom ssh host` is ssh with the desktop app's socket
// forwarded to the server. There, `loom-desktop .` finds that socket, runs
// the editor on the server (files, git, search and language servers are all
// there) and asks the desktop app here for a window, which draws the frames
// the server sends and sends keys, clicks and the clipboard back.
//
// The connection carries one JSON object per line. From the window: a call,
// {"call": "Key", "args": [...]}. From the editor: {"frame": ...},
// {"clipboard": "..."}, {"appearance": true} or {"quit": true}.

// remoteMessage is what the editor sends to the window.
type remoteMessage struct {
	Frame      any    `json:"frame,omitempty"`
	Clipboard  string `json:"clipboard,omitempty"`
	Appearance *bool  `json:"appearance,omitempty"`
	Quit       bool   `json:"quit,omitempty"`
}

// remoteCall is what the window sends to the editor.
type remoteCall struct {
	Call string            `json:"call"`
	Args []json.RawMessage `json:"args"`
}

// remotePeer is the window's end: page calls go out on it.
type remotePeer struct {
	mu  sync.Mutex
	enc *json.Encoder
}

func newRemotePeer(conn net.Conn) *remotePeer { return &remotePeer{enc: json.NewEncoder(conn)} }

func (p *remotePeer) forward(call string, args ...any) {
	raw := make([]json.RawMessage, len(args))
	for i, a := range args {
		raw[i], _ = json.Marshal(a)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	_ = p.enc.Encode(remoteCall{Call: call, Args: raw})
}

// remoteHost is the editor's end: frames go to the window.
type remoteHost struct {
	mu   sync.Mutex
	enc  *json.Encoder
	conn net.Conn
}

func (h *remoteHost) send(m remoteMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	_ = h.enc.Encode(m)
}

func (h *remoteHost) emit(frame any) { h.send(remoteMessage{Frame: frame}) }

func (h *remoteHost) quit() {
	h.send(remoteMessage{Quit: true})
	_ = h.conn.Close()
}

func (h *remoteHost) appearance(dark bool) { h.send(remoteMessage{Appearance: &dark}) }

// pick has no panel to show: the window's file panels would browse the
// wrong computer.
func (h *remoteHost) pick(pickKind, string, func(string)) bool { return false }

// serveRemote runs the editor for a window on the other end of conn until
// the window closes or the connection drops.
func serveRemote(conn net.Conn, e *editor, explicit bool, host string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := &remoteHost{enc: json.NewEncoder(conn), conn: conn}
	b := newBridge(e, ctx, explicit, h)
	defer b.ticker.Stop()
	b.remoteHost = host

	// The clipboard is the one on the computer with the window.
	var clip string
	clipboardCopy = func(text string) bool {
		h.send(remoteMessage{Clipboard: text})
		return true
	}
	clipboardPaste = func() (string, bool) { return clip, clip != "" }

	done := make(chan struct{})
	go func() {
		b.loop()
		close(done)
	}()
	dec := json.NewDecoder(bufio.NewReader(conn))
	for {
		var c remoteCall
		if err := dec.Decode(&c); err != nil {
			break
		}
		if c.Call == "Clipboard" {
			var text string
			if len(c.Args) == 1 && json.Unmarshal(c.Args[0], &text) == nil {
				b.call(func() { clip = text })
			}
			continue
		}
		b.dispatch(c)
	}
	// The window went away, or the connection did: stop here too.
	select {
	case <-done:
	default:
		b.call(func() { b.m.editor.quit = true })
		<-done
	}
	e.hub.finish()
	return nil
}

// dispatch runs a call from the window as the page would have made it.
func (b *Bridge) dispatch(c remoteCall) {
	arg := func(i int, v any) bool { return i < len(c.Args) && json.Unmarshal(c.Args[i], v) == nil }
	var (
		x, y, i, j  int
		s, t        string
		held, shift bool
		key         KeyEvent
	)
	switch c.Call {
	case "Start":
		if arg(0, &x) && arg(1, &y) {
			b.Start(x, y)
		}
	case "Resize":
		if arg(0, &x) && arg(1, &y) {
			b.Resize(x, y)
		}
	case "Redraw":
		b.Redraw()
	case "Key":
		if arg(0, &key) {
			b.Key(key)
		}
	case "Text":
		if arg(0, &s) {
			b.Text(s)
		}
	case "Mouse":
		if arg(0, &s) && arg(1, &x) && arg(2, &y) && arg(3, &i) && arg(4, &held) && arg(5, &shift) {
			b.Mouse(s, x, y, i, held, shift)
		}
	case "Wheel":
		if arg(0, &x) && arg(1, &y) && arg(2, &i) && arg(3, &j) {
			b.Wheel(x, y, i, j)
		}
	case "Command":
		if arg(0, &s) {
			b.Command(s)
		}
	case "Act":
		if arg(0, &s) && arg(1, &i) && arg(2, &t) {
			b.Act(s, i, t)
		}
	case "Answer":
		if arg(0, &i) {
			b.Answer(i)
		}
	case "Close":
		// The window's close button: quit, or ask about unsaved files first.
		b.call(func() {
			if b.unsaved() {
				b.requestQuit()
			} else {
				b.m.editor.hub.quitAll(false)
			}
		})
	}
}

// remoteSocket finds the desktop app's socket that loom ssh forwarded to this
// machine, or returns "" when there is none.
func remoteSocket() string {
	var candidates []string
	if s := os.Getenv("LOOM_REMOTE_SOCKET"); s != "" {
		candidates = append(candidates, s)
	}
	inSSH := os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != ""
	if inSSH {
		if s := os.Getenv("LC_LOOM_SOCKET"); s != "" {
			candidates = append(candidates, s)
		}
		// Without LC_ variables passed through, look for any forwarded socket
		// of this user, the newest first.
		found, _ := filepath.Glob(filepath.Join(os.TempDir(), "loom-fwd-*.sock"))
		if os.TempDir() != "/tmp" {
			more, _ := filepath.Glob("/tmp/loom-fwd-*.sock")
			found = append(found, more...)
		}
		sort.Slice(found, func(a, b int) bool { return modTime(found[a]).After(modTime(found[b])) })
		for _, f := range found {
			if owned(f) {
				candidates = append(candidates, f)
			}
		}
	}
	for _, c := range candidates {
		if conn, err := net.DialTimeout("unix", c, 500*time.Millisecond); err == nil {
			conn.Close()
			return c
		}
	}
	return ""
}

func modTime(path string) time.Time {
	info, err := os.Lstat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func owned(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}

// runRemote is loom-desktop on a server: it runs the editor here for a
// window on the computer the SSH session came from.
func runRemote(sock string) error {
	e, explicit, done, err := prepare(true)
	if err != nil || done {
		return err
	}
	conn, err := net.DialTimeout("unix", sock, 3*time.Second)
	if err != nil {
		return fmt.Errorf("the Loom on your computer is not reachable: %w", err)
	}
	host, _ := os.Hostname()
	wd, _ := os.Getwd()
	if err := json.NewEncoder(conn).Encode(openRequest{Cwd: wd, Remote: &remoteHello{Host: host, Root: e.root}}); err != nil {
		return err
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	reply, err := readLine(conn)
	if err != nil || reply != "ok" {
		return errors.New("the Loom on your computer did not open a window; is it a version from before remote sessions?")
	}
	_ = conn.SetReadDeadline(time.Time{})
	return serveRemote(conn, e, explicit, host)
}

// readLine reads one line byte by byte, so nothing after it is taken from
// the connection.
func readLine(conn net.Conn) (string, error) {
	var line []byte
	buf := make([]byte, 1)
	for {
		if _, err := conn.Read(buf); err != nil {
			return "", err
		}
		if buf[0] == '\n' {
			return strings.TrimSpace(string(line)), nil
		}
		line = append(line, buf[0])
	}
}

// remoteUnavailable is what loom-desktop says on a server it can't serve.
const remoteUnavailable = `loom-desktop: no Loom window to open in.
On a server, connect with "loom ssh host" from the computer that runs the
Loom app (it forwards the app's socket), with the app open; or add the
forward to ~/.ssh/config, as "loom ssh-config" prints.`

// sshCommand is loom ssh: ssh with the desktop app's socket forwarded to a
// fresh path on the server, which LC_LOOM_SOCKET names there.
func sshCommand(args []string) error {
	local := ipcSocket()
	if local == "" {
		return errors.New("no state folder for the desktop app's socket")
	}
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		return err
	}
	if conn, err := net.DialTimeout("unix", local, 300*time.Millisecond); err == nil {
		conn.Close()
	} else {
		fmt.Fprintln(os.Stderr, "loom: the Loom app is not open here; open it before running loom-desktop on the server.")
	}
	id := make([]byte, 6)
	_, _ = rand.Read(id)
	remote := "/tmp/loom-fwd-" + hex.EncodeToString(id) + ".sock"
	argv := append([]string{"ssh", "-o", "StreamLocalBindUnlink=yes", "-o", "SendEnv=LC_LOOM_SOCKET", "-R", remote + ":" + local}, args...)
	return syscall.Exec(ssh, argv, append(os.Environ(), "LC_LOOM_SOCKET="+remote))
}

// sshConfig prints what to add to ~/.ssh/config so plain ssh forwards the
// socket too.
func sshConfig() {
	user := os.Getenv("USER")
	if user == "" {
		user = "me"
	}
	fmt.Printf(`# Lets loom-desktop on these servers open files in the Loom app here.
# Put it in ~/.ssh/config and change the Host line to the servers you use.
Host my-server
    RemoteForward /tmp/loom-fwd-%s.sock %s
    StreamLocalBindUnlink yes
`, user, ipcSocket())
}
