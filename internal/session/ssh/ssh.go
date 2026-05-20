package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	gossh "golang.org/x/crypto/ssh"

	"github.com/leofds/conduit/internal/resolver"
	"github.com/leofds/conduit/internal/session"
)

const (
	idleTimeout       = 10 * time.Minute
	keepaliveInterval = 30 * time.Second
	dialTimeout       = 10 * time.Second
)

type Runner struct {
	addr string // host:port
	user string
	term string
}

func New(cfg resolver.SSHConfig) *Runner {
	term := cfg.Term
	if term == "" {
		term = "xterm-256color"
	}
	return &Runner{
		addr: fmt.Sprintf("%s:%s", cfg.Address, cfg.Port),
		user: cfg.Username,
		term: term,
	}
}

func (r *Runner) Run(parentCtx context.Context, wsConn *websocket.Conn) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	notify := func(format string, args ...any) {
		msg := fmt.Sprintf("\r\n["+format+"]\r\n", args...)
		log.Printf("%s", msg)
		wsConn.WriteMessage(websocket.BinaryMessage, []byte(msg)) //nolint:errcheck
		cancel()
	}

	readLine := func() (string, error) {
		return session.ReadLine(wsConn)
	}

	cfg := &gossh.ClientConfig{
		User: r.user,
		Auth: []gossh.AuthMethod{},
		BannerCallback: func(message string) error {
			normalized := strings.ReplaceAll(strings.ReplaceAll(message, "\r\n", "\n"), "\n", "\r\n")
			wsConn.WriteMessage(websocket.BinaryMessage, []byte(normalized)) //nolint:errcheck
			return nil
		},
		// TODO: replace with stored host key fingerprint check
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         dialTimeout,
	}

	kbdInt := gossh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
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

	passwordPrompt := func() (string, error) {
		wsConn.WriteMessage(websocket.BinaryMessage, []byte("Password: ")) //nolint:errcheck
		pass, err := readLine()
		if err != nil {
			return "", err
		}
		wsConn.WriteMessage(websocket.BinaryMessage, []byte("\r\n")) //nolint:errcheck
		return pass, nil
	}

	cfg.Auth = []gossh.AuthMethod{
		gossh.RetryableAuthMethod(kbdInt, 3),
		gossh.RetryableAuthMethod(gossh.PasswordCallback(passwordPrompt), 3),
	}

	client, err := gossh.Dial("tcp", r.addr, cfg)
	if err != nil {
		notify("SSH connection failed: %v", err)
		return
	}
	defer func() { _ = client.Close() }()

	sesh, err := client.NewSession()
	if err != nil {
		notify("SSH session failed: %v", err)
		return
	}
	defer func() { _ = sesh.Close() }()

	modes := gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}
	if err := sesh.RequestPty(r.term, 24, 80, modes); err != nil {
		notify("PTY request failed: %v", err)
		return
	}

	writer := &session.Writer{Conn: wsConn}
	sesh.Stdout = writer
	sesh.Stderr = writer

	stdinPipe, err := sesh.StdinPipe()
	if err != nil {
		notify("stdin pipe failed: %v", err)
		return
	}
	defer func() { _ = stdinPipe.Close() }()

	if err := sesh.Shell(); err != nil {
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
				if _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
					notify("SSH keepalive failed — connection lost")
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Session-exit watcher
	go func() {
		if err := sesh.Wait(); err != nil {
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

	// Main loop: WebSocket -> SSH stdin
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
			var msg session.ResizeMsg
			if err := json.Unmarshal(data, &msg); err == nil && msg.Type == "resize" {
				sesh.WindowChange(int(msg.Rows), int(msg.Cols)) //nolint:errcheck
				continue
			}
		}

		if _, err := stdinPipe.Write(data); err != nil {
			return
		}
	}
}
