package storage

import (
	"errors"
	"reflect"
	"sort"
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

func assertKeysEqual(t *testing.T, got, want []string) {
	t.Helper()

	got = append([]string(nil), got...)
	want = append([]string(nil), want...)

	sort.Strings(got)
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Keys returned %v, want %v", got, want)
	}
}

func setPermanentKeyForKeysTest(
	t *testing.T,
	store *MemoryStore,
	key string,
) {
	t.Helper()

	if err := store.Set(
		key,
		NewStringValue([]byte("value")),
		0,
	); err != nil {
		t.Fatalf("Set(%q): unexpected error: %v", key, err)
	}
}

func TestMemoryStore_Keys_MatchesPattern(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()

	for _, key := range []string{
		"user:1",
		"user:2",
		"user:abc",
		"prefix-user:3",
		"session:1",
	} {
		setPermanentKeyForKeysTest(t, store, key)
	}

	store.entries["user:3"] = Entry{
		Value:     NewStringValue([]byte("expired")),
		ExpiresAt: time.Now().Add(-time.Second),
	}

	got, err := store.Keys(`^user:[0-9]+$`)
	if err != nil {
		t.Fatalf("Keys: unexpected error: %v", err)
	}

	assertKeysEqual(t, got, []string{
		"user:1",
		"user:2",
	})
}

func TestMemoryStore_Keys_EmptyPattern(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()

	setPermanentKeyForKeysTest(t, store, "")
	setPermanentKeyForKeysTest(t, store, "alpha")
	setPermanentKeyForKeysTest(t, store, "beta")

	store.entries["expired"] = Entry{
		Value:     NewStringValue([]byte("old")),
		ExpiresAt: time.Now().Add(-time.Second),
	}

	got, err := store.Keys("")
	if err != nil {
		t.Fatalf("Keys with empty pattern: unexpected error: %v", err)
	}

	assertKeysEqual(t, got, []string{
		"",
		"alpha",
		"beta",
	})
}

func TestMemoryStore_Keys_InvalidPattern(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	setPermanentKeyForKeysTest(t, store, "alpha")

	got, err := store.Keys("[")
	if !errors.Is(err, ErrInvalidPattern) {
		t.Fatalf(
			"Keys with invalid pattern: got error %v, want ErrInvalidPattern",
			err,
		)
	}

	if len(got) != 0 {
		t.Errorf("Keys with invalid pattern returned %v, want no keys", got)
	}
}

func TestMemoryStore_Keys_NoMatches(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	setPermanentKeyForKeysTest(t, store, "user:1")
	setPermanentKeyForKeysTest(t, store, "session:1")

	got, err := store.Keys(`^missing:[0-9]+$`)
	if err != nil {
		t.Fatalf("Keys: unexpected error: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("Keys returned %v, want no matches", got)
	}
}

func TestMemoryStore_Keys_EmptyStore(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()

	got, err := store.Keys(".*")
	if err != nil {
		t.Fatalf("Keys on empty store: unexpected error: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("Keys on empty store returned %v, want no keys", got)
	}
}
