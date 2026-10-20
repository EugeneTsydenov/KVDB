package storage

import (
	"errors"
	"testing"
	"time"
)

func TestMemoryStore_SetEX(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()

	ttl := time.Minute

	if err := store.SetEX(
		"key",
		NewStringValue([]byte("first")),
		ttl,
	); err != nil {
		t.Fatalf("SetEX: unexpected error: %v", err)
	}

	checkValue(t, store, "key", "first")

	if err := store.SetEX(
		"key",
		NewStringValue([]byte("second")),
		ttl,
	); err != nil {
		t.Fatalf("SetEX overwrite: unexpected error: %v", err)
	}

	checkValue(t, store, "key", "second")

	gotTTL, err := store.TTL("key")
	if err != nil {
		t.Fatalf("TTL: unexpected error: %v", err)
	}

	if gotTTL <= 0 || gotTTL > ttl {
		t.Fatalf("TTL = %v, want (0, %v]", gotTTL, ttl)
	}
}

func TestMemoryStore_SetEX_InvalidTTL(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	store.Set("key", NewStringValue([]byte("original")))

	invalidTTLs := []time.Duration{
		0,
		-time.Second,
	}

	for _, ttl := range invalidTTLs {
		if err := store.SetEX(
			"key",
			NewStringValue([]byte("replacement")),
			ttl,
		); !errors.Is(err, ErrTTLInvalid) {
			t.Errorf(
				"SetEX with ttl %v: got error %v, want ErrTTLInvalid",
				ttl,
				err,
			)
		}

		checkValue(t, store, "key", "original")
	}
}

func TestMemoryStore_Expire(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()

	store.Set("key", NewStringValue([]byte("value")))

	ttl := time.Minute

	if err := store.Expire("key", ttl); err != nil {
		t.Fatalf("Expire: unexpected error: %v", err)
	}

	gotTTL, err := store.TTL("key")
	if err != nil {
		t.Fatalf("TTL after Expire: unexpected error: %v", err)
	}

	if gotTTL <= 0 || gotTTL > ttl {
		t.Fatalf("TTL = %v, want (0, %v]", gotTTL, ttl)
	}

	if err := store.Expire("key", 0); !errors.Is(err, ErrTTLInvalid) {
		t.Errorf(
			"Expire with zero TTL: got %v, want ErrTTLInvalid",
			err,
		)
	}

	if err := store.Expire("missing", ttl); !errors.Is(err, ErrEntryNotFound) {
		t.Errorf(
			"Expire on missing key: got %v, want ErrEntryNotFound",
			err,
		)
	}

	store.entries["expired"] = Entry{
		Value:     NewStringValue([]byte("old")),
		ExpiresAt: time.Now().Add(-time.Second),
	}

	if err := store.Expire("expired", ttl); !errors.Is(err, ErrEntryNotFound) {
		t.Errorf(
			"Expire on expired key: got %v, want ErrEntryNotFound",
			err,
		)
	}

	if _, exists := store.entries["expired"]; exists {
		t.Error("Expire: expired entry was not removed")
	}
}

func TestMemoryStore_TTL(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()

	if _, err := store.TTL("missing"); !errors.Is(err, ErrEntryNotFound) {
		t.Errorf(
			"TTL on missing key: got %v, want ErrEntryNotFound",
			err,
		)
	}

	store.Set("permanent", NewStringValue([]byte("value")))

	if _, err := store.TTL("permanent"); !errors.Is(err, ErrKeyNoTTL) {
		t.Errorf(
			"TTL on permanent key: got %v, want ErrKeyNoTTL",
			err,
		)
	}

	ttl := time.Minute

	if err := store.SetEX(
		"expiring",
		NewStringValue([]byte("value")),
		ttl,
	); err != nil {
		t.Fatalf("SetEX: unexpected error: %v", err)
	}

	gotTTL, err := store.TTL("expiring")
	if err != nil {
		t.Fatalf("TTL on expiring key: unexpected error: %v", err)
	}

	if gotTTL <= 0 || gotTTL > ttl {
		t.Fatalf("TTL = %v, want (0, %v]", gotTTL, ttl)
	}

	store.entries["expired"] = Entry{
		Value:     NewStringValue([]byte("old")),
		ExpiresAt: time.Now().Add(-time.Second),
	}

	if _, err := store.TTL("expired"); !errors.Is(err, ErrEntryNotFound) {
		t.Errorf(
			"TTL on expired key: got %v, want ErrEntryNotFound",
			err,
		)
	}

	if _, exists := store.entries["expired"]; exists {
		t.Error("TTL: expired entry was not removed")
	}
}

func TestMemoryStore_LazyCleanup(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()

	store.entries["expired"] = Entry{
		Value:     NewStringValue([]byte("old")),
		ExpiresAt: time.Now().Add(-time.Second),
	}

	if _, err := store.Get("expired"); !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf(
			"Get on expired key: got %v, want ErrEntryNotFound",
			err,
		)
	}

	if _, exists := store.entries["expired"]; exists {
		t.Error("Get: expired entry was not removed")
	}
}

func TestMemoryStore_DeleteExpiredBatch(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()

	store.entries["expired1"] = Entry{
		Value:     NewStringValue([]byte("old1")),
		ExpiresAt: time.Now().Add(-time.Second),
	}

	store.entries["expired2"] = Entry{
		Value:     NewStringValue([]byte("old2")),
		ExpiresAt: time.Now().Add(-time.Second),
	}

	if err := store.SetEX(
		"valid",
		NewStringValue([]byte("value")),
		time.Hour,
	); err != nil {
		t.Fatalf("SetEX: unexpected error: %v", err)
	}

	deleted := store.DeleteExpiredBatch(100)

	if deleted != 2 {
		t.Fatalf("DeleteExpiredBatch = %d, want 2", deleted)
	}

	if _, exists := store.entries["expired1"]; exists {
		t.Error("expired1 was not deleted")
	}

	if _, exists := store.entries["expired2"]; exists {
		t.Error("expired2 was not deleted")
	}

	if _, exists := store.entries["valid"]; !exists {
		t.Error("valid entry was unexpectedly deleted")
	}

	if deleted := store.DeleteExpiredBatch(100); deleted != 0 {
		t.Errorf("second DeleteExpiredBatch = %d, want 0", deleted)
	}
}
