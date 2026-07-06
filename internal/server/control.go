package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/leofds/conduit/internal/session"
)

type ControlServer struct {
	path           string
	listener       net.Listener
	sessionManager *session.Manager
}

func NewControlServer(path string, sessionManager *session.Manager) (*ControlServer, error) {
	if sessionManager == nil {
		return nil, fmt.Errorf("session manager is required")
	}
	return &ControlServer{
		path:           path,
		sessionManager: sessionManager,
	}, nil
}

// StartUnixSocket listens on a Unix socket for control commands.
func (c *ControlServer) StartUnixSocket() error {

	// Remove stale socket file.
	if err := os.Remove(c.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	listener, err := net.Listen("unix", c.path)
	if err != nil {
		return fmt.Errorf("listen unix socket: %w", err)
	}
	c.listener = listener

	// Restrict permissions to owner only.
	if err := os.Chmod(c.path, 0600); err != nil {
		return fmt.Errorf("chmod socket: %w", err)
	}

	log.Printf("Control socket created at %s", c.path)

	go func() {
		log.Printf("Control socket listening on %s", c.path)
		for {
			conn, err := listener.Accept()
			if err != nil {
				if !strings.Contains(err.Error(), "use of closed network connection") {
					log.Printf("Control socket accept error: %v", err)
				}
				return
			}
			go c.handleControlConn(conn)
		}
	}()

	return nil
}

// StopUnixSocket closes the control socket listener and removes the socket file.
func (c *ControlServer) StopUnixSocket() error {
	if c.listener != nil {
		if err := c.listener.Close(); err != nil {
			return fmt.Errorf("close control socket: %w", err)
		}
	}
	if c.path != "" {
		if err := os.Remove(c.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove control socket: %w", err)
		}
	}
	return nil
}

func (c *ControlServer) handleControlConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	scanner := bufio.NewScanner(conn)
	// Only process one command per connection, then close.
	if !scanner.Scan() {
		return
	}
	line := strings.TrimSpace(scanner.Text())
	if line == "" {
		return
	}

	log.Printf("Control socket received command: %s", line)

	switch {
	case line == "list":
		sessions := c.sessionManager.List()
		data, _ := json.MarshalIndent(sessions, "", "  ")
		_, _ = fmt.Fprintf(conn, "%s\n", string(data))

	case line == "close-all":
		count := c.sessionManager.CancelAll()
		_, _ =fmt.Fprintf(conn, "cancelled %d sessions\n", count)

	case strings.HasPrefix(line, "close "):
		id := strings.TrimSpace(strings.TrimPrefix(line, "close "))
		if id == "" {
			_, _ = fmt.Fprintf(conn, "usage: close <session-id>\n")
			return
		}
		if c.sessionManager.Cancel(id) {
			_, _ = fmt.Fprintf(conn, "cancelled session %s\n", id)
		} else {
			_, _ = fmt.Fprintf(conn, "session %s not found\n", id)
		}

	default:
		_, _ = fmt.Fprintf(conn, "unknown command: %s\n", line)
	}
}
