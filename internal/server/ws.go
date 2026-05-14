package server

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/leofds/conduit/internal/session"
	sessionlocal "github.com/leofds/conduit/internal/session/local"
	sessionssh "github.com/leofds/conduit/internal/session/ssh"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO: validate Origin against allowed hosts
		return true
	},
}

// wsHandler upgrades the connection to WebSocket and dispatches to the appropriate session runner.
//
// Query parameters:
//
//	method - "ssh" or "local" (default: "local")
//	user   - username (required for ssh; optional for local)
//	shell  - shell binary for local sessions (default: $SHELL or /bin/sh)
//	host   - SSH host (required for ssh)
//	port   - SSH port (default: 22, ssh only)
func wsHandler(c *gin.Context) {
	method := c.DefaultQuery("method", "local")
	user := c.Query("user")
	shell := c.Query("shell")

	var runner session.Runner
	switch method {
	case "ssh":
		host := c.Query("host")
		port := c.DefaultQuery("port", "22")
		if host == "" || user == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "host and user are required for SSH"})
			return
		}
		runner = sessionssh.New(host, port, user)
	case "local":
		runner = sessionlocal.New(user, shell)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown method: " + method})
		return
	}

	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	defer func() { _ = wsConn.Close() }()

	log.Printf("session open  method=%s user=%s", method, user)
	runner.Run(c.Request.Context(), wsConn)
	log.Printf("session close method=%s user=%s", method, user)
}
