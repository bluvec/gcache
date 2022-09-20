package gcache

import (
	"context"
	"math"
	"sync"
	"time"
)

const (
	NO_EXPIRATION time.Duration = -1
)

var (
	gNoExpiration = time.Unix(math.MaxInt64, 0)
)

type Cache interface {
	Exists(key string) bool

	Set(key string, val interface{}, ttl time.Duration)
	Del(key string)

	IncInt(key string, val int) (int, error)
	IncInt64(key string, val int64) (int64, error)
	IntUint64(key string, val uint64) (uint64, error)

	DecInt(key string, val int) (int, error)
	DecInt64(key string, val int64) (int64, error)
	DecUint64(key string, val uint64) (uint64, error)

	Get(key string) (interface{}, error)
	GetTTL(key string) (time.Duration, error)
	GetWithTTL(key string) (interface{}, time.Duration, error)

	ItemCount() int           // may include the expired items not cleaned up
	NotExpiredItemCount() int // expensive
}

type cache struct {
	ctx       context.Context
	cancel    context.CancelFunc
	mtx       sync.RWMutex
	items     map[string]Item
	changed   bool
	w         watcher
	persister Persister
}

func New(ctx context.Context, cleanupInterval, persistInterval time.Duration, persister Persister) Cache {
	var c cache
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.items = make(map[string]Item)
	c.changed = false
	c.w.cleanupInterval = cleanupInterval
	c.w.persistInterval = persistInterval
	c.persister = persister

	if persister != nil {
		if items, err := persister.Load(); err != nil {
			return nil
		} else {
			c.items = items
		}
	}

	go c.w.Run(c.ctx, &c)

	return &c
}

func (c *cache) Close() error {
	c.cancel()
	return nil
}

func (c *cache) cleanup() {
	c.mtx.Lock()
	now := time.Now()
	for key, item := range c.items {
		if now.After(item.ExpireAt) {
			delete(c.items, key)
			c.changed = true
		}
	}
	c.mtx.Unlock()
}

func (c *cache) persist() {
	if c.persister == nil {
		return
	}

	items := make(map[string]Item)
	c.mtx.Lock()
	if c.changed {
		for key, item := range c.items {
			if !item.expired() {
				items[key] = item
			}
		}
		c.changed = false
	}
	c.mtx.Unlock()

	c.persister.Save(items)
}

func (c *cache) Exists(key string) bool {
	c.mtx.RLock()
	item, exists := c.items[key]
	c.mtx.RUnlock()

	if time.Now().After(item.ExpireAt) {
		return false
	}

	return exists
}

func (c *cache) Set(key string, val interface{}, ttl time.Duration) {
	var expireAt time.Time
	if ttl == 0 {
		expireAt = gNoExpiration
	} else {
		expireAt = time.Now().Add(ttl)
	}

	c.mtx.Lock()
	c.items[key] = Item{
		Object:   val,
		ExpireAt: expireAt,
	}
	c.changed = true

	// do not use defer because it adds ~200 ns
	c.mtx.Unlock()
}

func (c *cache) Del(key string) {
	c.mtx.Lock()
	if _, existed := c.items[key]; existed {
		delete(c.items, key)
		c.changed = true
	}
	c.mtx.Unlock()
}

func (c *cache) IncInt(key string, val int) (int, error) {
	c.mtx.Lock()
	item, exists := c.items[key]
	if !exists || item.expired() {
		c.mtx.Unlock()
		return 0, ErrNotExists
	}

	curVal, ok := item.Object.(int)
	if !ok {
		c.mtx.Unlock()
		return 0, ErrInvalidType
	}

	newVal := curVal + val
	item.Object = newVal
	c.items[key] = item
	c.changed = true
	c.mtx.Unlock()

	return newVal, nil
}

func (c *cache) IncInt64(key string, val int64) (int64, error) {
	c.mtx.Lock()
	item, exists := c.items[key]
	if !exists || item.expired() {
		c.mtx.Unlock()
		return 0, ErrNotExists
	}

	curVal, ok := item.Object.(int64)
	if !ok {
		c.mtx.Unlock()
		return 0, ErrInvalidType
	}

	newVal := curVal + val
	item.Object = newVal
	c.items[key] = item
	c.changed = true
	c.mtx.Unlock()

	return newVal, nil
}

func (c *cache) IntUint64(key string, val uint64) (uint64, error) {
	c.mtx.Lock()
	item, exists := c.items[key]
	if !exists || item.expired() {
		c.mtx.Unlock()
		return 0, ErrNotExists
	}

	curVal, ok := item.Object.(uint64)
	if !ok {
		c.mtx.Unlock()
		return 0, ErrInvalidType
	}

	newVal := curVal + val
	item.Object = newVal
	c.items[key] = item
	c.changed = true
	c.mtx.Unlock()

	return newVal, nil
}

func (c *cache) DecInt(key string, val int) (int, error) {
	c.mtx.Lock()
	item, exists := c.items[key]
	if !exists || item.expired() {
		c.mtx.Unlock()
		return 0, ErrNotExists
	}

	curVal, ok := item.Object.(int)
	if !ok {
		c.mtx.Unlock()
		return 0, ErrInvalidType
	}

	newVal := curVal - val
	item.Object = newVal
	c.items[key] = item
	c.changed = true
	c.mtx.Unlock()

	return newVal, nil
}

func (c *cache) DecInt64(key string, val int64) (int64, error) {
	c.mtx.Lock()
	item, exists := c.items[key]
	if !exists || item.expired() {
		c.mtx.Unlock()
		return 0, ErrNotExists
	}

	curVal, ok := item.Object.(int64)
	if !ok {
		c.mtx.Unlock()
		return 0, ErrInvalidType
	}

	newVal := curVal - val
	item.Object = newVal
	c.items[key] = item
	c.changed = true
	c.mtx.Unlock()

	return newVal, nil
}

func (c *cache) DecUint64(key string, val uint64) (uint64, error) {
	c.mtx.Lock()
	item, exists := c.items[key]
	if !exists || item.expired() {
		c.mtx.Unlock()
		return 0, ErrNotExists
	}

	curVal, ok := item.Object.(uint64)
	if !ok {
		c.mtx.Unlock()
		return 0, ErrInvalidType
	}

	newVal := curVal - val
	item.Object = newVal
	c.items[key] = item
	c.changed = true
	c.mtx.Unlock()

	return newVal, nil
}

func (c *cache) Get(key string) (interface{}, error) {
	c.mtx.RLock()
	item, exists := c.items[key]
	c.mtx.RUnlock()

	if !exists || item.expired() {
		return nil, ErrNotExists
	}

	return item.Object, nil
}

func (c *cache) GetTTL(key string) (time.Duration, error) {
	c.mtx.RLock()
	item, exists := c.items[key]
	c.mtx.RUnlock()

	if !exists {
		return 0, ErrNotExists
	}

	if item.ExpireAt == gNoExpiration {
		return NO_EXPIRATION, nil
	}

	ttl := time.Until(item.ExpireAt)
	if ttl < 0 {
		return 0, ErrNotExists
	} else {
		return ttl, nil
	}
}

func (c *cache) GetWithTTL(key string) (interface{}, time.Duration, error) {
	c.mtx.RLock()
	item, exists := c.items[key]
	c.mtx.RUnlock()

	if !exists {
		return nil, 0, ErrNotExists
	}

	if item.ExpireAt == gNoExpiration {
		return item.Object, NO_EXPIRATION, nil
	}

	ttl := time.Until(item.ExpireAt)
	if ttl < 0 {
		return nil, 0, ErrNotExists
	} else {
		return item.Object, ttl, nil
	}
}

func (c *cache) ItemCount() int {
	c.mtx.RLock()
	n := len(c.items)
	c.mtx.RUnlock()

	return n
}

func (c *cache) NotExpiredItemCount() int {
	c.mtx.RLock()
	now := time.Now()
	var n = 0
	for _, item := range c.items {
		if now.Before(item.ExpireAt) {
			n++
		}
	}
	c.mtx.RUnlock()

	return n
}
