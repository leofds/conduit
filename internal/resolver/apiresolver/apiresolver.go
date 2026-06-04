// Package apiresolver resolves session configuration by calling an external REST API.
// Implement the API endpoints in your own backend to integrate Conduit with your auth system.
//
// Two endpoints are derived from the base URL configured in conduit.yaml:
//   - POST <url>/ssh   — resolve an SSH host
//   - POST <url>/local — resolve a local shell session
package apiresolver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/resolver"
)

// Config holds the connection parameters for the API resolver.
type Config struct {
	URL             string
	ConnectTimeout  time.Duration
	ResponseTimeout time.Duration
}

// Resolver calls an external REST API to resolve session configuration.
type Resolver struct {
	url    string
	client *http.Client
}

func New(cfg Config) (*Resolver, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil || !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		return nil, fmt.Errorf("apiresolver: url must start with http:// or https://")
	}
	connTimeout := cfg.ConnectTimeout
	if connTimeout == 0 {
		connTimeout = 5 * time.Second
	}
	respTimeout := cfg.ResponseTimeout
	if respTimeout == 0 {
		respTimeout = 10 * time.Second
	}
	return &Resolver{
		url: cfg.URL,
		client: &http.Client{
			Timeout: respTimeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{Timeout: connTimeout}).DialContext,
			},
		},
	}, nil
}

// requestBody is the JSON payload sent to both endpoints.
type RequestBody struct {
	Host string `json:"host"`
}

// SSHResponseBody is the JSON payload returned by the /ssh endpoint.
type SSHResponseBody struct {
	Address           string            `json:"address,omitempty"`
	Port              string            `json:"port,omitempty"`
	Username          string            `json:"username,omitempty"`
	Password          string            `json:"password,omitempty"`
	PrivateKeyFile    string            `json:"private_key_file,omitempty"`
	Term              string            `json:"term,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	IdleTimeout       string            `json:"idle_timeout,omitempty"`
	KeepaliveInterval string            `json:"keepalive_interval,omitempty"`
	TOFUAutoAccept    *bool             `json:"tofu_auto_accept,omitempty"`
	VerifyHostKey     *bool             `json:"verify_host_key,omitempty"`
}

// LocalResponseBody is the JSON payload returned by the /local endpoint.
type LocalResponseBody struct {
	Command     string            `json:"command,omitempty"`
	Term        string            `json:"term,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
	IdleTimeout string            `json:"idle_timeout,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

func (r *Resolver) Resolve(req resolver.Request) (resolver.SessionConfig, error) {
	isLocal := req.Host == config.Local
	endpoint := r.url + "/ssh"
	if isLocal {
		endpoint = r.url + "/local"
	}

	var httpReq *http.Request
	if isLocal {
		var err error
		httpReq, err = http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, http.NoBody)
		if err != nil {
			return nil, fmt.Errorf("apiresolver: build request: %w", err)
		}
	} else {
		body, err := json.Marshal(RequestBody{Host: req.Host})
		if err != nil {
			return nil, fmt.Errorf("apiresolver: marshal: %w", err)
		}
		httpReq, err = http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("apiresolver: build request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if req.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.Token)
	}

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("apiresolver: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		// handled below
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("apiresolver: unauthorized")
	case http.StatusNotFound:
		return nil, fmt.Errorf("apiresolver: host %s not found", req.Host)
	default:
		return nil, fmt.Errorf("apiresolver: unexpected status %d", resp.StatusCode)
	}

	if isLocal {
		var result LocalResponseBody
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("apiresolver: decode response: %w", err)
		}
		var idleTimeout *time.Duration
		if result.IdleTimeout != "" {
			d, err := time.ParseDuration(result.IdleTimeout)
			if err != nil {
				return nil, fmt.Errorf("apiresolver: invalid idle_timeout %q: %w", result.IdleTimeout, err)
			}
			idleTimeout = &d
		}
		return resolver.LocalConfig{
			Command:     result.Command,
			Term:        result.Term,
			WorkingDir:  result.WorkingDir,
			IdleTimeout: idleTimeout,
			Env:         result.Env,
		}, nil
	}

	var result SSHResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("apiresolver: decode response: %w", err)
	}
	var idleTimeout *time.Duration
	if result.IdleTimeout != "" {
		d, err := time.ParseDuration(result.IdleTimeout)
		if err != nil {
			return nil, fmt.Errorf("apiresolver: invalid idle_timeout %q: %w", result.IdleTimeout, err)
		}
		idleTimeout = &d
	}
	var keepaliveInterval *time.Duration
	if result.KeepaliveInterval != "" {
		d, err := time.ParseDuration(result.KeepaliveInterval)
		if err != nil {
			return nil, fmt.Errorf("apiresolver: invalid keepalive_interval %q: %w", result.KeepaliveInterval, err)
		}
		keepaliveInterval = &d
	}
	return resolver.SSHConfig{
		Address:           result.Address,
		Port:              result.Port,
		Username:          result.Username,
		Password:          result.Password,
		PrivateKeyFile:    result.PrivateKeyFile,
		Term:              result.Term,
		Env:               result.Env,
		TOFUAutoAccept:    result.TOFUAutoAccept,
		VerifyHostKey:     result.VerifyHostKey,
		IdleTimeout:       idleTimeout,
		KeepaliveInterval: keepaliveInterval,
	}, nil
}
