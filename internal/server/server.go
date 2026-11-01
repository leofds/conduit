package server

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/leofds/conduit/knownhosts"
	"github.com/leofds/conduit/resolver"
	"github.com/leofds/conduit/session"
)

//go:embed static
var staticFiles embed.FS

// SSHOptions are the server-level defaults applied to resolved SSH sessions.
type SSHOptions struct {
	Port              string
	Term              string
	IdleTimeout       time.Duration
	KeepaliveInterval time.Duration
	DialTimeout       time.Duration
	VerifyHostKey     bool
	AutoAcceptHostKey bool
	Env               map[string]string
}

// LocalOptions are the server-level defaults applied to resolved local sessions.
type LocalOptions struct {
	Command     string
	Term        string
	WorkingDir  string
	IdleTimeout time.Duration
	Env         map[string]string
}

// Timeouts mirrors the embedded HTTP server timeout settings.
type Timeouts struct {
	Read        time.Duration
	Write       time.Duration
	ReadHeader  time.Duration
	Idle        time.Duration
	WSHandshake time.Duration
}

// Options configures an internal Server. It is populated by the public façade.
type Options struct {
	Resolver        resolver.Resolver
	SessionManager  *session.Manager
	KnownHosts      knownhosts.Provider
	RequestMapper   func(*http.Request) resolver.Request
	AllowLocal      bool
	Demo            bool
	DebugBanner     bool
	AllowedOrigins  []string
	HTTPHeaders     map[string]string
	TerminalOptions map[string]any
	SSH             SSHOptions
	Local           LocalOptions
	Timeouts        Timeouts
}

type Server struct {
	router          *gin.Engine
	httpServer      *http.Server
	resolver        resolver.Resolver
	allowLocal      bool
	demo            bool
	debugBanner     bool
	terminalOptions map[string]any
	requestMapper   func(*http.Request) resolver.Request
	sshCfg          SSHOptions
	localCfg        LocalOptions
	allowedOrigins  []string
	knownHosts      knownhosts.Provider
	httpHeaders     map[string]string
	timeouts        Timeouts
	sessionManager  *session.Manager
}

func New(opts Options) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	s := &Server{
		router:          router,
		resolver:        opts.Resolver,
		allowLocal:      opts.AllowLocal,
		demo:            opts.Demo,
		debugBanner:     opts.DebugBanner,
		terminalOptions: opts.TerminalOptions,
		requestMapper:   opts.RequestMapper,
		sshCfg:          opts.SSH,
		localCfg:        opts.Local,
		allowedOrigins:  opts.AllowedOrigins,
		knownHosts:      opts.KnownHosts,
		httpHeaders:     opts.HTTPHeaders,
		timeouts:        opts.Timeouts,
		sessionManager:  opts.SessionManager,
	}
	s.router.Use(securityHeaders(opts.HTTPHeaders))
	s.registerRoutes()

	return s
}

// SetRequestMapper overrides how the resolver.Request is computed per connection.
func (s *Server) SetRequestMapper(fn func(*http.Request) resolver.Request) {
	s.requestMapper = fn
}

// SetDemo toggles the demo page at runtime.
func (s *Server) SetDemo(enabled bool) {
	s.demo = enabled
}

// Handler returns the HTTP handler so the server can be mounted on a host router.
func (s *Server) Handler() http.Handler {
	return s.router
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

// buildHTTPServer constructs the *http.Server from the configured timeouts.
func (s *Server) buildHTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadTimeout:       s.timeouts.Read,
		WriteTimeout:      s.timeouts.Write,
		ReadHeaderTimeout: s.timeouts.ReadHeader,
		IdleTimeout:       s.timeouts.Idle,
	}
}

// Start creates a listener on addr, owns it, and serves until shutdown.
func (s *Server) Start(addr string) error {
	s.httpServer = s.buildHTTPServer(addr)
	return s.httpServer.ListenAndServe()
}

// Serve serves on a caller-provided listener (e.g. for TLS/mTLS the host controls).
func (s *Server) Serve(ln net.Listener) error {
	s.httpServer = s.buildHTTPServer("")
	return s.httpServer.Serve(ln)
}

func (s *Server) Shutdown() error {
	if s.httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}
