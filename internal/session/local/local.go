package local

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"

	"github.com/leofds/conduit/internal/session"
)

// Config holds session-level parameters for local sessions.
type Config struct {
	WorkingDir  string
	Term        string
	Command     string
	IdleTimeout time.Duration
	Env         map[string]string
}

type Runner struct {
	cfg  Config
	rows uint16
	cols uint16
}

func New(localCfg Config, cols, rows uint16) *Runner {
	return &Runner{
		cfg:  localCfg,
		rows: rows,
		cols: cols,
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
	parts := strings.Fields(r.cfg.Command)
	cmd = exec.Command(parts[0], parts[1:]...)
	cmd.Env = os.Environ()
	for k, v := range r.cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Env = append(cmd.Env, "TERM="+r.cfg.Term)
	if r.cfg.WorkingDir != "" {
		cmd.Dir = r.cfg.WorkingDir
	}
	ptmx, err := pty.Start(cmd)
	if err != nil {
		notify("PTY start failed: %v", err)
		return
	}
	pty.Setsize(ptmx, &pty.Winsize{Rows: r.rows, Cols: r.cols}) //nolint:errcheck
	defer func() { _ = ptmx.Close() }()

	// waitCh is closed when the process exits, which lets the graceful
	// shutdown goroutine know it can stop waiting for the process.
	waitCh := make(chan struct{})

	// Graceful shutdown goroutine: when ctx is cancelled, send SIGTERM first,
	// wait briefly, then escalate to SIGKILL. This gives the shell and its
	// children a chance to clean up (e.g. .bash_logout, temp files, history).
	// If the process has already exited (normal exit via "exit"), waitCh will
	// be closed and the select below will return immediately.
	go func() {
		<-ctx.Done()
		if cmd.Process == nil {
			return
		}
		log.Printf("sending SIGTERM to local session process %d", cmd.Process.Pid)
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			// Process may have already exited — that's fine.
			return
		}
		// Wait briefly for the process to exit after SIGTERM.
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			log.Printf("grace period expired, sending SIGKILL to %d", cmd.Process.Pid)
			_ = cmd.Process.Kill()
		case <-waitCh:
			// Process exited gracefully during the grace period.
		}
	}()

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

	// Process-exit watcher: close waitCh so the graceful shutdown goroutine
	// knows the process has already exited (avoids unnecessary SIGKILL).
	go func() {
		defer close(waitCh)
		if err := cmd.Wait(); err != nil {
			notify("session ended: %v", err)
		} else {
			notify("session ended")
		}
	}()

	// Idle timer
	var idleTimer *time.Timer
	if r.cfg.IdleTimeout > 0 {
		idleTimer = time.NewTimer(r.cfg.IdleTimeout)
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
			idleTimer.Reset(r.cfg.IdleTimeout)
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
