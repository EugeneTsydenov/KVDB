package storage

import (
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"
)

var (
	ErrEntryNotFound = errors.New("entry not found")
	ErrTTLInvalid    = errors.New("ttl invalid")
	ErrKeyNoTTL      = errors.New("key has no expiration")
	ErrInvlidPattern = errors.New("invalid pattern")
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

func (s *MemoryStore) Keys(pattern string) ([]string, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%w, %w", ErrInvlidPattern, err)
	}

	now := time.Now()

	s.mu.RLock()
	keysSnapshot := make([]string, 0, len(s.entries))
	for k, entry := range s.entries {
		if !entry.IsExpired(now) {
			keysSnapshot = append(keysSnapshot, k)
		}
	}
	s.mu.RUnlock()

	keys := make([]string, 0)
	for _, k := range keysSnapshot {
		if regex.MatchString(k) {
			keys = append(keys, k)
		}
	}

	return keys, nil
}

// Del removes a key.
func (s *MemoryStore) Del(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok {
		return false
	}

	if entry.IsExpired(time.Now()) {
		delete(s.entries, key)

		return false
	}

	delete(s.entries, key)

	return true
}

func (s *MemoryStore) DeleteExpiredBatch(sampleSize int) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	deleted := 0
	cnt := 0

	for k, v := range s.entries {
		if cnt >= sampleSize {
			break
		}
		cnt++

		if v.IsExpired(now) {
			delete(s.entries, k)
			deleted++
		}
	}

	return deleted
}
