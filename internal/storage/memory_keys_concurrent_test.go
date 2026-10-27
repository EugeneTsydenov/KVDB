package storage

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMemoryStore_KeysConcurrentAccess(t *testing.T) {
	t.Parallel()

	const workers = 8
	const iterations = 250
	const initialKeys = 128

	store := NewMemoryStore()

	for i := 0; i < initialKeys; i++ {
		key := fmt.Sprintf("stable:%04d", i)
		if err := store.Set(
			key,
			NewStringValue([]byte("initial")),
			0,
		); err != nil {
			t.Fatalf("Set(%q): %v", key, err)
		}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup

	for workerID := 0; workerID < workers; workerID++ {
		workerID := workerID

		wg.Add(1)
		go func() {
			defer wg.Done()

			<-start

			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf(
					"worker:%02d:%03d",
					workerID,
					i%64,
				)

				value := NewStringValue([]byte(
					fmt.Sprintf("worker-%d-value-%d", workerID, i),
				))

				if err := store.Set(key, value, 0); err != nil {
					t.Errorf("Set(%q): %v", key, err)
					continue
				}

				got, err := store.Get(key)
				if err != nil {
					t.Errorf("Get(%q): %v", key, err)
				} else if got.Type != TypeString ||
					string(got.Data) != string(value.Data) {
					t.Errorf("Get(%q) returned an unexpected value", key)
				}

				if err := store.Set("shared:hot", value, 0); err != nil {
					t.Errorf("Set(shared:hot): %v", err)
					continue
				}

				shared, err := store.Get("shared:hot")
				if err != nil {
					t.Errorf("Get(shared:hot): %v", err)
				} else if shared.Type != TypeString || len(shared.Data) == 0 {
					t.Error("Get(shared:hot) returned an invalid value")
				}

				keys, err := store.Keys(`^(stable|worker|shared):`)
				if err != nil {
					t.Errorf("Keys: %v", err)
					continue
				}

				for _, found := range keys {
					if !strings.HasPrefix(found, "stable:") &&
						!strings.HasPrefix(found, "worker:") &&
						!strings.HasPrefix(found, "shared:") {
						t.Errorf("Keys returned a non-matching key %q", found)
						break
					}
				}

				if i%3 == 0 {
					if !store.Del(key) {
						t.Errorf("Del(%q) returned false for an existing key", key)
					}
				}
			}
		}()
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("concurrent Keys operations did not finish within 10 seconds")
	}
}
