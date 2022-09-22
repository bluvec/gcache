package gcache

import (
	"context"
	"sync"
	"time"
)

const (
	NEVER_EXPIRE             time.Duration = -1
	DEFAULT_CLEANUP_INTERVAL time.Duration = time.Minute * 5
	DEFAULT_PERSIST_INTERVAL time.Duration = time.Minute * 2
)

type Cache struct {
	ctx           context.Context
	cancel        context.CancelFunc
	mtx           sync.RWMutex
	persistItems  map[string]Item
	volatileItems map[string]Item
	changed       bool
	w             watcher
	wg            sync.WaitGroup
	persister     Persister
}

func New(ctx context.Context, cleanupInterval, persistInterval time.Duration, persister Persister) (*Cache, error) {
	c := new(Cache)

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.persistItems = make(map[string]Item)
	c.volatileItems = make(map[string]Item)
	c.changed = false
	c.w.cleanupInterval = cleanupInterval
	c.w.persistInterval = persistInterval
	c.persister = persister

	if persister != nil {
		if items, err := persister.Load(); err != nil {
			return nil, err
		} else {
			for key, item := range items {
				if item.neverExpire() {
					c.persistItems[key] = item
				} else {
					c.volatileItems[key] = item
				}
			}
		}
	}

	c.wg.Add(1)
	go c.w.Run(c.ctx, &c.wg, c.persister, c.cleanup, c.persist)

	return c, nil
}

func (c *Cache) Close() error {
	c.cancel()
	c.wg.Wait()
	return nil
}

func (c *Cache) cleanup() {
	nowMs := time.Now().UnixMilli()
	c.mtx.Lock()
	for key, item := range c.volatileItems {
		if nowMs > item.ExpireMs {
			delete(c.volatileItems, key)
			c.changed = true
		}
	}
	c.mtx.Unlock()
}

func (c *Cache) persist() {
	if c.persister == nil {
		return
	}

	changed := false
	items := make(map[string]Item)
	c.mtx.RLock()
	if c.changed {
		changed = true

		for key, item := range c.persistItems {
			items[key] = item
		}

		for key, item := range c.volatileItems {
			if !item.expired() {
				items[key] = item
			}
		}
		c.changed = false
	}
	c.mtx.RUnlock()

	if changed {
		c.persister.Save(items)
	}
}
