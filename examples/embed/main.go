// Package main demonstrates embedding Conduit in an existing Go HTTP server
// using a custom resolver and a custom in-memory known-hosts provider.
//
// Run with:
//
//	go run ./examples/embed
//
// Then open http://localhost:9090/demo and connect to host "demo".
//
// Note: Conduit's terminal page loads its assets and the WebSocket on absolute
// paths (/static, /ws/...). To serve it under a sub-path, use a reverse proxy
// or rewrite those paths; mounting at the root is the simplest option.
package main

import (
	"fmt"
	"log"
	"net/http"

	conduit "github.com/leofds/conduit"
	"github.com/leofds/conduit/examples/internal/defaults"
	"github.com/leofds/conduit/knownhosts"
	"github.com/leofds/conduit/resolver"
)

// myResolver resolves hosts from an in-memory table. In a real application this
// would query your database, LDAP, or an internal API — and validate req.Token.
type myResolver struct {
	hosts map[string]resolver.SSHConfig
}

func (r myResolver) Resolve(req resolver.Request) (resolver.SessionConfig, error) {
	// Example auth: reject requests with an empty token.
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

	// Start from the standard defaults (timeouts, terminal options, SSH/local
	// defaults), then set only what this example needs.
	opts := defaults.Options()
	opts.Resolver = myResolver{hosts: hosts}
	opts.KnownHosts = knownhosts.NewMemory() // in-memory TOFU for this example (defaults.Options() uses nil, which also means in-memory TOFU)

	srv, err := conduit.New(opts)
	if err != nil {
		log.Fatal(err)
	}

	// Mount Conduit's handler on your own mux (e.g. behind your own middleware).
	mux := http.NewServeMux()
	mux.Handle("/", srv.Handler())

	log.Println("embed example listening on :9090 (http://localhost:9090/demo)")
	if err := http.ListenAndServe(":9090", mux); err != nil { //nolint:gosec
		log.Fatal(err)
	}
}
