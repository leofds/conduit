// mockapi is a lightweight HTTP server that simulates the Conduit API resolver
// endpoints with static responses. Use it to test the apiresolver without
// needing a real backend.
//
// The listen address and base endpoint path are read from conduit.yaml (api.url).
// Two sub-paths are registered:
//   - POST <path>/ssh   — returns SSH session config
//   - POST <path>/local — returns local shell config
//
// Incoming requests are validated against the embedded OpenAPI spec.
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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi3"

	conduitapi "github.com/leofds/conduit/api"
	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/resolver"
	"github.com/leofds/conduit/internal/resolver/fileresolver"
)

// These structs mirror apiresolver's request/response bodies.
type resolveRequest struct {
	Host string `json:"host"`
}

type sshResponse struct {
	Address        string            `json:"address,omitempty"`
	Port           string            `json:"port,omitempty"`
	Username       string            `json:"username,omitempty"`
	Password       string            `json:"password,omitempty"`
	PrivateKeyFile string            `json:"private_key_file,omitempty"`
	Term           string            `json:"term,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	TOFUAutoAccept *bool             `json:"tofu_auto_accept,omitempty"`
}

type localResponse struct {
	Command     string            `json:"command,omitempty"`
	Term        string            `json:"term,omitempty"`
	WorkingDir  *string           `json:"working_dir,omitempty"`
	IdleTimeout *string           `json:"idle_timeout,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

// validateBody reads r.Body, validates its JSON against the named schema in doc,
// restores r.Body so the handler can read it again, and returns any validation error.
func validateBody(doc *openapi3.T, schemaName string, r *http.Request) error {
	schemaRef, ok := doc.Components.Schemas[schemaName]
	if !ok || schemaRef == nil {
		return nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("reading body: %w", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := schemaRef.Value.VisitJSON(data); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func makeSSHHandler(doc *openapi3.T, fr *fileresolver.Resolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := validateBody(doc, "SSHRequest", r); err != nil {
			log.Printf("mockapi: ssh request validation failed: %v", err)
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}

		var req resolveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("mockapi: bad request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		auth := r.Header.Get("Authorization")
		log.Printf("mockapi: ssh host=%s auth=%q", req.Host, auth)

		cfg, err := fr.Resolve(resolver.Request{Host: req.Host})
		if err != nil {
			log.Printf("mockapi: resolve error: %v", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		v, ok := cfg.(resolver.SSHConfig)
		if !ok {
			http.Error(w, "not an SSH host", http.StatusBadRequest)
			return
		}
		resp := sshResponse{
			Address:        v.Address,
			Port:           v.Port,
			Username:       v.Username,
			Password:       v.Password,
			PrivateKeyFile: v.PrivateKeyFile,
			Term:           v.Term,
			Env:            v.Env,
			TOFUAutoAccept: v.TOFUAutoAccept,
		}

		if b, err := json.Marshal(resp); err == nil {
			log.Printf("mockapi: ssh response: %s", b)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("mockapi: encode error: %v", err)
		}
	}
}

func makeLocalHandler(doc *openapi3.T, fr *fileresolver.Resolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		auth := r.Header.Get("Authorization")
		log.Printf("mockapi: local auth=%q", auth)

		cfg, err := fr.Resolve(resolver.Request{Host: config.Local})
		if err != nil {
			log.Printf("mockapi: resolve error: %v", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		v, ok := cfg.(resolver.LocalConfig)
		if !ok {
			http.Error(w, "not a local session", http.StatusBadRequest)
			return
		}
		resp := localResponse{
			Command:    v.Command,
			Term:       v.Term,
			WorkingDir: v.WorkingDir,
			Env:        v.Env,
		}
		if v.IdleTimeout != nil {
			s := v.IdleTimeout.String()
			resp.IdleTimeout = &s
		}

		if b, err := json.Marshal(resp); err == nil {
			log.Printf("mockapi: ssh response: %s", b)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("mockapi: encode error: %v", err)
		}
	}
}

// chdirToBin changes the working directory to the directory containing the
// binary so that all relative paths in config files resolve consistently,
// regardless of where conduit was invoked from.
func chdirToBin() {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("warning: could not determine executable path: %v", err)
		return
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		log.Printf("warning: could not resolve executable symlinks: %v", err)
		return
	}
	if err := os.Chdir(filepath.Dir(exe)); err != nil {
		log.Printf("warning: could not chdir to binary directory: %v", err)
	}
}

func main() {
	chdirToBin()
	addr := ":8040"
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

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(conduitapi.Spec)
	if err != nil {
		log.Fatalf("mockapi: load openapi spec: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		log.Fatalf("mockapi: invalid openapi spec: %v", err)
	}
	log.Printf("mockapi: OpenAPI spec loaded and validated")

	fr, err := fileresolver.NewFromPaths(config.HostsConfigPaths, cfg.Local)
	if err != nil {
		log.Fatalf("mockapi: load hosts: %v", err)
	}

	http.HandleFunc(endpoint+"/ssh", makeSSHHandler(doc, fr))
	http.HandleFunc(endpoint+"/local", makeLocalHandler(doc, fr))
	log.Printf("mockapi: listening on %s, endpoints %s/ssh and %s/local", addr, endpoint, endpoint)
	if err := http.ListenAndServe(addr, nil); err != nil { //nolint:gosec
		log.Fatalf("mockapi: %v", err)
	}
}
