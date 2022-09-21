package gcache

import (
	"context"
	"math"
	"sync"
	"time"
)

const (
	NO_EXPIRATION            time.Duration = -1
	DEFAULT_CLEANUP_INTERVAL time.Duration = time.Minute * 5
	DEFAULT_PERSIST_INTERVAL time.Duration = time.Minute * 2
)

var (
	gNoExpiration = time.Unix(math.MaxInt64, 0)
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
				if item.ExpireAt == gNoExpiration {
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
	c.mtx.Lock()
	now := time.Now()
	for key, item := range c.volatileItems {
		if now.After(item.ExpireAt) {
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

	items := make(map[string]Item)
	c.mtx.RLock()
	if c.changed {
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

	c.persister.Save(items)
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
		return NO_EXPIRATION, nil
	}

	item, exists = c.volatileItems[key]
	if !exists {
		return 0, ErrNotExists
	}

	ttl := time.Until(item.ExpireAt)
	if ttl < 0 {
		return 0, ErrNotExists
	} else {
		return ttl, nil
	}
}

func (c *Cache) Set(key string, val any, ttl time.Duration) {
	var expireAt time.Time
	if ttl == NO_EXPIRATION {
		expireAt = gNoExpiration
	} else {
		expireAt = time.Now().Add(ttl)
	}

	c.mtx.Lock()
	if ttl == NO_EXPIRATION {
		delete(c.volatileItems, key)
		c.persistItems[key] = Item{
			Object:   val,
			ExpireAt: expireAt,
		}
	} else {
		delete(c.persistItems, key)
		c.volatileItems[key] = Item{
			Object:   val,
			ExpireAt: expireAt,
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

	now := time.Now()
	n2 := 0
	for _, item := range c.volatileItems {
		if now.Before(item.ExpireAt) {
			n2++
		}
	}
	c.mtx.RUnlock()

	return n1 + n2
}
