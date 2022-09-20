package gcache

import (
	"context"
	"testing"
	"time"
)

func TestCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := New(ctx, time.Second*2, 0, nil)
	if err != nil {
		t.Error("create cache error:", err)
		return
	}

	k1 := "key1"
	v1 := "val1"
	Set(c, k1, v1, time.Second*5)
	val, err := Get[string](c, k1)
	if err != nil {
		t.Error(err)
		return
	}
	if val != v1 {
		t.Errorf("invalid get val: %v -> %v", v1, val)
		return
	}

	time.Sleep(time.Second * 8)
	if _, err := Get[string](c, k1); err != ErrNotExists {
		t.Errorf("not expired")
		return
	}
}

func TestPersist(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	persister := &DefaultPersister{PersistFilePath: "persist.bin"}
	c, err := New(ctx, time.Second*2, time.Second*3, persister)
	if err != nil {
		t.Error("create cache error:", err)
		return
	}

	k1 := "key1"
	v1 := "val1"
	Set(c, k1, v1, time.Second*100)
	val, err := Get[string](c, k1)
	if err != nil {
		t.Error(err)
		return
	}
	if val != v1 {
		t.Errorf("invalid get val: %v -> %v", v1, val)
		return
	}

	time.Sleep(time.Second * 4)
	c2, err := New(ctx, time.Second*2, time.Second*3, persister)
	if err != nil {
		t.Error("create cache error:", err)
		return
	}

	v2, err := Get[string](c2, k1)
	if err != nil {
		t.Error(err)
		return
	}

	if v2 != v1 {
		t.Errorf("invalid get val: %v -> %v", v1, v2)
		return
	}
}
