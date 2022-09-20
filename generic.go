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

func Get[T ValType](c Cache, key string) (t T, retErr error) {
	val, err := c.Get(key)
	if err != nil {
		retErr = err
		return
	}

	v, ok := val.(T)
	if !ok {
		retErr = ErrInvalidType
		return
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

func Inc[T NumType](c Cache, key string, val T) (retVal T, retErr error) {
	cc := c.(*cache)
	cc.mtx.Lock()
	item, exists := cc.items[key]
	if !exists || item.expired() {
		cc.mtx.Unlock()
		retErr = ErrNotExists
		return
	}

	oldV, ok := item.Object.(T)
	if !ok {
		cc.mtx.Unlock()
		retErr = ErrInvalidType
		return
	}

	newV := oldV + val
	item.Object = newV
	cc.items[key] = item
	cc.mtx.Unlock()

	return newV, nil
}

func Dec[T NumType](c Cache, key string, val T) (retVal T, retErr error) {
	cc := c.(*cache)
	cc.mtx.Lock()
	item, exists := cc.items[key]
	if !exists || item.expired() {
		cc.mtx.Unlock()
		retErr = ErrNotExists
		return
	}

	oldV, ok := item.Object.(T)
	if !ok {
		cc.mtx.Unlock()
		retErr = ErrInvalidType
		return
	}

	newV := oldV - val
	item.Object = newV
	cc.items[key] = item
	cc.mtx.Unlock()

	return newV, nil
}

func ItemCount(c Cache) int {
	return c.ItemCount()
}

func NotExpiredItemCount(c Cache) int {
	return c.NotExpiredItemCount()
}
