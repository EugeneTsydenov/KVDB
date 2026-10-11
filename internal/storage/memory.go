package storage

import (
	"errors"
	"sync"
	"time"
)

var _ Storage = (*MemoryStore)(nil)

var ErrEntryNotFound = errors.New("entry not found")

// memoryStore is an in-memory implementation of the Storage interface.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]Entry
}

// NewMemoryStore creates a new thread-safe in-memory key-value store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entries: make(map[string]Entry),
	}
}

// Set stores a value.
func (s *MemoryStore) Set(key string, v Value) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = Entry{
		Value: v,
	}
}

// SetNX stores a value if it doesn't exist or has expired.
func (s *MemoryStore) SetNX(key string, v Value) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, ok := s.entries[key]; ok && !entry.IsExpired(time.Now()) {
		return false
	}

	s.entries[key] = Entry{
		Value: v,
	}

	return true
}

// Get retrieves a value and cleans up expired entries lazily.
func (s *MemoryStore) Get(key string) (Value, error) {
	s.mu.RLock()
	entry, ok := s.entries[key]
	now := time.Now()

	if !ok {
		s.mu.RUnlock()

		return Value{}, ErrEntryNotFound
	}

	if entry.IsExpired(now) {
		s.mu.RUnlock()
		s.mu.Lock()
		if e, exists := s.entries[key]; exists && e.IsExpired(now) {
			delete(s.entries, key)
		}
		s.mu.Unlock()

		return Value{}, ErrEntryNotFound
	}

	s.mu.RUnlock()

	return entry.Value, nil
}

// Del removes a key.
func (s *MemoryStore) Del(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.entries[key]; !ok {
		return false
	}

	delete(s.entries, key)

	return true
}
