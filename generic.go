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

// Slice types are not recommended to use.
type SliceType interface {
	[]string | []bool |
		[]float32 | []float64 |
		[]int | []int8 | []int16 | []int32 | []int64 |
		[]uint | []uint8 | []uint16 | []uint32 | []uint64
}

// Map types are not recommended to use.
type MapType interface {
	map[string]string | map[string]bool |
		map[string]float32 | map[string]float64 |
		map[string]int | map[string]int8 | map[string]int16 | map[string]int32 | map[string]int64 |
		map[string]uint | map[string]uint8 | map[string]uint16 | map[string]uint32 | map[string]uint64
}

type ValType interface {
	ScalarType | SliceType | MapType
}

func Exists(c *Cache, key string) bool {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	_, exists := c.persistItems[key]
	if exists {
		return true
	}

	item, exists := c.volatileItems[key]
	return exists && !item.expired()
}

// WARNING: If value is in SliceType or MapType, the operation on the returned value is not thread-safe.
func Get[T ValType](c *Cache, key string) (retV T, retErr error) {
	var item Item
	var exists bool

	c.mtx.RLock()
	item, exists = c.persistItems[key]
	if !exists {
		item, exists = c.volatileItems[key]
		if !exists || item.expired() {
			c.mtx.RUnlock()
			retErr = ErrNotExists
			return
		}
	}
	c.mtx.RUnlock()

	v, ok := item.Object.(T)
	if !ok {
		retErr = ErrInvalidType
		return
	}

	return v, nil
}

func GetTTL(c *Cache, key string) (time.Duration, error) {
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

func GetWithTTL[T ValType](c *Cache, key string) (T, time.Duration, error) {
	var t T
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	item, exists := c.persistItems[key]
	if exists {
		if v, ok := item.Object.(T); !ok {
			return t, 0, ErrInvalidType
		} else {
			return v, NEVER_EXPIRE, nil
		}
	}

	item, exists = c.volatileItems[key]
	if !exists {
		return t, 0, ErrNotExists
	}

	v, ok := item.Object.(T)
	if !ok {
		return t, 0, ErrInvalidType
	}

	nowMs := time.Now().UnixMilli()
	if item.ExpireMs < nowMs {
		return t, 0, ErrNotExists
	} else {
		return v, time.Duration(item.ExpireMs-nowMs) * time.Millisecond, nil
	}
}

// Note: thread-safe but expensive
func GetSliceCopy[T ScalarType](c *Cache, key string) (retV []T, retErr error) {
	var item Item
	var exists bool

	c.mtx.RLock()
	defer c.mtx.RUnlock()

	item, exists = c.persistItems[key]
	if !exists {
		item, exists = c.volatileItems[key]
		if !exists || item.expired() {
			c.mtx.RUnlock()
			retErr = ErrNotExists
			return
		}
	}

	vv, ok := item.Object.([]T)
	if !ok {
		retErr = ErrInvalidType
		return
	}

	retV = make([]T, len(vv))
	copy(retV, vv)

	return
}

// Note: Thread-safe but expensive
func GetMapCopy[T ScalarType](c *Cache, key string) (retV map[string]T, retErr error) {
	var item Item
	var exists bool

	c.mtx.RLock()
	defer c.mtx.RUnlock()

	item, exists = c.persistItems[key]
	if !exists {
		item, exists = c.volatileItems[key]
		if !exists || item.expired() {
			c.mtx.RUnlock()
			retErr = ErrNotExists
			return
		}
	}

	vv, ok := item.Object.(map[string]T)
	if !ok {
		retErr = ErrInvalidType
		return
	}

	retV = make(map[string]T)
	for k, v := range vv {
		retV[k] = v
	}

	return
}

func Set[T ValType](c *Cache, key string, val T, ttl time.Duration) {
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

func Delete(c *Cache, key string) {
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

func DeleteKeys(c *Cache, keys []string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	for _, key := range keys {
		delete(c.persistItems, key)
		delete(c.volatileItems, key)
	}
}

func Increase[T NumType](c *Cache, key string, val T) (T, error) {
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

	if item.neverExpire() {
		c.persistItems[key] = item
	} else {
		c.volatileItems[key] = item
	}
	c.changed = true

	return newV, nil
}

func Decrease[T NumType](c *Cache, key string, val T) (T, error) {
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

	if item.neverExpire() {
		c.persistItems[key] = item
	} else {
		c.volatileItems[key] = item
	}
	c.changed = true

	return newV, nil
}

// Append scalar to an existing slice cache
func AppendToSlice[T ScalarType](c *Cache, key string, val T) error {
	var item Item
	var exists bool

	c.mtx.Lock()
	defer c.mtx.Unlock()

	item, exists = c.persistItems[key]
	if !exists {
		item, exists = c.volatileItems[key]
		if !exists || item.expired() {
			return ErrNotExists
		}
	}

	valSlice, ok := item.Object.([]T)
	if !ok {
		return ErrInvalidType
	}
	valSlice = append(valSlice, val)
	item.Object = valSlice

	if item.neverExpire() {
		c.persistItems[key] = item
	} else {
		c.volatileItems[key] = item
	}
	c.changed = true

	return nil
}

// Insert scalar to an existing map cache
func InsertToMap[T ScalarType](c *Cache, key string, name string, val T) error {
	var item Item
	var exists bool

	c.mtx.Lock()
	defer c.mtx.Unlock()

	item, exists = c.persistItems[key]
	if !exists {
		item, exists = c.volatileItems[key]
		if !exists || item.expired() {
			return ErrNotExists
		}
	}

	valMap, ok := item.Object.(map[string]T)
	if !ok {
		return ErrInvalidType
	}
	valMap[name] = val

	if item.neverExpire() {
		c.persistItems[key] = item
	} else {
		c.volatileItems[key] = item
	}
	c.changed = true

	return nil
}

// Delete value from an existing map
func DeleteFromMap[T ScalarType](c *Cache, key string, name string) error {
	var item Item
	var exists bool

	c.mtx.Lock()
	defer c.mtx.Unlock()

	item, exists = c.persistItems[key]
	if !exists {
		item, exists = c.volatileItems[key]
		if !exists || item.expired() {
			return ErrNotExists
		}
	}

	valMap, ok := item.Object.(map[string]T)
	if !ok {
		return ErrInvalidType
	}
	delete(valMap, name)

	return nil
}

func Keys(c *Cache) []string {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	keys := make([]string, 0, len(c.persistItems)+len(c.volatileItems))
	for k := range c.persistItems {
		keys = append(keys, k)
	}
	for k := range c.volatileItems {
		keys = append(keys, k)
	}

	return keys
}

// could all items which may include the expired items
func Len(c *Cache) int {
	c.mtx.RLock()
	n1 := len(c.persistItems)
	n2 := len(c.volatileItems)
	c.mtx.RUnlock()

	return n1 + n2
}

// count only unexpired items, more expensive than TotalItems
func LenValid(c *Cache) int {
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
