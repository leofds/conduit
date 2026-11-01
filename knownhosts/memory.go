package knownhosts

import "sync"

// Memory is an in-process Provider with no persistence. It is the default used
// when a Server is created without an explicit KnownHosts provider. TOFU
// fingerprints live only for the lifetime of the process.
type Memory struct {
	mu    sync.RWMutex
	hosts map[string]string
}

// NewMemory returns an empty in-memory provider.
func NewMemory() *Memory {
	return &Memory{hosts: make(map[string]string)}
}

// Get returns the stored fingerprint for host, or "" if unknown.
func (m *Memory) Get(host string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hosts[host]
}

// Set stores the fingerprint for host in memory.
func (m *Memory) Set(host, fingerprint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hosts[host] = fingerprint
	return nil
}

// Remove deletes the fingerprint for host from memory. It is not an error if
// the host is unknown.
func (m *Memory) Remove(host string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.hosts, host)
	return nil
}
