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

type localEntry struct {
	Shell    string `yaml:"shell"`
	Username string `yaml:"username"`
}

type hostsFile struct {
	Local *localEntry          `yaml:"local"`
	Hosts map[string]hostEntry `yaml:"hosts"`
}

type Resolver struct {
	local *localEntry
	hosts map[string]hostEntry
}

func New() (*Resolver, error) {
	return NewFromPaths(configPaths)
}

func NewFromPaths(paths []string) (*Resolver, error) {
	hosts := make(map[string]hostEntry)
	var local *localEntry
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
		if f.Local != nil {
			local = f.Local
		}
		log.Printf("fileresolver: loaded %d hosts from %s", len(f.Hosts), path)
	}
	if len(hosts) == 0 && local == nil {
		log.Printf("fileresolver: warning: no config files found (looked in %v)", paths)
	}
	return &Resolver{hosts: hosts, local: local}, nil
}

func (r *Resolver) Resolve(req resolver.Request) (resolver.SessionConfig, error) {
	if req.Host == config.Local {
		if r.local == nil {
			return nil, fmt.Errorf("local session not configured")
		}
		username := req.User
		if username == "" {
			username = r.local.Username
		}
		shell := r.local.Shell
		if shell == "" {
			shell = "/bin/bash"
		}
		return resolver.LocalConfig{
			Shell:    shell,
			Username: username,
		}, nil
	}
	entry, ok := r.hosts[req.Host]
	if !ok {
		return nil, fmt.Errorf("host %q not found", req.Host)
	}
	username := req.User
	if username == "" {
		username = entry.Username
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
