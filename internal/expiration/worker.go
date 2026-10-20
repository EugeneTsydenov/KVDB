package expiration

import (
	"context"
	"sync"
	"time"
)

type BatchDeleter interface {
	DeleteExpiredBatch(sampleSize int) int
}

type Worker struct {
	store      BatchDeleter
	sampleSize int
	interval   time.Duration
}

func NewWorker(store BatchDeleter, sampleSize int, interval time.Duration) *Worker {
	return &Worker{
		store:      store,
		sampleSize: sampleSize,
		interval:   interval,
	}
}

func (w *Worker) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.store.DeleteExpiredBatch(w.sampleSize)
		case <-ctx.Done():
			return
		}
	}
}
