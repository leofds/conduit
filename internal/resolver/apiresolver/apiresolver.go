// Package apiresolver resolves session configuration by calling an external REST API.
// Implement the API endpoint in your own backend to integrate Conduit with your auth system.
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

// requestBody is the JSON payload sent to the API.
// Type is "ssh" for SSH hosts and "local" for local shell requests.
type requestBody struct {
	Type config.SessionType `json:"type"`
	Host string             `json:"host"`
}

// responseBody is the JSON payload returned by the API.
// The "type" field must be "ssh" or "local".
//
// SSH example:
//
//	{"type":"ssh","address":"192.168.1.10","port":"22","username":"admin","password":""}
//
// Local example:
//
//	{"type":"local","command":"/bin/bash"}
type responseBody struct {
	Type     config.SessionType `json:"type"`
	Address  string             `json:"address"`
	Port     string             `json:"port"`
	Username string             `json:"username"`
	Password string             `json:"password"`
	Command  string             `json:"command"`
}

func (r *Resolver) Resolve(req resolver.Request) (resolver.SessionConfig, error) {
	reqType := config.SessionTypeSSH
	if req.Host == config.Local {
		reqType = config.SessionTypeLocal
	}
	body, err := json.Marshal(requestBody{Type: reqType, Host: req.Host})
	if err != nil {
		return nil, fmt.Errorf("apiresolver: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("apiresolver: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
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
	default:
		return nil, fmt.Errorf("apiresolver: unexpected status %d", resp.StatusCode)
	}

	var result responseBody
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("apiresolver: decode response: %w", err)
	}

	switch result.Type {
	case config.SessionTypeSSH:
		port := result.Port
		if port == "" {
			port = "22"
		}
		return resolver.SSHConfig{
			Address:  result.Address,
			Port:     port,
			Username: result.Username,
			Password: result.Password,
		}, nil
	case config.SessionTypeLocal:
		command := result.Command
		return resolver.LocalConfig{
			Command: command,
		}, nil
	default:
		return nil, fmt.Errorf("apiresolver: unknown session type %q", result.Type)
	}
}
