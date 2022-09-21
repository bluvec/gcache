package gcache

import "time"

type NumType interface {
	float32 | float64 |
		int | int8 | int16 | int32 | int64 |
		uint | uint8 | uint16 | uint32 | uint64
}

type ScalarType interface {
	string | bool | NumType
}

type SliceType interface {
	[]string | []bool |
		[]float32 | []float64 |
		[]int | []int8 | []int16 | []int32 | []int64 |
		[]uint | []uint8 | []uint16 | []uint32 | []uint64
}

type ValType interface {
	ScalarType | SliceType
}

func Exists(c *Cache, key string) bool {
	return c.Exists(key)
}

func Get[T ValType](c *Cache, key string) (T, error) {
	var retV T

	val, err := c.Get(key)
	if err != nil {
		return retV, err
	}

	v, ok := val.(T)
	if !ok {
		return retV, ErrInvalidType
	}

	return v, nil
}

func GetTTL(c *Cache, key string) (time.Duration, error) {
	return c.GetTTL(key)
}

func Set[T ValType](c *Cache, key string, val T, ttl time.Duration) {
	c.Set(key, val, ttl)
}

func Del(c *Cache, key string) {
	c.Del(key)
}

func Inc[T NumType](c *Cache, key string, val T) (T, error) {
	var retVal T
	var item Item
	var exists bool

	c.mtx.Lock()
	defer c.mtx.Unlock()

	item, exists = c.persistItems[key]
	if !exists {
		item, exists = c.volatileItems[key]
		if !exists || item.expired() {
			return retVal, ErrNotExists
		}
	}

	oldV, ok := item.Object.(T)
	if !ok {
		return retVal, ErrInvalidType
	}

	newV := oldV + val
	item.Object = newV

	if item.ExpireAt == gNoExpiration {
		c.persistItems[key] = item
	} else {
		c.volatileItems[key] = item
	}
	c.changed = true

	return newV, nil
}

func Dec[T NumType](c *Cache, key string, val T) (T, error) {
	var retVal T
	var item Item
	var exists bool

	c.mtx.Lock()
	defer c.mtx.Unlock()

	item, exists = c.persistItems[key]
	if !exists {
		item, exists = c.volatileItems[key]
		if !exists || item.expired() {
			return retVal, ErrNotExists
		}
	}

	oldV, ok := item.Object.(T)
	if !ok {
		return retVal, ErrInvalidType
	}

	newV := oldV - val
	item.Object = newV

	if item.ExpireAt == gNoExpiration {
		c.persistItems[key] = item
	} else {
		c.volatileItems[key] = item
	}
	c.changed = true

	return newV, nil
}

func Append[T ScalarType](c *Cache, key string, val T) ([]T, error) {
	var item Item
	var exists bool

	c.mtx.Lock()
	defer c.mtx.Unlock()

	item, exists = c.persistItems[key]
	if !exists {
		item, exists = c.volatileItems[key]
		if !exists || item.expired() {
			return nil, ErrNotExists
		}
	}

	oldVal, ok := item.Object.([]T)
	if !ok {
		return nil, ErrInvalidType
	}

	newVal := append(oldVal, val)
	item.Object = newVal

	if item.ExpireAt == gNoExpiration {
		c.persistItems[key] = item
	} else {
		c.volatileItems[key] = item
	}
	c.changed = true

	return newVal, nil
}

// could all items which may include the expired items
func TotalItems(c *Cache) int {
	return c.TotalItems()
}

// count only unexpired items, more expensive than TotalItems
func TotalValidItems(c *Cache) int {
	return c.TotalValidItems()
}
