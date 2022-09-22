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

	persister := &FilePersister{FilePath: "persist.bin"}
	c, err := New(ctx, time.Second*2, time.Second*3, persister)
	if err != nil {
		t.Error("create cache error:", err)
		return
	}

	Set(c, "k1", "v1", time.Second*100)
	Set(c, "k2", true, time.Second*100)
	Set(c, "k3", int8(-1), NEVER_EXPIRE)
	Set(c, "k4", uint8(1), NEVER_EXPIRE)
	Set(c, "k5", int(-2), NEVER_EXPIRE)
	Set(c, "k6", uint(2), NEVER_EXPIRE)
	Set(c, "k7", float32(1.0), NEVER_EXPIRE)
	Set(c, "k8", []string{"a", "b"}, NEVER_EXPIRE)
	Set(c, "k9", []int{1, 2, 3}, time.Second*100)

	totalItems := Len(c)
	if totalItems != 9 {
		t.Error("invalid number of items")
		return
	}

	v1, err := Get[string](c, "k1")
	if err != nil {
		t.Error(err)
		return
	} else if v1 != "v1" {
		t.Errorf("value error. key: k1, expect: v1, got: %v", v1)
		return
	}

	v3, err := Get[int8](c, "k3")
	if err != nil {
		t.Error(err)
		return
	} else if v3 != -1 {
		t.Errorf("value error. key: k3, expect: 1, got: %v", v3)
		return
	}

	time.Sleep(time.Second * 4)
	c2, err := New(ctx, time.Second*2, time.Second*3, persister)
	if err != nil {
		t.Error("create cache error:", err)
		return
	}

	v2, err := Get[string](c2, "k1")
	if err != nil {
		t.Error(err)
		return
	} else if v2 != "v1" {
		t.Errorf("invalid get val: %v -> %v", v1, v2)
		return
	}
}
