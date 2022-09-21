package gcache

import "time"

type Item struct {
	Object   interface{}
	ExpireAt time.Time // expiration time, never expire if equals to 0
}

func (item *Item) expired() bool {
	return time.Now().After(item.ExpireAt)
}
