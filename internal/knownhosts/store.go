// Package knownhosts provides a thread-safe persistent store for SSH host key
// fingerprints, implementing Trust on First Use (TOFU) semantics.
package knownhosts

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
)

type file struct {
	Hosts map[string]string `yaml:"hosts"`
}

// Store is a thread-safe persistent map of logical host name → SHA256 fingerprint.
type Store struct {
	path    string
	mu      sync.RWMutex
	hosts   map[string]string
	lastMod time.Time // mtime of the file at the last successful load
}

// New loads the known-hosts file at path.
// Returns an empty store (without error) if the file does not yet exist.
func New(path string) (*Store, error) {
	s := &Store{path: path, hosts: make(map[string]string)}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// load reads and parses the file, updating hosts and lastMod.
// Must be called with s.mu held for writing, or before the Store is shared.
func (s *Store) load() error {
	info, err := os.Stat(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("knownhosts: stat %s: %w", s.path, err)
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("knownhosts: reading %s: %w", s.path, err)
	}
	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("knownhosts: parsing %s: %w", s.path, err)
	}
	if f.Hosts != nil {
		s.hosts = f.Hosts
	} else {
		s.hosts = make(map[string]string)
	}
	s.lastMod = info.ModTime()
	log.Printf("knownhosts: loaded %d entries from %s", len(s.hosts), s.path)
	return nil
}

// reloadIfChanged re-reads the file when its mtime differs from the last load.
// Must be called with s.mu held for writing.
func (s *Store) reloadIfChanged() {
	info, err := os.Stat(s.path)
	if err != nil || !info.ModTime().After(s.lastMod) {
		return
	}
	if err := s.load(); err != nil {
		log.Printf("knownhosts: reload failed: %v", err)
	}
}

// Get returns the stored fingerprint for host, or an empty string if not yet known.
// The file is transparently reloaded if it has been modified since the last read.
func (s *Store) Get(host string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChanged()
	return s.hosts[host]
}

// Set stores the fingerprint for host and persists the file atomically.
func (s *Store) Set(host, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hosts[host] = fingerprint
	data, err := yaml.Marshal(file{Hosts: s.hosts})
	if err != nil {
		return fmt.Errorf("knownhosts: marshal: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("knownhosts: writing %s: %w", s.path, err)
	}
	// Update lastMod so the next Get does not trigger an unnecessary reload.
	if info, err := os.Stat(s.path); err == nil {
		s.lastMod = info.ModTime()
	}
	return nil
}

// Remove deletes the stored fingerprint for host and persists the file.
// It is not an error if the host is not known.
func (s *Store) Remove(host string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.hosts, host)
	data, err := yaml.Marshal(file{Hosts: s.hosts})
	if err != nil {
		return fmt.Errorf("knownhosts: marshal: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("knownhosts: writing %s: %w", s.path, err)
	}
	if info, err := os.Stat(s.path); err == nil {
		s.lastMod = info.ModTime()
	}
	return nil
}

