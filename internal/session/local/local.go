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

// Config holds session-level parameters for local sessions.
type Config struct {
	WorkingDir  string
	IdleTimeout time.Duration
	Env         map[string]string
}

type Runner struct {
	command     string
	workingDir  string
	term        string
	cols        uint16
	rows        uint16
	idleTimeout time.Duration
	env         map[string]string
}

func New(cfg resolver.LocalConfig, localCfg Config, cols, rows uint16) *Runner {
	term := cfg.Term
	if term == "" {
		term = "xterm-256color"
	}
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}
	return &Runner{
		command:     cfg.Command,
		workingDir:  localCfg.WorkingDir,
		term:        term,
		cols:        cols,
		rows:        rows,
		idleTimeout: localCfg.IdleTimeout,
		env:         localCfg.Env,
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

	var cmd *exec.Cmd
	if r.command != "" {
		cmd = exec.CommandContext(ctx, r.command)
	} else if os.Getuid() == 0 {
		cmd = exec.CommandContext(ctx, "/bin/login")
	} else {
		cmd = exec.CommandContext(ctx, "sudo", "-n", "/bin/login")
	}
	cmd.Env = os.Environ()
	for k, v := range r.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Env = append(cmd.Env, "TERM="+r.term)
	if r.workingDir != "" {
		cmd.Dir = r.workingDir
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		notify("PTY start failed: %v", err)
		return
	}
	pty.Setsize(ptmx, &pty.Winsize{Rows: r.rows, Cols: r.cols}) //nolint:errcheck
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
	var idleTimer *time.Timer
	if r.idleTimeout > 0 {
		idleTimer = time.NewTimer(r.idleTimeout)
		defer idleTimer.Stop()
		go func() {
			select {
			case <-idleTimer.C:
				notify("session closed due to inactivity")
			case <-ctx.Done():
			}
		}()
	}

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

		if idleTimer != nil {
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(r.idleTimeout)
		}

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
