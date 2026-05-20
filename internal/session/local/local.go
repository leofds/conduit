package local

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"

	"github.com/leofds/conduit/internal/resolver"
	"github.com/leofds/conduit/internal/session"
)

const idleTimeout = 10 * time.Minute

type Runner struct {
	command string
	term    string
}

func New(cfg resolver.LocalConfig) *Runner {
	term := cfg.Term
	if term == "" {
		term = "xterm-256color"
	}
	return &Runner{command: cfg.Command, term: term}
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

	var cmd *exec.Cmd
	if r.command != "" {
		cmd = exec.CommandContext(ctx, r.command)
	} else if os.Getuid() == 0 {
		cmd = exec.CommandContext(ctx, "/bin/login")
	} else {
		cmd = exec.CommandContext(ctx, "sudo", "-n", "/bin/login")
	}
	cmd.Env = append(os.Environ(), "TERM="+r.term)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		notify("PTY start failed: %v", err)
		return
	}
	defer func() { _ = ptmx.Close() }()

	// PTY output -> WebSocket
	writer := &session.Writer{Conn: wsConn}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				writer.Write(buf[:n]) //nolint:errcheck
			}
			if readErr != nil {
				return
			}
		}
	}()

	// Process-exit watcher
	go func() {
		if err := cmd.Wait(); err != nil {
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

	// Main loop: WebSocket -> PTY
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
				pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(msg.Rows), Cols: uint16(msg.Cols)}) //nolint:errcheck
				continue
			}
		}

		if _, err := ptmx.Write(data); err != nil {
			return
		}
	}
}
