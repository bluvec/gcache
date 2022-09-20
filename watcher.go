package gcache

import (
	"context"
	"time"
)

type watcher struct {
	cleanupInterval time.Duration
	persistInterval time.Duration
}

func (w *watcher) Run(ctx context.Context, c *cache) {
	cleanupTicker := time.NewTicker(w.cleanupInterval)
	defer cleanupTicker.Stop()

	persistTicker := time.NewTicker(w.persistInterval)
	defer persistTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.persist()
			return

		case <-cleanupTicker.C:
			c.cleanup()

		case <-persistTicker.C:
			c.persist()
		}
	}
}
