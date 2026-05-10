package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

const (
	idleTimeout       = 10 * time.Minute
	keepaliveInterval = 30 * time.Second
	sshDialTimeout    = 10 * time.Second
)

// wsResizeMsg is the JSON message sent by xterm.js when the terminal is resized.
type wsResizeMsg struct {
	Type string `json:"type"` // "resize"
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO: validate Origin against allowed hosts
		return true
	},
}

// wsWriter implements io.Writer forwarding SSH output to the WebSocket.
type wsWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *wsWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// wsHandler upgrades the HTTP connection to WebSocket and starts an SSH session.
//
// Query parameters:
//
//	host - SSH host (required)
//	port - SSH port (default: 22)
//	user - SSH username (required)
func wsHandler(c *gin.Context) {
	host := c.Query("host")
	port := c.DefaultQuery("port", "22")
	user := c.Query("user")

	if host == "" || user == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host and user are required"})
		return
	}

	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	defer wsConn.Close() //nolint:errcheck

	addr := fmt.Sprintf("%s:%s", host, port)
	log.Printf("session open  user=%s addr=%s", user, addr)

	runSSHSession(c.Request.Context(), wsConn, addr, user)

	log.Printf("session close user=%s addr=%s", user, addr)
}

func runSSHSession(parentCtx context.Context, wsConn *websocket.Conn, addr, user string) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	notify := func(format string, args ...any) {
		msg := fmt.Sprintf("\r\n["+format+"]\r\n", args...)
		wsConn.WriteMessage(websocket.TextMessage, []byte(msg)) //nolint:errcheck
		cancel()
	}

	readLine := func() (string, error) {
		var buf []byte
		for {
			_, data, err := wsConn.ReadMessage()
			if err != nil {
				return "", err
			}
			for _, b := range data {
				if b == '\r' || b == '\n' {
					return string(buf), nil
				}
				buf = append(buf, b)
			}
		}
	}

	sshCfg := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{},
		BannerCallback: func(message string) error {
			wsConn.WriteMessage(websocket.BinaryMessage, []byte(message+"\r\n")) //nolint:errcheck
			return nil
		},
		// TODO: replace with stored host key fingerprint check
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         sshDialTimeout,
	}

	// Repeat keyboard-interactive 3 times
	kbdInt := ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, len(questions))
		if name != "" {
			wsConn.WriteMessage(websocket.BinaryMessage, []byte(name+"\r\n")) //nolint:errcheck
		}
		if instruction != "" {
			wsConn.WriteMessage(websocket.BinaryMessage, []byte(instruction+"\r\n")) //nolint:errcheck
		}
		for i, q := range questions {
			if q != "" {
				wsConn.WriteMessage(websocket.BinaryMessage, []byte(q)) //nolint:errcheck
			}
			answer, err := readLine()
			if err != nil {
				return nil, err
			}
			wsConn.WriteMessage(websocket.BinaryMessage, []byte("\r\n")) //nolint:errcheck
			answers[i] = answer
		}
		return answers, nil
	})
	sshCfg.Auth = []ssh.AuthMethod{ssh.RetryableAuthMethod(kbdInt, 3)}

	// Dial SSH
	sshClient, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		notify("SSH connection failed: %v", err)
		return
	}
	defer sshClient.Close() //nolint:errcheck

	// Open session
	session, err := sshClient.NewSession()
	if err != nil {
		notify("SSH session failed: %v", err)
		return
	}
	defer session.Close() //nolint:errcheck

	// Request PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		notify("PTY request failed: %v", err)
		return
	}

	// Wire SSH stdout/stderr -> WebSocket
	writer := &wsWriter{conn: wsConn}
	session.Stdout = writer
	session.Stderr = writer

	// SSH stdin pipe
	stdinPipe, err := session.StdinPipe()
	if err != nil {
		notify("stdin pipe failed: %v", err)
		return
	}
	defer stdinPipe.Close() //nolint:errcheck

	// Start shell
	if err := session.Shell(); err != nil {
		notify("shell start failed: %v", err)
		return
	}

	// Keepalive goroutine
	go func() {
		ticker := time.NewTicker(keepaliveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if _, _, err := sshClient.SendRequest("keepalive@openssh.com", true, nil); err != nil {
					notify("SSH keepalive failed — connection lost")
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Session-exit watcher goroutine
	go func() {
		if err := session.Wait(); err != nil {
			notify("session ended: %v", err)
		} else {
			notify("session ended")
		}
	}()

	// Idle timer
	idleTimer := time.NewTimer(idleTimeout)
	defer idleTimer.Stop()
	go func() {
		select {
		case <-idleTimer.C:
			notify("session closed due to inactivity")
		case <-ctx.Done():
		}
	}()

	// Main loop: WebSocket → SSH stdin
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msgType, data, err := wsConn.ReadMessage()
		if err != nil {
			return
		}

		if !idleTimer.Stop() {
			select {
			case <-idleTimer.C:
			default:
			}
		}
		idleTimer.Reset(idleTimeout)

		if msgType == websocket.TextMessage {
			var msg wsResizeMsg
			if err := json.Unmarshal(data, &msg); err == nil && msg.Type == "resize" {
				session.WindowChange(int(msg.Rows), int(msg.Cols)) //nolint:errcheck
				continue
			}
		}

		if _, err := stdinPipe.Write(data); err != nil {
			return
		}
	}
}
