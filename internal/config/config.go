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

// APIConfig holds the connection parameters for the API resolver.
type APIConfig struct {
	URL             string        `yaml:"url"`
	ConnectTimeout  time.Duration `yaml:"connect_timeout"`
	ResponseTimeout time.Duration `yaml:"response_timeout"`
}

// SSHConfig holds session-level parameters for SSH connections.
type SSHConfig struct {
	Port              string            `yaml:"port"`
	Term              string            `yaml:"term"`
	IdleTimeout       time.Duration     `yaml:"idle_timeout"`
	KeepaliveInterval time.Duration     `yaml:"keepalive_interval"`
	DialTimeout       time.Duration     `yaml:"dial_timeout"`
	VerifyHostKey     bool              `yaml:"verify_host_key"`
	AutoAcceptHostKey bool              `yaml:"auto_accept_host_key"`
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
	DebugBanner     bool              `yaml:"debug_banner"`
	Resolver        ResolverType      `yaml:"resolver"`
	Port            int               `yaml:"port"`
	Demo            bool              `yaml:"demo"`
	AllowLocalShell bool              `yaml:"allow_local_shell"`
	AllowedOrigins  []string          `yaml:"allowed_origins"`
	Headers         map[string]string `yaml:"headers"`
	TerminalOptions map[string]any    `yaml:"terminal_options"`
	Local           LocalShellConfig  `yaml:"local"`
	API             APIConfig         `yaml:"api"`
	SSH             SSHConfig         `yaml:"ssh"`
}

type fileReader interface {
	ReadFile(path string) ([]byte, error)
}

type yamlDecoder interface {
	Unmarshal(data []byte, out any) error
}

type yamlDecoderFunc func([]byte, any) error

func (f yamlDecoderFunc) Unmarshal(data []byte, out any) error {
	return f(data, out)
}

type osFileReader struct{}

func (osFileReader) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func defaultConfig() *Config {
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

func load(paths []string, read fileReader, decode yamlDecoder) (*Config, error) {
	cfg := defaultConfig()

	for _, path := range paths {
		data, err := read.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("config: reading %s: %w", path, err)
		}
		if err := decode.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config: parsing %s: %w", path, err)
		}
		log.Printf("config: loaded from %s", path)
	}

	return cfg, nil
}

// Load reads conduit.yaml from the standard paths and returns the merged config.
// Missing files are silently skipped. Returns a default config if none are found.
func Load(paths []string) (*Config, error) {
	return load(paths, osFileReader{}, yamlDecoderFunc(yaml.Unmarshal))
}
