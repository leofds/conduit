package server

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/knownhosts"
	"github.com/leofds/conduit/internal/resolver"
)

//go:embed static
var staticFiles embed.FS

type Server struct {
	router             *gin.Engine
	httpServer         *http.Server
	resolver           resolver.Resolver
	allowLocal         bool
	demo               bool
	debugBanner        bool
	terminalOptions    map[string]any
	sshCfg             config.SSHConfig
	localCfg           config.LocalShellConfig
	allowedOrigins     []string
	knownHosts         *knownhosts.Store
	httpHeaders        map[string]string
	serverConfig       config.ServerConfig
}

func New(r resolver.Resolver, serverConfig config.ServerConfig, headers map[string]string) *Server {
	gin.SetMode(gin.ReleaseMode)
	gin := gin.Default()

	s := &Server{
		router:       gin,
		resolver:     r,
		allowLocal:   true,
		demo:         true,
		httpHeaders:  headers,
		serverConfig: serverConfig,
	}
	s.router.Use(securityHeaders(headers))
	s.registerRoutes()

	return s
}

// SetTerminalOptions sets the options passed to the xterm.js Terminal constructor.
func (s *Server) SetTerminalOptions(opts map[string]any) {
	s.terminalOptions = opts
}

// SetAllowLocal controls whether local shell sessions are permitted.
func (s *Server) SetAllowLocal(allow bool) {
	s.allowLocal = allow
}

// SetDebugBanner controls whether a banner with session details is shown before the shell.
func (s *Server) SetDebugBanner(debug bool) {
	s.debugBanner = debug
}

// SetDemo controls whether the demo page is enabled.
func (s *Server) SetDemo(enabled bool) {
	s.demo = enabled
}

// SetSSHConfig applies SSH session-level parameters (timeouts, keepalive).
func (s *Server) SetSSHConfig(cfg config.SSHConfig) {
	s.sshCfg = cfg
}

// SetLocalConfig sets the global defaults for local shell sessions.
func (s *Server) SetLocalConfig(cfg config.LocalShellConfig) {
	s.localCfg = cfg
}

// SetAllowedOrigins sets the WebSocket Origin allowlist.
// An empty list permits all origins (default, suitable when behind a trusted reverse proxy).
func (s *Server) SetAllowedOrigins(origins []string) {
	s.allowedOrigins = origins
}

// SetKnownHosts sets the store used for SSH host key TOFU verification.
func (s *Server) SetKnownHosts(ks *knownhosts.Store) {
	s.knownHosts = ks
}

func (s *Server) registerRoutes() {
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	s.router.GET("/", func(c *gin.Context) {
		if !s.demo {
			c.Status(http.StatusNotFound)
			return
		}
		c.Redirect(http.StatusFound, "/demo")
	})

	s.router.GET("/demo", func(c *gin.Context) {
		if !s.demo {
			c.Status(http.StatusNotFound)
			return
		}
		data, err := staticFiles.ReadFile("static/demo.html")
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	s.router.GET("/terminal", func(c *gin.Context) {
		data, err := staticFiles.ReadFile("static/terminal.html")
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		content := string(data)
		jsonBytes, err := json.Marshal(s.terminalOptions)
		if err == nil {
			content = strings.ReplaceAll(content, "/*CONDUIT_TERMINAL_OPTS*/", string(jsonBytes))
		} else {
			log.Printf("Failed to marshal terminal options: %v", err)
			content = strings.ReplaceAll(content, "/*CONDUIT_TERMINAL_OPTS*/", `{}`)
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(content))
	})

	sub, _ := fs.Sub(staticFiles, "static")
	s.router.StaticFS("/static", http.FS(sub))

	s.router.GET("/ws/:host", s.wsHandler)
}

func (s *Server) Start(addr string) error {
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadTimeout:       s.serverConfig.Timeouts.Read,
		WriteTimeout:      s.serverConfig.Timeouts.Write,
		ReadHeaderTimeout: s.serverConfig.Timeouts.ReadHeader,
		IdleTimeout:       s.serverConfig.Timeouts.Idle,
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}
