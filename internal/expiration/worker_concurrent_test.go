package expiration

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"kvdb/internal/storage"
)

type recordingBatchDeleter struct {
	store *storage.MemoryStore

	mu      sync.Mutex
	calls   int
	deleted int
}

func (d *recordingBatchDeleter) DeleteExpiredBatch(
	sampleSize int,
) int {
	deleted := d.store.DeleteExpiredBatch(sampleSize)

	d.mu.Lock()
	d.calls++
	d.deleted += deleted
	d.mu.Unlock()

	return deleted
}

func (d *recordingBatchDeleter) stats() (int, int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.calls, d.deleted
}

func TestWorker_ConcurrentGetSetAndCleanup(t *testing.T) {
	t.Parallel()

	const workers = 12
	const iterations = 300

	store := storage.NewMemoryStore()

	if err := store.Set(
		"cleanup-seed",
		storage.NewStringValue([]byte("old")),
		2*time.Millisecond,
	); err != nil {
		t.Fatalf("Set cleanup-seed: %v", err)
	}

	deleter := &recordingBatchDeleter{
		store: store,
	}

	ctx, cancel := context.WithCancel(context.Background())

	var workerWG sync.WaitGroup
	workerWG.Add(1)

	worker := NewWorker(
		deleter,
		1024,
		time.Millisecond,
	)

	go worker.Start(ctx, &workerWG)

	defer func() {
		cancel()
		workerWG.Wait()
	}()

	var loadWG sync.WaitGroup

	for workerID := 0; workerID < workers; workerID++ {
		loadWG.Add(1)

		go func(id int) {
			defer loadWG.Done()

			key := fmt.Sprintf("worker:%d", id)
			expiringKey := fmt.Sprintf("expiring:%d", id)

			for i := 0; i < iterations; i++ {
				value := storage.NewStringValue(
					[]byte(fmt.Sprintf("worker-%d-value-%d", id, i)),
				)

				store.Set(key, value, 0)

				got, err := store.Get(key)
				if err != nil {
					t.Errorf(
						"worker %d: Get(%q): %v",
						id,
						key,
						err,
					)
					continue
				}

				if got.Type != storage.TypeString ||
					string(got.Data) != string(value.Data) {
					t.Errorf(
						"worker %d: Get(%q) returned unexpected value",
						id,
						key,
					)
				}

				store.Set("shared", value, 0)

				got, err = store.Get("shared")
				if err != nil {
					t.Errorf(
						"worker %d: Get(shared): %v",
						id,
						err,
					)
					continue
				}

				if got.Type != storage.TypeString || len(got.Data) == 0 {
					t.Errorf(
						"worker %d: Get(shared) returned invalid value",
						id,
					)
				}

				if err := store.Set(
					expiringKey,
					value,
					time.Millisecond,
				); err != nil {
					t.Errorf(
						"worker %d: Set(%q): %v",
						id,
						expiringKey,
						err,
					)
					continue
				}

				got, err = store.Get(expiringKey)

				if err != nil &&
					!errors.Is(err, storage.ErrEntryNotFound) {
					t.Errorf(
						"worker %d: Get(%q): unexpected error: %v",
						id,
						expiringKey,
						err,
					)
				}

				if err == nil &&
					(string(got.Data) != string(value.Data) ||
						got.Type != storage.TypeString) {
					t.Errorf(
						"worker %d: Get(%q) returned unexpected value",
						id,
						expiringKey,
					)
				}
			}
		}(workerID)
	}

	loadWG.Wait()

	deadline := time.Now().Add(2 * time.Second)

	for {
		calls, deleted := deleter.stats()

		if deleted > 0 {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf(
				"background cleanup did not delete expired entries: "+
					"calls=%d, deleted=%d",
				calls,
				deleted,
			)
		}

		time.Sleep(time.Millisecond)
	}
}
