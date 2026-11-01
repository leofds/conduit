// Package main demonstrates embedding Conduit using only the public module:
// it starts the HTTP server and shuts it down gracefully on SIGINT/SIGTERM,
// mirroring how cmd/conduit/main.go wires things up, but without config files,
// the API resolver, or the Unix control socket.
//
// Host key fingerprints are persisted to a file via the built-in TOFU store.
//
// Run with:
//
//	go run ./examples/server
//
// Then open http://localhost:9090/demo and connect to host "demo".
// Press Ctrl+C to trigger a graceful shutdown.
package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	conduit "github.com/leofds/conduit"
	"github.com/leofds/conduit/examples/internal/defaults"
	"github.com/leofds/conduit/knownhosts"
	"github.com/leofds/conduit/resolver"
)

// myResolver resolves hosts from an in-memory table and validates the token.
// In a real application this would query your database or an internal service.
type myResolver struct {
	hosts map[string]resolver.SSHConfig
}

func (r myResolver) Resolve(req resolver.Request) (resolver.SessionConfig, error) {
	if req.Token == "" {
		return nil, fmt.Errorf("missing token")
	}
	if req.Host == "local" {
		// Signal a local session; command/term come from the global defaults
		// configured in conduit.LocalOptions below.
		return resolver.LocalConfig{}, nil
	}
	cfg, ok := r.hosts[req.Host]
	if !ok {
		return nil, fmt.Errorf("host %q not found", req.Host)
	}
	return cfg, nil
}

func main() {

	// Example host table. In a real application this would be dynamic.
	hosts := map[string]resolver.SSHConfig{
		"demo": { // "demo" is a placeholder; replace with your own host.
			Address:  "127.0.0.1", // replace with your host's address
			Username: "user",      // replace with your username
			//Password: "password",  // omit for key-based auth or use a private key with PrivateKeyPath
		},
	}

	// Use a file-based known-hosts store to persist host key fingerprints across restarts.
	provider, err := knownhosts.New("/tmp/known_hosts.yaml") // replace with your desired path
	if err != nil {
		log.Fatalf("Failed to create known hosts store: %v", err)
	}

	// Start from the standard defaults (timeouts, terminal options, SSH/local
	// defaults), then set only what this example needs.
	opts := defaults.Options()
	opts.Resolver = myResolver{hosts: hosts}
	opts.KnownHosts = provider

	srv, err := conduit.New(opts)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start the HTTP server in a goroutine; Start owns the listener and blocks.
	addr := ":9090"
	go func() {
		log.Printf("Starting conduit on %s (http://localhost%s/demo)", addr, addr)
		if err := srv.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for SIGINT/SIGTERM, then shut down gracefully.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	if err := srv.Shutdown(); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
	log.Println("Stopped")
}
