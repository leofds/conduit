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
	Port			  string            `yaml:"port"`
	Term              string            `yaml:"term"`
	IdleTimeout       time.Duration     `yaml:"idle_timeout"`
	KeepaliveInterval time.Duration     `yaml:"keepalive_interval"`
	DialTimeout       time.Duration     `yaml:"dial_timeout"`
	VerifyHostKey     bool              `yaml:"verify_host_key"`
	TOFUAutoAccept    bool              `yaml:"tofu_auto_accept"`
	KnownHostsFile    string            `yaml:"known_hosts_file"`
	Env               map[string]string `yaml:"env"`
}

// LocalShellConfig holds shell parameters for local sessions.
type LocalShellConfig struct {
	Command     string            `yaml:"command"`
	Term        string            `yaml:"term"`
	WorkingDir  string            `yaml:"working_dir"`
	IdleTimeout time.Duration     `yaml:"idle_timeout"`
	Env         map[string]string `yaml:"env"`
}

// Config is the top-level Conduit configuration.
type Config struct {
	DebugBanner     bool             `yaml:"debug_banner"`
	Resolver        ResolverType     `yaml:"resolver"`
	Port            int              `yaml:"port"`
	Demo            bool             `yaml:"demo"`
	AllowLocalShell bool             `yaml:"allow_local_shell"`
	AllowedOrigins  []string         `yaml:"allowed_origins"`
	Local           LocalShellConfig `yaml:"local"`
	API             APIConfig        `yaml:"api"`
	SSH             SSHConfig        `yaml:"ssh"`
}

// Load reads conduit.yaml from the standard paths and returns the merged config.
// Missing files are silently skipped. Returns a default config if none are found.
func Load() (*Config, error) {
	cfg := &Config{
		DebugBanner:     false,
		Resolver:        ResolverFile,
		Port:            8080,
		Demo:            true,
		AllowLocalShell: true,
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
			TOFUAutoAccept:    false,
			KnownHostsFile:    "./known_hosts.yaml",
		},
		API: APIConfig{
			ConnectTimeout:  5 * time.Second,
			ResponseTimeout: 10 * time.Second,
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
