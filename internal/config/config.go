// Package config loads the main Conduit configuration from conduit.yaml.
// Files are read from /etc/conduit/conduit.yaml and ./conduit.yaml in order;
// the last file found wins on duplicate keys.
package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

type ResolverType string

const (
	ResolverFile ResolverType = "file"
	ResolverAPI  ResolverType = "api"
)

// APIConfig holds the connection parameters for the API resolver.
type APIConfig struct {
	URL             string        `yaml:"url"`
	ConnectTimeout  time.Duration `yaml:"connect_timeout"`
	ResponseTimeout time.Duration `yaml:"response_timeout"`
}

// SSHConfig holds session-level parameters for SSH connections.
type SSHConfig struct {
	Term              string            `yaml:"term"` // terminal type for SSH sessions (default xterm-256color)
	IdleTimeout       time.Duration     `yaml:"idle_timeout"`
	KeepaliveInterval time.Duration     `yaml:"keepalive_interval"`
	DialTimeout       time.Duration     `yaml:"dial_timeout"`
	VerifyHostKey     bool              `yaml:"verify_host_key"`  // enable TOFU host key verification for all SSH hosts
	TOFUAutoAccept    bool              `yaml:"tofu_auto_accept"` // skip the interactive prompt and auto-accept unknown fingerprints
	KnownHostsFile    string            `yaml:"known_hosts_file"` // path to the TOFU known-hosts YAML file
	Env               map[string]string `yaml:"env"`
}

// LocalShellConfig holds shell parameters for local sessions.
type LocalShellConfig struct {
	Command     string            `yaml:"command"`
	Term        string            `yaml:"term"`        // terminal type for local sessions (default xterm-256color)
	WorkingDir  string            `yaml:"working_dir"` // working directory for local sessions; empty = inherit conduit's cwd
	IdleTimeout time.Duration     `yaml:"idle_timeout"`
	Env         map[string]string `yaml:"env"`
}

// Config is the top-level Conduit configuration.
type Config struct {
	Debug           bool             `yaml:"debug"`             // show debug banner and session details in the terminal
	Resolver        ResolverType     `yaml:"resolver"`          // "file" (default) or "api"
	Port            int              `yaml:"port"`              // HTTP listen port (default 8080)
	Demo            bool             `yaml:"demo"`              // enable the demo page (default true)
	AllowLocalShell bool             `yaml:"allow_local_shell"` // enable local shell sessions
	AllowedOrigins  []string         `yaml:"allowed_origins"`   // WebSocket origin allowlist; empty = allow all
	Local           LocalShellConfig `yaml:"local"`             // local shell session config
	API             APIConfig        `yaml:"api"`
	SSH             SSHConfig        `yaml:"ssh"`
}

// Load reads conduit.yaml from the standard paths and returns the merged config.
// Missing files are silently skipped. Returns a default config if none are found.
func Load() (*Config, error) {
	cfg := &Config{
		Debug:           false,
		Resolver:        ResolverFile,
		Port:            8080,
		Demo:            true,
		AllowLocalShell: true,
		Local: LocalShellConfig{
			Command:     "/bin/bash",
			Term:        "xterm-256color",
			IdleTimeout: 10 * time.Minute,
		},
		API: APIConfig{
			ConnectTimeout:  5 * time.Second,
			ResponseTimeout: 10 * time.Second,
		},
		SSH: SSHConfig{
			Term:              "xterm-256color",
			IdleTimeout:       10 * time.Minute,
			KeepaliveInterval: 30 * time.Second,
			DialTimeout:       10 * time.Second,
			VerifyHostKey:     true,
			TOFUAutoAccept:    false,
			KnownHostsFile:    "./known_hosts.yaml",
		},
	}
	for _, path := range ConduitConfigPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("config: reading %s: %w", path, err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config: parsing %s: %w", path, err)
		}
		log.Printf("config: loaded from %s", path)
	}
	return cfg, nil
}
