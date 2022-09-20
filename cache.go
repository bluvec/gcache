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

	Get(key string) (interface{}, error)
	GetTTL(key string) (time.Duration, error)
	Set(key string, val interface{}, ttl time.Duration)
	Del(key string)
	Inc(key string, val interface{}) (interface{}, error)
	Dec(key string, val interface{}) (interface{}, error)

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

func New(ctx context.Context, cleanupInterval, persistInterval time.Duration, persister Persister) (Cache, error) {
	var c cache
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.items = make(map[string]Item)
	c.changed = false
	c.w.cleanupInterval = cleanupInterval
	c.w.persistInterval = persistInterval
	c.persister = persister

	if persister != nil {
		if items, err := persister.Load(); err != nil {
			return nil, err
		} else {
			c.items = items
		}
	}

	go c.w.Run(c.ctx, &c)

	return &c, nil
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

func (c *cache) Inc(key string, val interface{}) (interface{}, error) {
	c.mtx.Lock()
	item, exists := c.items[key]
	if !exists || item.expired() {
		c.mtx.Unlock()
		return nil, ErrNotExists
	}

	switch item.Object.(type) {
	case int:
		if v, ok := val.(int); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(int) + v
		}
	case int8:
		if v, ok := val.(int8); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(int8) + v
		}
	case int16:
		if v, ok := val.(int16); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(int16) + v
		}
	case int32:
		if v, ok := val.(int32); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(int32) + v
		}
	case int64:
		if v, ok := val.(int64); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(int64) + v
		}
	case uint:
		if v, ok := val.(uint); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(uint) + v
		}
	case uint8:
		if v, ok := val.(uint8); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(uint8) + v
		}
	case uint16:
		if v, ok := val.(uint16); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(uint16) + v
		}
	case uint32:
		if v, ok := val.(uint32); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(uint32) + v
		}
	case uint64:
		if v, ok := val.(uint64); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(uint64) + v
		}
	case float32:
		if v, ok := val.(float32); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(float32) + v
		}
	case float64:
		if v, ok := val.(float64); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(float64) + v
		}
	default:
		c.mtx.Unlock()
		return nil, ErrInvalidType
	}

	c.items[key] = item
	c.changed = true
	c.mtx.Unlock()

	return item.Object, nil
}

func (c *cache) Dec(key string, val interface{}) (interface{}, error) {
	c.mtx.Lock()
	item, exists := c.items[key]
	if !exists || item.expired() {
		c.mtx.Unlock()
		return 0, ErrNotExists
	}

	switch item.Object.(type) {
	case int:
		if v, ok := val.(int); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(int) - v
		}
	case int8:
		if v, ok := val.(int8); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(int8) - v
		}
	case int16:
		if v, ok := val.(int16); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(int16) - v
		}
	case int32:
		if v, ok := val.(int32); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(int32) - v
		}
	case int64:
		if v, ok := val.(int64); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(int64) - v
		}
	case uint:
		if v, ok := val.(uint); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(uint) - v
		}
	case uint8:
		if v, ok := val.(uint8); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(uint8) - v
		}
	case uint16:
		if v, ok := val.(uint16); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(uint16) - v
		}
	case uint32:
		if v, ok := val.(uint32); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(uint32) - v
		}
	case uint64:
		if v, ok := val.(uint64); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(uint64) - v
		}
	case float32:
		if v, ok := val.(float32); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(float32) - v
		}
	case float64:
		if v, ok := val.(float64); !ok {
			c.mtx.Unlock()
			return nil, ErrInvalidType
		} else {
			item.Object = item.Object.(float64) - v
		}
	default:
		c.mtx.Unlock()
		return nil, ErrInvalidType
	}

	c.items[key] = item
	c.changed = true
	c.mtx.Unlock()

	return item.Object, nil
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
