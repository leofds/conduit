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

var configPaths = []string{
	"/etc/conduit/conduit.yaml",
	"/etc/conduit/conduit.yml",
	"./conduit.yaml",
	"./conduit.yml",
}

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

// LocalShellConfig holds shell parameters for local sessions.
type LocalShellConfig struct {
	Enable  bool   `yaml:"enable"`
	Command string `yaml:"command"`
	Term    string `yaml:"term"`
}

// Config is the top-level Conduit configuration.
type Config struct {
	Resolver ResolverType     `yaml:"resolver"` // "file" (default) or "api"
	Demo     bool             `yaml:"demo"`     // enable the demo page (default true)
	Local    LocalShellConfig `yaml:"local"`    // local shell session config
	API      APIConfig        `yaml:"api"`
}

// Load reads conduit.yaml from the standard paths and returns the merged config.
// Missing files are silently skipped. Returns a default config if none are found.
func Load() (*Config, error) {
	cfg := &Config{
		Resolver: ResolverFile,
		Demo:     true,
		Local: LocalShellConfig{
			Enable: true,
			Term:   "xterm-256color",
		},
		API: APIConfig{
			ConnectTimeout:  5 * time.Second,
			ResponseTimeout: 10 * time.Second,
		},
	}
	for _, path := range configPaths {
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
