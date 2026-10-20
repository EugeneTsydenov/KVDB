package storage

import (
	"errors"
	"sync"
	"time"
)

var _ Storage = (*MemoryStore)(nil)

var (
	ErrEntryNotFound = errors.New("entry not found")
	ErrTTLInvalid    = errors.New("ttl invalid")
	ErrKeyNoTTL      = errors.New("key has no expiration")
)

// MemoryStore is an in-memory implementation of the Storage interface.
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
		Value: v.Clone(),
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
		Value: v.Clone(),
	}

	return true
}

// Set stores a value with ttl.
func (s *MemoryStore) SetEX(key string, v Value, ttl time.Duration) error {
	if ttl <= 0 {
		return ErrTTLInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = Entry{
		Value:     v.Clone(),
		ExpiresAt: time.Now().Add(ttl),
	}

	return nil
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

	if !entry.IsExpired(now) {
		val := entry.Value.Clone()
		s.mu.RUnlock()

		return val, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, exists := s.entries[key]; exists && entry.IsExpired(now) {
		delete(s.entries, key)
	}

	return Value{}, ErrEntryNotFound
}

// TTL returns the remaining ttl for a key.
func (s *MemoryStore) TTL(key string) (time.Duration, error) {
	s.mu.RLock()
	entry, ok := s.entries[key]
	now := time.Now()

	if !ok {
		s.mu.RUnlock()

		return 0, ErrEntryNotFound
	}

	if !entry.IsExpired(now) {
		s.mu.RUnlock()
		if entry.ExpiresAt.IsZero() {
			return 0, ErrKeyNoTTL
		}

		return time.Until(entry.ExpiresAt), nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, exists := s.entries[key]; exists && entry.IsExpired(now) {
		delete(s.entries, key)
	}

	return 0, ErrEntryNotFound
}

// Expire sets or updates ttl for an existing key.
func (s *MemoryStore) Expire(key string, ttl time.Duration) error {
	if ttl <= 0 {
		return ErrTTLInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	now := time.Now()
	if !ok {
		return ErrEntryNotFound
	}

	if entry.IsExpired(now) {
		delete(s.entries, key)

		return ErrEntryNotFound
	}

	s.entries[key] = Entry{
		Value:     entry.Value.Clone(),
		ExpiresAt: now.Add(ttl),
	}

	return nil
}

// Del removes a key.
func (s *MemoryStore) Del(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if entry, ok := s.entries[key]; !ok && entry.IsExpired(now) {
		return false
	}

	delete(s.entries, key)

	return true
}
