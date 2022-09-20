package gcache

import "time"

type NumType interface {
	float32 | float64 |
		int | int8 | int16 | int32 | int64 |
		uint | uint8 | uint16 | uint32 | uint64
}

type SliceType interface {
	[]string | []bool |
		[]float32 | []float64 |
		[]int | []int8 | []int16 | []int32 | []int64 |
		[]uint | []uint8 | []uint16 | []uint32 | []uint64
}

type ValType interface {
	string | bool | NumType | SliceType
}

func Exists(c Cache, key string) bool {
	return c.Exists(key)
}

func Get[T ValType](c Cache, key string) (T, error) {
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

func GetTTL(c Cache, key string) (time.Duration, error) {
	return c.GetTTL(key)
}

func Set[T ValType](c Cache, key string, val T, ttl time.Duration) {
	c.Set(key, val, ttl)
}

func Del(c Cache, key string) {
	c.Del(key)
}

func Inc[T NumType](c Cache, key string, val T) (T, error) {
	var retVal T
	var item Item
	var exists bool
	cc := c.(*cache)

	cc.mtx.Lock()
	defer cc.mtx.Unlock()

	item, exists = cc.persistItems[key]
	if !exists {
		item, exists = cc.volatileItems[key]
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
		cc.persistItems[key] = item
	} else {
		cc.volatileItems[key] = item
	}
	cc.changed = true

	return newV, nil
}

func Dec[T NumType](c Cache, key string, val T) (T, error) {
	var retVal T
	var item Item
	var exists bool
	cc := c.(*cache)

	cc.mtx.Lock()
	defer cc.mtx.Unlock()

	item, exists = cc.persistItems[key]
	if !exists {
		item, exists = cc.volatileItems[key]
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
		cc.persistItems[key] = item
	} else {
		cc.volatileItems[key] = item
	}
	cc.changed = true

	return newV, nil
}

func TotalItems(c Cache) int {
	return c.TotalItems()
}

func TotalValidItems(c Cache) int {
	return c.TotalValidItems()
}
