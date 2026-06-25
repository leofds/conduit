package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/resolver"
)

// stubResolver is a minimal Resolver that always returns "not found".
type stubResolver struct{}

func (stubResolver) Resolve(_ resolver.Request) (resolver.SessionConfig, error) {
	return nil, fmt.Errorf("not found")
}

func newTestServer() *Server {
	serverConfig := config.ServerConfig{Timeouts: config.HTTPServerTimeouts{Read: 10 * time.Second, Write: 0, ReadHeader: 10 * time.Second, Idle: 120 * time.Second}}
	return New(stubResolver{}, serverConfig, nil)
}

func TestHealthEndpoint(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}

func TestIndexRedirectsToDemoWhenEnabled(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/demo", w.Header().Get("Location"))
}

func TestIndexReturns404WhenDemoDisabled(t *testing.T) {
	s := newTestServer()
	s.SetDemo(false)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDemoServesConnectForm(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/demo", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")

	body := w.Body.String()
	assert.Contains(t, body, `id="host"`)
	assert.Contains(t, body, `type="submit"`)
	assert.Contains(t, body, "/terminal")
}

func TestDemoReturns404WhenDisabled(t *testing.T) {
	s := newTestServer()
	s.SetDemo(false)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/demo", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTerminalServesXterm(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/terminal", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")

	body := w.Body.String()

	// term.open() must be called before the WebSocket is created (not deferred to ws.onopen)
	assert.Contains(t, body, "term.open(")
	assert.Contains(t, body, "new WebSocket(")
	assert.Less(t,
		strings.Index(body, "term.open("),
		strings.Index(body, "new WebSocket("),
		"term.open() must be called before WebSocket is created",
	)

	// WebSocket connects to /ws/<host> on the same origin
	assert.Contains(t, body, "/ws/")

	// xterm assets loaded from /static
	assert.Contains(t, body, "/static/xterm/lib/xterm.js")
	assert.Contains(t, body, "/static/xterm/css/xterm.css")
}
