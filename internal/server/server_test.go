package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthEndpoint(t *testing.T) {
	s := New()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}

func TestIndexServesLoginForm(t *testing.T) {
	s := New()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")

	body := w.Body.String()

	// login form fields
	assert.Contains(t, body, `id="host"`)
	assert.Contains(t, body, `id="user"`)
	assert.Contains(t, body, `type="submit"`)

	// redirects to /terminal with params
	assert.Contains(t, body, "/terminal")
}

func TestTerminalServesXterm(t *testing.T) {
	s := New()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/terminal", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")

	body := w.Body.String()

	// terminal opens immediately, not inside ws.onopen
	assert.True(t, strings.Contains(body, "term.open("), "term.open must be called")
	assert.False(t, strings.Contains(body, "ws.onopen"), "term.open must not be deferred to ws.onopen")

	// WebSocket connects to /ws on same origin
	assert.Contains(t, body, `new WebSocket(`)
	assert.Contains(t, body, `/ws?`)

	// xterm assets loaded from /static
	assert.Contains(t, body, "/static/xterm/lib/xterm.js")
	assert.Contains(t, body, "/static/xterm/css/xterm.css")
}
