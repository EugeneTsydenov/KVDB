package storage

import "sync"

type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]Entry
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entries: make(map[string]Entry),
	}
}
