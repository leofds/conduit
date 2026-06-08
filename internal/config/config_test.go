package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func defaultTestConfig() *Config {
	return &Config{
		DebugBanner:     true,
		Resolver:        ResolverFile,
		Port:            8080,
		Demo:            true,
		AllowLocalShell: true,
		AllowedOrigins:  nil,
		Headers:         nil,
		TerminalOptions: map[string]any{
			"scrollback": 5000,
			"theme": map[string]any{
				"background": "#1e1e1e",
				"foreground": "#d4d4d4",
			},
		},
		Local: LocalShellConfig{
			Command:     "/bin/bash",
			Term:        "xterm-256color",
			IdleTimeout: 10 * time.Minute,
		},
		SSH: SSHConfig{
			Port:              "22",
			Term:              "xterm-256color",
			IdleTimeout:       10 * time.Minute,
			KeepaliveInterval: 30 * time.Second,
			DialTimeout:       10 * time.Second,
			VerifyHostKey:     true,
			AutoAcceptHostKey: false,
			KnownHostsFile:    "./known_hosts.yaml",
		},
		API: APIConfig{
			ConnectTimeout:  5 * time.Second,
			ResponseTimeout: 10 * time.Second,
		},
	}
}

func compare(a, b Config) (bool, error) {
	// Global settings
	if a.DebugBanner != b.DebugBanner {
		return false, fmt.Errorf("DebugBanner = %v, want %v", a.DebugBanner, b.DebugBanner)
	}
	if a.Resolver != b.Resolver {
		return false, fmt.Errorf("Resolver = %q, want %q", a.Resolver, b.Resolver)
	}
	if a.Port != b.Port {
		return false, fmt.Errorf("Port = %d, want %d", a.Port, b.Port)
	}
	if a.Demo != b.Demo {
		return false, fmt.Errorf("Demo = %v, want %v", a.Demo, b.Demo)
	}
	if a.AllowLocalShell != b.AllowLocalShell {
		return false, fmt.Errorf("AllowLocalShell = %v, want %v", a.AllowLocalShell, b.AllowLocalShell)
	}
	// AllowedOrigins
	if !slices.Equal(a.AllowedOrigins, b.AllowedOrigins) {
		return false, fmt.Errorf("AllowedOrigins = %v, want %v", a.AllowedOrigins, b.AllowedOrigins)
	}
	// Headers
	if !maps.Equal(a.Headers, b.Headers) {
		return false, fmt.Errorf("Headers = %v, want %v", a.Headers, b.Headers)
	}
	// TerminalOptions
	if len(a.TerminalOptions) != len(b.TerminalOptions) {
		return false, fmt.Errorf("TerminalOptions = %v, want %v", a.TerminalOptions, b.TerminalOptions)
	}
	if a.TerminalOptions["scrollback"] != b.TerminalOptions["scrollback"] {
		return false, fmt.Errorf("TerminalOptions[\"scrollback\"] = %v, want 5000", a.TerminalOptions["scrollback"])
	}
	theme := a.TerminalOptions["theme"].(map[string]any)
	if len(theme) != len(b.TerminalOptions["theme"].(map[string]any)) {
		return false, fmt.Errorf("TerminalOptions[\"theme\"] = %v, want %v", theme, b.TerminalOptions["theme"])
	}
	if theme["background"] != b.TerminalOptions["theme"].(map[string]any)["background"] {
		return false, fmt.Errorf("TerminalOptions[\"theme\"][\"background\"] = %v, want #1e1e1e", theme["background"])
	}
	if theme["foreground"] != b.TerminalOptions["theme"].(map[string]any)["foreground"] {
		return false, fmt.Errorf("TerminalOptions[\"theme\"][\"foreground\"] = %v, want #d4d4d4", theme["foreground"])
	}
	// Local session
	if a.Local.Command != b.Local.Command {
		return false, fmt.Errorf("Local.Command = %q, want %q", a.Local.Command, b.Local.Command)
	}
	if a.Local.Term != b.Local.Term {
		return false, fmt.Errorf("Local.Term = %q, want %q", a.Local.Term, b.Local.Term)
	}
	if a.Local.IdleTimeout != b.Local.IdleTimeout {
		return false, fmt.Errorf("Local.IdleTimeout = %v, want %v", a.Local.IdleTimeout, b.Local.IdleTimeout)
	}
	// SSH session
	if a.SSH.Port != b.SSH.Port {
		return false, fmt.Errorf("SSH.Port = %q, want %q", a.SSH.Port, b.SSH.Port)
	}
	if a.SSH.Term != b.SSH.Term {
		return false, fmt.Errorf("SSH.Term = %q, want %q", a.SSH.Term, b.SSH.Term)
	}
	if a.SSH.IdleTimeout != b.SSH.IdleTimeout {
		return false, fmt.Errorf("SSH.IdleTimeout = %v, want %v", a.SSH.IdleTimeout, b.SSH.IdleTimeout)
	}
	if a.SSH.KeepaliveInterval != b.SSH.KeepaliveInterval {
		return false, fmt.Errorf("SSH.KeepaliveInterval = %v, want %v", a.SSH.KeepaliveInterval, b.SSH.KeepaliveInterval)
	}
	if a.SSH.DialTimeout != b.SSH.DialTimeout {
		return false, fmt.Errorf("SSH.DialTimeout = %v, want %v", a.SSH.DialTimeout, b.SSH.DialTimeout)
	}
	if a.SSH.VerifyHostKey != b.SSH.VerifyHostKey {
		return false, fmt.Errorf("SSH.VerifyHostKey = %v, want %v", a.SSH.VerifyHostKey, b.SSH.VerifyHostKey)
	}
	if a.SSH.AutoAcceptHostKey != b.SSH.AutoAcceptHostKey {
		return false, fmt.Errorf("SSH.AutoAcceptHostKey = %v, want %v", a.SSH.AutoAcceptHostKey, b.SSH.AutoAcceptHostKey)
	}
	if a.SSH.KnownHostsFile != b.SSH.KnownHostsFile {
		return false, fmt.Errorf("SSH.KnownHostsFile = %q, want %q", a.SSH.KnownHostsFile, b.SSH.KnownHostsFile)
	}
	// API
	if a.API.ConnectTimeout != b.API.ConnectTimeout {
		return false, fmt.Errorf("API.ConnectTimeout = %v, want %v", a.API.ConnectTimeout, b.API.ConnectTimeout)
	}
	if a.API.ResponseTimeout != b.API.ResponseTimeout {
		return false, fmt.Errorf("API.ResponseTimeout = %v, want %v", a.API.ResponseTimeout, b.API.ResponseTimeout)
	}
	return true, nil
}

func TestLoadReturnsDefaultsWhenNoConfigFilesExist(t *testing.T) {
	paths := []string{
		"dummy.yaml",
	}

	cfg, err := Load(paths)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	expected := defaultTestConfig()
	if equal, err := compare(*cfg, *expected); !equal {
		t.Fatalf("Config does not match expected defaults: %v", err)
	}

	expected.Port = 8000
	if equal, err := compare(*cfg, *expected); equal {
		t.Fatal("Config should not match when expected.Port is changed")
	} else {
		t.Logf("Config correctly does not match when expected.Port is changed: %v", err)
	}
}

type fakeReader struct {
	files map[string][]byte
}

func (f fakeReader) ReadFile(path string) ([]byte, error) {
	if data, ok := f.files[path]; ok {
		return data, nil
	}
	return nil, os.ErrNotExist
}

type fakeDecoder struct{}

func (fakeDecoder) Unmarshal(_ []byte, out any) error {
	cfg := out.(*Config)
	cfg.Port = 4242
	cfg.Resolver = ResolverAPI
	return nil
}

func TestLoadUsesInjectedDependencies(t *testing.T) {
	cfg, err := load([]string{"/tmp/example.yaml"}, fakeReader{files: map[string][]byte{"/tmp/example.yaml": []byte("unused")}}, fakeDecoder{})
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	if cfg.Resolver != ResolverAPI {
		t.Fatalf("Resolver = %q, want %q", cfg.Resolver, ResolverAPI)
	}

	if cfg.Port != 4242 {
		t.Fatalf("Port = %d, want 4242", cfg.Port)
	}
}

func TestLoadMergesFilesAndUsesLaterValues(t *testing.T) {
	oldPaths := append([]string(nil), ConduitConfigPaths...)
	tempDir := t.TempDir()

	first := filepath.Join(tempDir, "first.yaml")
	second := filepath.Join(tempDir, "second.yaml")

	firstContent := `
resolver: file
port: 9000
local:
  command: /bin/sh
  idle_timeout: 15m
ssh:
  port: "22"
`
	secondContent := `
resolver: api
port: 7000
local:
  command: /bin/zsh
ssh:
  port: "2222"
  verify_host_key: false
`

	if err := os.WriteFile(first, []byte(firstContent), 0o644); err != nil {
		t.Fatalf("WriteFile(first) error = %v", err)
	}
	if err := os.WriteFile(second, []byte(secondContent), 0o644); err != nil {
		t.Fatalf("WriteFile(second) error = %v", err)
	}

	ConduitConfigPaths = []string{first, second}
	t.Cleanup(func() {
		ConduitConfigPaths = oldPaths
	})

	paths := []string{first, second}
	cfg, err := Load(paths)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	expected := defaultTestConfig()
	expected.Resolver = ResolverAPI
	expected.Port = 7000
	expected.Local.Command = "/bin/zsh"
	expected.Local.IdleTimeout = 15 * time.Minute
	expected.SSH.Port = "2222"
	expected.SSH.VerifyHostKey = false

	if equal, err := compare(*cfg, *expected); !equal {
		t.Fatalf("Config does not match expected defaults: %v", err)
	}
}

func TestLoadFileWithHeaders(t *testing.T) {
	oldPaths := append([]string(nil), ConduitConfigPaths...)
	tempDir := t.TempDir()

	dummy := filepath.Join(tempDir, "dummy.yaml")
	dummyContent := `
headers:
  X-Content-Type-Options: nosniff
  X-Frame-Options: DENY
  Content-Security-Policy: "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; script-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:; base-uri 'none'; frame-ancestors 'none'"
  Referrer-Policy: no-referrer
`

	if err := os.WriteFile(dummy, []byte(dummyContent), 0o644); err != nil {
		t.Fatalf("WriteFile(dummy) error = %v", err)
	}

	ConduitConfigPaths = []string{dummy}
	t.Cleanup(func() {
		ConduitConfigPaths = oldPaths
	})

	paths := []string{dummy}
	cfg, err := Load(paths)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	expected := defaultTestConfig()
	expected.Headers = map[string]string{}
	expected.Headers["X-Content-Type-Options"] = "nosniff"
	expected.Headers["X-Frame-Options"] = "DENY"
	expected.Headers["Content-Security-Policy"] = "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; script-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:; base-uri 'none'; frame-ancestors 'none'"
	expected.Headers["Referrer-Policy"] = "no-referrer"

	// Test that Headers is set correctly from the config file.
	if equal, err := compare(*cfg, *expected); !equal {
		t.Fatalf("Config does not match expected defaults: %v", err)
	}

	// Test that changing Headers causes the config to not match.
	expected.Headers["X-Frame-Options"] = "dummy"
	if equal, err := compare(*cfg, *expected); equal {
		t.Fatal("Config should not match when expected.Headers[\"X-Frame-Options\"] is changed")
	} else {
		t.Logf("Config correctly does not match when expected.Headers[\"X-Frame-Options\"] is changed: %v", err)
	}
}

func TestLoadFileWithAllowOrigins(t *testing.T) {
	oldPaths := append([]string(nil), ConduitConfigPaths...)
	tempDir := t.TempDir()

	dummy := filepath.Join(tempDir, "dummy.yaml")
	dummyContent := `
allowed_origins:
  - http://localhost:8080
  - https://myapp.example.com
`

	if err := os.WriteFile(dummy, []byte(dummyContent), 0o644); err != nil {
		t.Fatalf("WriteFile(dummy) error = %v", err)
	}

	ConduitConfigPaths = []string{dummy}
	t.Cleanup(func() {
		ConduitConfigPaths = oldPaths
	})

	paths := []string{dummy}
	cfg, err := Load(paths)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Test that AllowedOrigins is set correctly from the config file.
	expected := defaultTestConfig()
	expected.AllowedOrigins = []string{"http://localhost:8080", "https://myapp.example.com"}
	if equal, err := compare(*cfg, *expected); !equal {
		t.Fatalf("Config does not match expected defaults: %v", err)
	}

	// Test that changing AllowedOrigins causes the config to not match.
	expected.AllowedOrigins = []string{"http://localhost:8080"}
	if equal, err := compare(*cfg, *expected); equal {
		t.Fatal("Config should not match when expected.AllowedOrigins is changed")
	} else {
		t.Logf("Config correctly does not match when expected.AllowedOrigins is changed: %v", err)
	}
}
