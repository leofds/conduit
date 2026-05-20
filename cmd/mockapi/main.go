// mockapi is a lightweight HTTP server that simulates the Conduit API resolver
// endpoint with static responses. Use it to test the apiresolver without
// needing a real backend.
//
// The listen address and endpoint path are read from conduit.yaml (api.url).
//
// Usage:
//
//	make run-mockapi
//	go run ./cmd/mockapi
//
// Point conduit.yaml at it:
//
//	resolver: api
//	api:
//	  url: http://localhost:8080/conduit/resolve
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/resolver"
	"github.com/leofds/conduit/internal/resolver/fileresolver"
)

var mockapiHostsPaths = []string{
	"/etc/conduit/hosts.yaml",
	"/etc/conduit/hosts.yaml",
	"./hosts.yaml",
	"./hosts.yaml",
}

// These structs mirror apiresolver's request/response bodies.
type resolveRequest struct {
	Type string `json:"type"`
	Host string `json:"host"`
	User string `json:"user"`
}

type resolveResponse struct {
	Type     string `json:"type"`
	Address  string `json:"address,omitempty"`
	Port     string `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Shell    string `json:"shell,omitempty"`
}

func makeHandler(fr *fileresolver.Resolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req resolveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("mockapi: bad request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		auth := r.Header.Get("Authorization")
		log.Printf("mockapi: type=%s host=%s user=%s auth=%q", req.Type, req.Host, req.User, auth)

		cfg, err := fr.Resolve(resolver.Request{Host: req.Host, User: req.User})
		if err != nil {
			log.Printf("mockapi: resolve error: %v", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		var resp resolveResponse
		switch v := cfg.(type) {
		case resolver.SSHConfig:
			resp = resolveResponse{
				Type:     string(config.SessionTypeSSH),
				Address:  v.Address,
				Port:     v.Port,
				Username: v.Username,
				Password: v.Password,
			}
		case resolver.LocalConfig:
			resp = resolveResponse{
				Type:     string(config.SessionTypeLocalShell),
				Shell:    v.Shell,
				Username: v.Username,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("mockapi: encode error: %v", err)
		}
	}
}

func main() {
	addr := ":8080"
	endpoint := "/conduit/resolve"
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("mockapi: load config: %v", err)
	}
	if cfg.API.URL != "" {
		if u, err := url.Parse(cfg.API.URL); err == nil {
			if u.Port() != "" {
				addr = ":" + u.Port()
			}
			if u.Path != "" {
				endpoint = u.Path
			}
		}
	}

	fr, err := fileresolver.NewFromPaths(mockapiHostsPaths)
	if err != nil {
		log.Fatalf("mockapi: load hosts: %v", err)
	}

	http.HandleFunc(endpoint, makeHandler(fr))
	log.Printf("mockapi: listening on %s, endpoint %s", addr, endpoint)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("mockapi: %v", err)
	}
}
