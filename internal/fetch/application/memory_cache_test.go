package application

import (
	"context"
	"github.com/javleds/api-engine/internal/fetch/domain"
	"sync"
)

type memoryCache struct {
	missFirst bool
	mu        sync.Mutex
	entries   map[string]domain.CacheEntry
	gets      int
	puts      int
	err       error
}

func (m *memoryCache) Get(_ context.Context, key string) (domain.CacheEntry, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gets++
	if m.missFirst && m.gets == 1 {
		return domain.CacheEntry{}, false, nil
	}
	entry, ok := m.entries[key]
	return entry.Clone(), ok, m.err
}
func (m *memoryCache) Put(_ context.Context, entry domain.CacheEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.puts++
	if m.entries == nil {
		m.entries = make(map[string]domain.CacheEntry)
	}
	m.entries[entry.Key] = entry.Clone()
	return m.err
}
func (m *memoryCache) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
	return m.err
}
