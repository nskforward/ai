package session

import (
	"sync"
	"time"

	"github.com/nskforward/ai/llm"
)

type entry struct {
	history    []llm.Message
	lastAccess time.Time
}

// MemoryStore is an in-memory session store with TTL-based cleanup.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*entry
	ttl      time.Duration
}

// NewMemoryStore creates an in-memory store. Sessions expire after ttl of inactivity.
func NewMemoryStore(ttl time.Duration) *MemoryStore {
	ms := &MemoryStore{
		sessions: make(map[string]*entry),
		ttl:      ttl,
	}
	go ms.cleanup()
	return ms
}

func (m *MemoryStore) Load(sessionID string) ([]llm.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	e, ok := m.sessions[sessionID]
	if !ok {
		return nil, nil
	}
	// Return a copy to avoid race conditions
	cp := make([]llm.Message, len(e.history))
	copy(cp, e.history)
	return cp, nil
}

func (m *MemoryStore) Save(sessionID string, history []llm.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := make([]llm.Message, len(history))
	copy(cp, history)
	m.sessions[sessionID] = &entry{
		history:    cp,
		lastAccess: time.Now(),
	}
	return nil
}

func (m *MemoryStore) cleanup() {
	ticker := time.NewTicker(m.ttl / 2)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for id, e := range m.sessions {
			if now.Sub(e.lastAccess) > m.ttl {
				delete(m.sessions, id)
			}
		}
		m.mu.Unlock()
	}
}
