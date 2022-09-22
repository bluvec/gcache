package gcache

import (
	"context"
	"sync"
	"time"
)

type watcher struct {
	cleanupInterval time.Duration
	persistInterval time.Duration
}

func (w *watcher) Run(ctx context.Context, wg *sync.WaitGroup,
	persister Persister, cleanup func(), persist func()) {
	defer wg.Done()
	defer persist()

	cleanupTicker := time.NewTicker(w.cleanupInterval)
	defer cleanupTicker.Stop()

	if persister == nil {
		for {
			select {
			case <-ctx.Done():
				return

			case <-cleanupTicker.C:
				cleanup()
			}
		}
	} else {
		persistTicker := time.NewTicker(w.persistInterval)
		defer persistTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-cleanupTicker.C:
				cleanup()

			case <-persistTicker.C:
				persist()
			}
		}
	}
}
