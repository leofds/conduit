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
			buf = append(buf, b)
		}
	}
}
