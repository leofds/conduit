package server

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/resolver"
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

// tokenFromCookie returns the value of the conduit_auth cookie, or empty string if absent.
func tokenFromCookie(c *gin.Context) string {
	token, _ := c.Cookie("conduit_token")
	return token
}

// wsHandler upgrades the connection to WebSocket and dispatches to the appropriate session runner.
// Session configuration (method, credentials, host, port, shell) is resolved via s.resolver.
func (s *Server) wsHandler(c *gin.Context) {
	host := c.Param("host")
	user := c.Query("user")
	token := tokenFromCookie(c)

	if host == config.Local && !s.allowLocal {
		log.Printf("local shell session blocked (enable_local_shell=false)")
		c.JSON(http.StatusForbidden, gin.H{"error": "local shell sessions are disabled"})
		return
	}

	cfg, err := s.resolver.Resolve(resolver.Request{Host: host, User: user, Token: token})
	if err != nil {
		log.Printf("resolver error host=%s user=%s: %v", host, user, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Clear the token cookie now that it has been consumed.
	if token != "" {
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     "conduit_token",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			SameSite: http.SameSiteLaxMode,
		})
	}

	var runner session.Runner
	switch sess := cfg.(type) {
	case resolver.SSHConfig:
		runner = sessionssh.New(sess)
		log.Printf("session open  method=ssh user=%s host=%s", sess.Username, sess.Address)
		defer log.Printf("session close method=ssh user=%s host=%s", sess.Username, sess.Address)
	case resolver.LocalConfig:
		runner = sessionlocal.New(sess)
		log.Printf("session open  method=local user=%s", sess.Username)
		defer log.Printf("session close method=local user=%s", sess.Username)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported session type"})
		return
	}

	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	defer func() { _ = wsConn.Close() }()

	runner.Run(c.Request.Context(), wsConn)
}
