package session

import (
	"context"
	"errors"
	"sync"

	"github.com/gorilla/websocket"
)

// ErrInterrupted is returned by ReadLine when the user presses Ctrl+C (\x03).
var ErrInterrupted = errors.New("interrupted")

// Runner is the interface implemented by each terminal backend.
type Runner interface {

	// Starts a new session (SSH, local, etc.) and pipes I/O with the WebSocket until the session ends.
	Run(ctx context.Context, conn *websocket.Conn)
}

// ResizeMsg is the JSON resize message sent by xterm.js onResize.
type ResizeMsg struct {
	Type string `json:"type"` // "resize"
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

// Writer is a mutex-protected io.Writer that forwards bytes to a WebSocket as binary frames.
type Writer struct {
	Conn *websocket.Conn
	mu   sync.Mutex
}

func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.Conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// ReadLine reads bytes from the WebSocket until \r or \n and returns the line without the terminator.
// Returns ErrInterrupted if the user sends Ctrl+C (\x03).
// Text messages (e.g. resize JSON) are silently skipped.
func ReadLine(conn *websocket.Conn) (string, error) {
	return readLine(conn, false)
}

// ReadLineEcho is like ReadLine but echoes each character back to the terminal.
// Backspace (\x7f / \x08) erases the last character. Use this before a PTY is active.
func ReadLineEcho(conn *websocket.Conn) (string, error) {
	return readLine(conn, true)
}

func readLine(conn *websocket.Conn, echo bool) (string, error) {
	var buf []byte
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return "", err
		}
		if msgType == websocket.TextMessage {
			continue // skip resize/control messages
		}
		for _, b := range data {
			if b == '\x03' {
				return "", ErrInterrupted
			}
			if b == '\r' || b == '\n' {
				return string(buf), nil
			}
			if b == '\x7f' || b == '\x08' { // backspace
				if len(buf) > 0 {
					buf = buf[:len(buf)-1]
					if echo {
						conn.WriteMessage(websocket.BinaryMessage, []byte("\x08 \x08")) //nolint:errcheck
					}
				}
				continue
			}
			buf = append(buf, b)
			if echo {
				conn.WriteMessage(websocket.BinaryMessage, []byte{b}) //nolint:errcheck
			}
		}
	}
}
