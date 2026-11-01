// Package conduit is an embeddable browser-based terminal server.
//
// Create a Server with New, then either mount its Handler on your own
// http.ServeMux/router or call Start/Serve. Session resolution is delegated to
// the Resolver you provide, so an embedding application can supply hosts and
// credentials directly from its own data sources.
//
// Typical embedding:
//
//	srv, err := conduit.New(conduit.Options{
//		Resolver:   myResolver{},              // implements resolver.Resolver
//		KnownHosts: knownhosts.NewMemory(),    // or your own Provider / file store
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//	mux.Handle("/conduit/", http.StripPrefix("/conduit", srv.Handler()))
package conduit

import (
	"net"
	"net/http"
	"reflect"
	"time"

	iserver "github.com/leofds/conduit/internal/server"
	"github.com/leofds/conduit/knownhosts"
	"github.com/leofds/conduit/resolver"
	"github.com/leofds/conduit/session"
)

// SSHOptions are the server-level defaults applied to every resolved SSH session.
// Per-session values returned by the resolver override these.
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

// LocalOptions are the server-level defaults applied to every resolved local session.
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

// Options configures an embedded Conduit server. All fields are optional except
// Resolver, which must be non-nil.
type Options struct {
	// Resolver is required: it maps a resolver.Request to a session configuration.
	Resolver resolver.Resolver

	// KnownHosts is the TOFU fingerprint store used for SSH host key verification.
	// A nil value defaults to an in-process in-memory store.
	KnownHosts knownhosts.Provider

	// RequestMapper, when non-nil, computes the resolver.Request for each
	// connection, allowing the embedder to derive Host and Token from arbitrary
	// sources (e.g. request context identity or custom headers). When nil, the
	// defaults are used: Host from the URL path and Token from the
	// conduit_session cookie or Authorization: Bearer header.
	RequestMapper func(*http.Request) resolver.Request

	// SessionManager tracks active sessions. Nil creates a new internal manager.
	SessionManager *session.Manager

	AllowLocal      bool
	Demo            bool
	DebugBanner     bool
	AllowedOrigins  []string
	HTTPHeaders     map[string]string
	TerminalOptions map[string]any

	SSH      SSHOptions
	Local    LocalOptions
	Timeouts Timeouts
}

// Server is an embeddable Conduit instance.
type Server struct {
	inner    *iserver.Server
	sessions *session.Manager
}

// New builds a Server from the given options.
func New(opts Options) (*Server, error) {
	manager := opts.SessionManager
	if manager == nil {
		manager = session.NewManager()
	}

	inner := iserver.New(iserver.Options{
		Resolver:        opts.Resolver,
		SessionManager:  manager,
		KnownHosts:      normalizeKnownHosts(opts.KnownHosts),
		RequestMapper:   opts.RequestMapper,
		AllowLocal:      opts.AllowLocal,
		Demo:            opts.Demo,
		DebugBanner:     opts.DebugBanner,
		AllowedOrigins:  opts.AllowedOrigins,
		HTTPHeaders:     opts.HTTPHeaders,
		TerminalOptions: opts.TerminalOptions,
		SSH: iserver.SSHOptions{
			Port:              opts.SSH.Port,
			Term:              opts.SSH.Term,
			IdleTimeout:       opts.SSH.IdleTimeout,
			KeepaliveInterval: opts.SSH.KeepaliveInterval,
			DialTimeout:       opts.SSH.DialTimeout,
			VerifyHostKey:     opts.SSH.VerifyHostKey,
			AutoAcceptHostKey: opts.SSH.AutoAcceptHostKey,
			Env:               opts.SSH.Env,
		},
		Local: iserver.LocalOptions{
			Command:     opts.Local.Command,
			Term:        opts.Local.Term,
			WorkingDir:  opts.Local.WorkingDir,
			IdleTimeout: opts.Local.IdleTimeout,
			Env:         opts.Local.Env,
		},
		Timeouts: iserver.Timeouts{
			Read:        opts.Timeouts.Read,
			Write:       opts.Timeouts.Write,
			ReadHeader:  opts.Timeouts.ReadHeader,
			Idle:        opts.Timeouts.Idle,
			WSHandshake: opts.Timeouts.WSHandshake,
		},
	})

	return &Server{inner: inner, sessions: manager}, nil
}

// Handler returns the HTTP handler so the server can be mounted on a host router.
func (s *Server) Handler() http.Handler {
	return s.inner.Handler()
}

// Start listens on addr (which it owns) and serves until Shutdown.
func (s *Server) Start(addr string) error {
	return s.inner.Start(addr)
}

// Serve serves on a caller-provided listener, e.g. one the embedder controls for
// TLS/mTLS. The caller is responsible for closing the listener after Shutdown.
func (s *Server) Serve(ln net.Listener) error {
	return s.inner.Serve(ln)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() error {
	return s.inner.Shutdown()
}

// Sessions returns the manager for the active sessions, allowing the embedder to
// list or close them without the Unix control socket.
func (s *Server) Sessions() *session.Manager {
	return s.sessions
}

// normalizeKnownHosts guarantees a non-nil Provider. It handles both a plain nil
// interface and a typed-nil pointer (e.g. a nil *knownhosts.Store) by falling
// back to the in-memory store.
func normalizeKnownHosts(p knownhosts.Provider) knownhosts.Provider {
	if p == nil {
		return knownhosts.NewMemory()
	}
	v := reflect.ValueOf(p)
	if v.Kind() == reflect.Pointer && v.IsNil() {
		return knownhosts.NewMemory()
	}
	return p
}
