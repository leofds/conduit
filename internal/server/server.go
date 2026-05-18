package server

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/leofds/conduit/internal/resolver"
)

//go:embed static
var staticFiles embed.FS

type Server struct {
	router     *gin.Engine
	httpServer *http.Server
	resolver   resolver.Resolver
	allowLocal bool
}

func New(r resolver.Resolver) *Server {
	gin := gin.Default()

	s := &Server{router: gin, resolver: r, allowLocal: true}
	s.registerRoutes()

	return s
}

// SetAllowLocal controls whether local shell sessions are permitted.
func (s *Server) SetAllowLocal(allow bool) {
	s.allowLocal = allow
}

func (s *Server) registerRoutes() {
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	s.router.GET("/", func(c *gin.Context) {
		data, err := staticFiles.ReadFile("static/index.html")
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
