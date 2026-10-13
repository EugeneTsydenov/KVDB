package storage

import (
	"errors"
	"fmt"
	"testing"
)

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore() // one store shared by every parallel subtest

	const workers = 16
	const iterations = 200

	for worker := 0; worker < workers; worker++ {
		worker := worker
		t.Run(fmt.Sprintf("worker_%d", worker), func(t *testing.T) {
			t.Parallel()
			ownKey := fmt.Sprintf("worker_key_%d", worker)

			for i := 0; i < iterations; i++ {
				store.Set(ownKey, NewStringValue([]byte("value")))
				checkValue(t, store, ownKey, "value")

				if store.SetNX(ownKey, NewStringValue([]byte("other"))) {
					t.Fatal("SetNX overwrote an existing key")
				}
				if !store.Del(ownKey) {
					t.Fatal("Del failed on an existing key")
				}
				if !store.SetNX(ownKey, NewStringValue([]byte("new"))) {
					t.Fatal("SetNX failed on a deleted key")
				}

				// Also exercise writes and reads of the SAME key across workers.
				// Its state is nondeterministic, so Get may legitimately miss.
				switch i % 4 {
				case 0:
					store.Set("shared", NewStringValue([]byte("value")))
				case 1:
					_, err := store.Get("shared")
					if err != nil && !errors.Is(err, ErrEntryNotFound) {
						t.Errorf("Get(shared): unexpected error: %v", err)
					}
				case 2:
					store.SetNX("shared", NewStringValue([]byte("value")))
				case 3:
					store.Del("shared")
				}
			}
		})
	}
}
