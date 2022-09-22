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

	go c.w.Run(c.ctx, c)

	return c, nil
}

func (c *Cache) Close() error {
	c.cancel()
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

func (c *Cache) Exists(key string) bool {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	_, exists := c.persistItems[key]
	if exists {
		return true
	}

	item, exists := c.volatileItems[key]
	return exists && !item.expired()
}

func (c *Cache) Get(key string) (any, error) {
	var item Item
	var exists bool

	c.mtx.RLock()
	defer c.mtx.RUnlock()

	item, exists = c.persistItems[key]
	if exists {
		return item.Object, nil
	}

	item, exists = c.volatileItems[key]
	if !exists || item.expired() {
		return nil, ErrNotExists
	}

	return item.Object, nil
}

func (c *Cache) GetTTL(key string) (time.Duration, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	item, exists := c.persistItems[key]
	if exists {
		return NEVER_EXPIRE, nil
	}

	item, exists = c.volatileItems[key]
	if !exists {
		return 0, ErrNotExists
	}

	nowMs := time.Now().UnixMilli()
	if item.ExpireMs < nowMs {
		return 0, ErrNotExists
	} else {
		return time.Duration(item.ExpireMs-nowMs) * time.Millisecond, nil
	}
}

func (c *Cache) Set(key string, val any, ttl time.Duration) {
	c.mtx.Lock()
	if ttl == NEVER_EXPIRE {
		delete(c.volatileItems, key)
		c.persistItems[key] = Item{
			Object:   val,
			ExpireMs: kNeverExpireMs,
		}
	} else {
		delete(c.persistItems, key)
		c.volatileItems[key] = Item{
			Object:   val,
			ExpireMs: time.Now().Add(ttl).UnixMilli(),
		}
	}
	c.changed = true
	c.mtx.Unlock()
}

func (c *Cache) Del(key string) {
	c.mtx.Lock()
	if _, existed := c.persistItems[key]; existed {
		delete(c.persistItems, key)
		c.changed = true
	} else if _, existed := c.volatileItems[key]; existed {
		delete(c.volatileItems, key)
		c.changed = true
	}
	c.mtx.Unlock()
}

func (c *Cache) TotalItems() int {
	c.mtx.RLock()
	n1 := len(c.persistItems)
	n2 := len(c.volatileItems)
	c.mtx.RUnlock()

	return n1 + n2
}

func (c *Cache) TotalValidItems() int {
	c.mtx.RLock()
	n1 := len(c.persistItems)

	n2 := 0
	for _, item := range c.volatileItems {
		if !item.expired() {
			n2++
		}
	}
	c.mtx.RUnlock()

	return n1 + n2
}
