package session

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type Info struct {
	ID        string    `json:"id"`
	Host      string    `json:"host"`
	Method    string    `json:"method"`
	CreatedAt time.Time `json:"created_at"`
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*managedSession
	counter  atomic.Uint64
}

type managedSession struct {
	info   Info
	cancel context.CancelFunc
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*managedSession),
	}
}

// Register adds a new session to tracking. Returns a context derived from parent
// that will be cancelled when the session is closed via the manager, and a release
// function that the caller MUST call when the session ends naturally (to unregister it).
func (m *Manager) Register(parent context.Context, host, method string) (context.Context, string, func()) {
	id := fmt.Sprintf("%s-%d", host, m.counter.Add(1))
	ctx, cancel := context.WithCancel(parent)

	ms := &managedSession{
		info: Info{
			ID:        id,
			Host:      host,
			Method:    method,
			CreatedAt: time.Now(),
		},
		cancel: cancel,
	}

	m.mu.Lock()
	m.sessions[id] = ms
	m.mu.Unlock()

	log.Printf("session registered id=%s host=%s method=%s", id, host, method)

	release := func() {
		m.mu.Lock()
		if _, ok := m.sessions[id]; ok {
			delete(m.sessions, id)
			log.Printf("session unregistered id=%s", id)
		}
		m.mu.Unlock()
	}

	return ctx, id, release
}

// Cancel terminates a session by its ID. Returns true if the session was found.
func (m *Manager) Cancel(id string) bool {
	m.mu.RLock()
	ms, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	log.Printf("cancelling session id=%s", id)
	ms.cancel()
	return true
}

// CancelAll terminates all active sessions. Returns the number of sessions cancelled.
func (m *Manager) CancelAll() int {
	m.mu.RLock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		m.Cancel(id)
	}
	return len(ids)
}

// List returns information about all active sessions.
func (m *Manager) List() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Info, 0, len(m.sessions))
	for _, ms := range m.sessions {
		info := ms.info
		result = append(result, info)
	}
	return result
}

// Count returns the number of active sessions.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}
