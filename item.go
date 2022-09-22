package gcache

import (
	"math"
	"time"
)

const (
	kNeverExpireMs int64 = math.MaxInt64
)

type Item struct {
	Object   interface{}
	ExpireMs int64 // expiration time in ms, never expire if equals to `kNoExpiration`
}

func (item *Item) expired() bool {
	return time.Now().UnixMilli() > item.ExpireMs
}

func (item *Item) neverExpire() bool {
	return item.ExpireMs == kNeverExpireMs
}
