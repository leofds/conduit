package server

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/resolver"
	"github.com/leofds/conduit/internal/session"
	sessionlocal "github.com/leofds/conduit/internal/session/local"
	sessionssh "github.com/leofds/conduit/internal/session/ssh"
)

// tokenFromCookie returns the value of the conduit_auth cookie, or empty string if absent.
func tokenFromCookie(c *gin.Context) string {
	token, _ := c.Cookie("conduit_session")
	return token
}

// wsHandler upgrades the connection to WebSocket and dispatches to the appropriate session runner.
// Session configuration (method, credentials, host, port, shell) is resolved via s.resolver.
func (s *Server) wsHandler(c *gin.Context) {
	host := c.Param("host")
	token := tokenFromCookie(c)

	cols := parseUint16(c.Query("cols"), 80)
	rows := parseUint16(c.Query("rows"), 24)

	if host == config.Local && !s.allowLocal {
		log.Printf("local shell session blocked (allow_local_shell=false)")
		c.JSON(http.StatusForbidden, gin.H{"error": "local shell sessions are disabled"})
		return
	}

	cfg, err := s.resolver.Resolve(resolver.Request{Host: host, Token: token})
	if err != nil {
		log.Printf("resolver error host=%q: %v", host, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Clear the token cookie now that it has been consumed.
	if token != "" {
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     "conduit_session",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	var runner session.Runner
	var bannerCfg any
	switch sess := cfg.(type) {
	case resolver.SSHConfig:
		address := sess.Address
		if address == "" {
			err := fmt.Errorf("address not found")
			log.Printf("resolver error host=%q: %v", host, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		port := s.sshCfg.Port
		if sess.Port != "" {
			port = sess.Port
		}
		username := sess.Username
		if username == "" {
			err := fmt.Errorf("username not found")
			log.Printf("resolver error host=%q: %v", host, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		password := sess.Password
		privateKeyFile := sess.PrivateKeyFile
		term := s.sshCfg.Term
		if sess.Term != "" {
			term = sess.Term
		}
		verifyHostKey := s.sshCfg.VerifyHostKey
		if sess.VerifyHostKey != nil {
			verifyHostKey = *sess.VerifyHostKey
		}
		autoAcceptHostKey := s.sshCfg.AutoAcceptHostKey
		if sess.AutoAcceptHostKey != nil {
			autoAcceptHostKey = *sess.AutoAcceptHostKey
		}
		idleTimeout := s.sshCfg.IdleTimeout
		if sess.IdleTimeout != nil {
			idleTimeout = *sess.IdleTimeout
		}
		keepaliveInterval := s.sshCfg.KeepaliveInterval
		if sess.KeepaliveInterval != nil {
			keepaliveInterval = *sess.KeepaliveInterval
		}
		sshEnv := make(map[string]string, len(s.sshCfg.Env)+len(sess.Env))
		for k, v := range s.sshCfg.Env {
			sshEnv[k] = v
		}
		for k, v := range sess.Env {
			sshEnv[k] = v
		}
		sess.Env = sshEnv
		knownFP := ""
		var saveHostKey func(string) error
		if verifyHostKey && s.knownHosts != nil {
			knownFP = s.knownHosts.Get(host)
			saveHostKey = func(fp string) error { return s.knownHosts.Set(host, fp) }
		}
		sshCfg := sessionssh.Config{
			Address:           address,
			Port:              port,
			Username:          username,
			Password:          password,
			PrivateKeyFile:    privateKeyFile,
			Term:              term,
			IdleTimeout:       idleTimeout,
			KeepaliveInterval: keepaliveInterval,
			DialTimeout:       s.sshCfg.DialTimeout,
			VerifyHostKey:     verifyHostKey,
			AutoAcceptHostKey: autoAcceptHostKey,
			KnownFingerprint:  knownFP,
			SaveHostKey:       saveHostKey,
			Env:               sshEnv,
			DebugBanner:       s.debugBanner,
		}
		runner = sessionssh.New(sshCfg, cols, rows)
		bannerCfg = sshCfg
		log.Printf("session open  method=ssh user=%s host=%s", sess.Username, sess.Address)
		defer log.Printf("session close method=ssh user=%s host=%s", sess.Username, sess.Address)
	case resolver.LocalConfig:
		command := s.localCfg.Command
		if sess.Command != "" {
			command = sess.Command
		}
		term := s.localCfg.Term
		if sess.Term != "" {
			term = sess.Term
		}
		localWorkingDir := s.localCfg.WorkingDir
		if sess.WorkingDir != "" {
			localWorkingDir = sess.WorkingDir
		}
		localIdleTimeout := s.localCfg.IdleTimeout
		if sess.IdleTimeout != nil {
			localIdleTimeout = *sess.IdleTimeout
		}
		// Join global and session-specific env, with session taking precedence.
		localEnv := make(map[string]string, len(s.localCfg.Env)+len(sess.Env))
		for k, v := range s.localCfg.Env {
			localEnv[k] = v
		}
		for k, v := range sess.Env {
			localEnv[k] = v
		}
		localCfg := sessionlocal.Config{
			Command:     command,
			Term:        term,
			WorkingDir:  localWorkingDir,
			IdleTimeout: localIdleTimeout,
			Env:         localEnv,
		}
		runner = sessionlocal.New(localCfg, cols, rows)
		bannerCfg = localCfg
		log.Printf("session open  method=local command=%s", sess.Command)
		defer log.Printf("session close method=local command=%s", sess.Command)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported session type"})
		return
	}

	upgrader := websocket.Upgrader{
		HandshakeTimeout: s.serverConfig.Timeouts.WSHandshake,
		CheckOrigin: func(r *http.Request) bool {
			if len(s.allowedOrigins) == 0 {
				return true
			}
			origin := r.Header.Get("Origin")
			for _, allowed := range s.allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			return false
		},
	}

	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	defer func() { _ = wsConn.Close() }()

	if s.debugBanner {
		log.Printf("Debug banner enabled")
		s.writeDebugBanner(wsConn, host, bannerCfg, cols)
	}

	runner.Run(c.Request.Context(), wsConn)
}

func parseUint16(s string, def uint16) uint16 {
	if s == "" {
		return def
	}
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return def
	}
	return uint16(n)
}
