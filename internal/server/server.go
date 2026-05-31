package server

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/knownhosts"
	"github.com/leofds/conduit/internal/resolver"
)

//go:embed static
var staticFiles embed.FS

type Server struct {
	router           *gin.Engine
	httpServer       *http.Server
	resolver         resolver.Resolver
	allowLocal       bool
	demo             bool
	sshTerm          string
	localTerm        string
	sshCfg           config.SSHConfig
	localIdleTimeout time.Duration
	localWorkingDir  string
	localEnv         map[string]string
	allowedOrigins   []string
	knownHosts       *knownhosts.Store
}

func New(r resolver.Resolver) *Server {
	gin.SetMode(gin.ReleaseMode)
	gin := gin.Default()

	s := &Server{router: gin, resolver: r, allowLocal: true, demo: true, sshTerm: "xterm-256color", localTerm: "xterm-256color"}
	s.registerRoutes()

	return s
}

// SetAllowLocal controls whether local shell sessions are permitted.
func (s *Server) SetAllowLocal(allow bool) {
	s.allowLocal = allow
}

// SetDemo controls whether the demo page is enabled.
func (s *Server) SetDemo(enabled bool) {
	s.demo = enabled
}

// SetSSHTerm sets the terminal type fallback for SSH sessions.
func (s *Server) SetSSHTerm(term string) {
	if term != "" {
		s.sshTerm = term
	}
}

// SetLocalTerm sets the terminal type fallback for local shell sessions.
func (s *Server) SetLocalTerm(term string) {
	if term != "" {
		s.localTerm = term
	}
}

// SetSSHConfig applies SSH session-level parameters (timeouts, keepalive).
func (s *Server) SetSSHConfig(cfg config.SSHConfig) {
	s.sshCfg = cfg
}

// SetLocalIdleTimeout sets the inactivity timeout for local shell sessions.
func (s *Server) SetLocalIdleTimeout(d time.Duration) {
	s.localIdleTimeout = d
}

// SetLocalWorkingDir sets the working directory for local shell sessions.
// An empty string means the session inherits conduit's working directory.
func (s *Server) SetLocalWorkingDir(dir string) {
	s.localWorkingDir = dir
}

// SetLocalEnv sets environment variables that are injected into every local shell session.
func (s *Server) SetLocalEnv(env map[string]string) {
	s.localEnv = env
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
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	sub, _ := fs.Sub(staticFiles, "static")
	s.router.StaticFS("/static", http.FS(sub))

	s.router.GET("/ws/:host", s.wsHandler)
}

func (s *Server) Start(addr string) error {
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}
