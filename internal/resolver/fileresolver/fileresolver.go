// Package fileresolver provides a file-based Resolver implementation.
// It reads SSH host configurations from YAML files, merging them in order.
package fileresolver

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/resolver"
)

type hostEntry struct {
	Address           string            `yaml:"address"`
	Port              string            `yaml:"port"`
	Username          string            `yaml:"username"`
	Password          string            `yaml:"password"`
	PrivateKeyFile    string            `yaml:"private_key_file"`
	Term              string            `yaml:"term"` // per-host override; omit to use global default
	Env               map[string]string `yaml:"env"`
	TOFUAutoAccept    *bool             `yaml:"tofu_auto_accept"`   // per-host override; omit to use global default
	VerifyHostKey     *bool             `yaml:"verify_host_key"`    // per-host override; omit to use global default
	IdleTimeout       *time.Duration    `yaml:"idle_timeout"`       // per-host override; omit to use global default
	KeepaliveInterval *time.Duration    `yaml:"keepalive_interval"` // per-host override; omit to use global default
}

type hostsFile struct {
	Hosts map[string]hostEntry `yaml:"hosts"`
}

type Resolver struct {
	local config.LocalShellConfig
	hosts map[string]hostEntry
	paths []string
}

func New(local config.LocalShellConfig) (*Resolver, error) {
	return NewFromPaths(config.HostsConfigPaths, local)
}

func NewFromPaths(paths []string, local config.LocalShellConfig) (*Resolver, error) {
	resolver := &Resolver{
		paths: paths,
		local: local,
		hosts: make(map[string]hostEntry),
	}
	if err := resolver.loadHosts(); err != nil {
		return nil, err
	}
	return resolver, nil
}

func (r *Resolver) loadHosts() error {
	hosts := make(map[string]hostEntry)
	for _, path := range r.paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("fileresolver: reading %s: %w", path, err)
		}
		var f hostsFile
		if err := yaml.Unmarshal(data, &f); err != nil {
			return fmt.Errorf("fileresolver: parsing %s: %w", path, err)
		}
		for k, v := range f.Hosts {
			hosts[k] = v
		}
		log.Printf("fileresolver: loaded %d hosts from %s", len(f.Hosts), path)
	}
	if len(hosts) == 0 {
		log.Printf("fileresolver: warning: no config files found (looked in %v)", r.paths)
	}
	r.hosts = hosts
	return nil
}

func (r *Resolver) Reload() error {
	log.Printf("fileresolver: reloading hosts from %v", r.paths)
	return r.loadHosts()
}

func (r *Resolver) Resolve(req resolver.Request) (resolver.SessionConfig, error) {
	if req.Host == config.Local {
		var workingDir *string
		if r.local.WorkingDir != "" {
			wd := r.local.WorkingDir
			workingDir = &wd
		}
		var idleTimeout *time.Duration
		if r.local.IdleTimeout != 0 {
			d := r.local.IdleTimeout
			idleTimeout = &d
		}
		return resolver.LocalConfig{
			Command:     r.local.Command,
			Term:        r.local.Term,
			WorkingDir:  workingDir,
			IdleTimeout: idleTimeout,
			Env:         r.local.Env,
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
		Address:           entry.Address,
		Port:              port,
		Username:          username,
		Password:          entry.Password,
		PrivateKeyFile:    entry.PrivateKeyFile,
		Term:              entry.Term,
		Env:               entry.Env,
		TOFUAutoAccept:    entry.TOFUAutoAccept,
		VerifyHostKey:     entry.VerifyHostKey,
		IdleTimeout:       entry.IdleTimeout,
		KeepaliveInterval: entry.KeepaliveInterval,
	}, nil
}
