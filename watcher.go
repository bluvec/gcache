package gcache

import (
	"context"
	"time"
)

type watcher struct {
	cleanupInterval time.Duration
	persistInterval time.Duration
}

func (w *watcher) Run(ctx context.Context, c *Cache) {
	cleanupTicker := time.NewTicker(w.cleanupInterval)
	defer cleanupTicker.Stop()

	if c.persister == nil {
		for {
			select {
			case <-ctx.Done():
				c.persist()
				return

			case <-cleanupTicker.C:
				c.cleanup()
			}
		}
	} else {
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
}
