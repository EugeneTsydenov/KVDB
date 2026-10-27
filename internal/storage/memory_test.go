package storage

import (
	"errors"
	"testing"
	"time"
)

func checkValue(t *testing.T, store *MemoryStore, key, want string) {
	t.Helper()
	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get(%q): unexpected error: %v", key, err)
	}
	if got.Type != TypeString || string(got.Data) != want {
		t.Fatalf("Get(%q) = {Type: %v, Data: %q}, want TypeString and %q", key, got.Type, got.Data, want)
	}
}

func TestMemoryStore_Set(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()

	store.Set("key", NewStringValue([]byte("first")), 0)
	checkValue(t, store, "key", "first")

	store.Set("key", NewStringValue([]byte("second")), 0)
	checkValue(t, store, "key", "second")
}

func TestMemoryStore_Get(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()
	store.Set("exists", NewStringValue([]byte("value")), 0)
	checkValue(t, store, "exists", "value")

	if _, err := store.Get("missing"); !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("Get(missing): got error %v, want ErrEntryNotFound", err)
	}

	store.entries["expired"] = Entry{
		Value:     NewStringValue([]byte("old")),
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	if _, err := store.Get("expired"); !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("Get(expired): got error %v, want ErrEntryNotFound", err)
	}
	if _, exists := store.entries["expired"]; exists {
		t.Fatal("Get(expired): expired entry was not removed")
	}
}

func TestMemoryStore_SetNX(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()

	if ok, err := store.SetNX("key", NewStringValue([]byte("first")), 0); err != nil || !ok {
		t.Fatal("SetNX on missing key returned false")
	}
	if ok, err := store.SetNX("key", NewStringValue([]byte("second")), 0); err != nil || ok {
		t.Fatal("SetNX on existing key returned true")
	}
	checkValue(t, store, "key", "first")

	store.entries["expired"] = Entry{
		Value:     NewStringValue([]byte("old")),
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	if ok, err := store.SetNX("expired", NewStringValue([]byte("new")), 0); err != nil || !ok {
		t.Fatal("SetNX on expired key returned false")
	}
	checkValue(t, store, "expired", "new")
}

func TestMemoryStore_Del(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()
	store.Set("key", NewStringValue([]byte("value")), 0)

	if ok := store.Del("key"); !ok {
		t.Fatal("Del on existing key returned false")
	}
	if _, err := store.Get("key"); !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("Get after Del: got error %v, want ErrEntryNotFound", err)
	}
	if ok := store.Del("key"); ok {
		t.Fatal("Del on missing key returned true")
	}
}
