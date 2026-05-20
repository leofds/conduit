// Package fileresolver provides a file-based Resolver implementation.
// It reads SSH host configurations from YAML files, merging them in order.
package fileresolver

import (
	"fmt"
	"log"
	"os"

	"github.com/goccy/go-yaml"

	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/resolver"
)

var configPaths = []string{
	"/etc/conduit/hosts.yaml",
	"/etc/conduit/hosts.yml",
	"./hosts.yaml",
	"./hosts.yml",
}

type hostEntry struct {
	Address  string `yaml:"address"`
	Port     string `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type hostsFile struct {
	Hosts map[string]hostEntry `yaml:"hosts"`
}

type Resolver struct {
	local config.LocalShellConfig
	hosts map[string]hostEntry
}

func New(local config.LocalShellConfig) (*Resolver, error) {
	return NewFromPaths(configPaths, local)
}

func NewFromPaths(paths []string, local config.LocalShellConfig) (*Resolver, error) {
	hosts := make(map[string]hostEntry)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("fileresolver: reading %s: %w", path, err)
		}
		var f hostsFile
		if err := yaml.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("fileresolver: parsing %s: %w", path, err)
		}
		for k, v := range f.Hosts {
			hosts[k] = v
		}
		log.Printf("fileresolver: loaded %d hosts from %s", len(f.Hosts), path)
	}
	if len(hosts) == 0 {
		log.Printf("fileresolver: warning: no config files found (looked in %v)", paths)
	}
	return &Resolver{hosts: hosts, local: local}, nil
}

func (r *Resolver) Resolve(req resolver.Request) (resolver.SessionConfig, error) {
	if req.Host == config.Local {
		command := r.local.Command
		return resolver.LocalConfig{
			Command: command,
		}, nil
	}
	entry, ok := r.hosts[req.Host]
	if !ok {
		return nil, fmt.Errorf("host %q not found", req.Host)
	}
	username := entry.Username
	if username == "" {
		return nil, fmt.Errorf("username not found for host %q", req.Host)
	}
	port := entry.Port
	if port == "" {
		port = "22"
	}
	return resolver.SSHConfig{
		Address:  entry.Address,
		Port:     port,
		Username: username,
		Password: entry.Password,
	}, nil
}
