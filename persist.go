package gcache

import (
	"encoding/gob"
	"errors"
	"os"
)

type Persister interface {
	Load() (map[string]Item, error)
	Save(items map[string]Item) error
}

type DefaultPersister struct {
	PersistFilePath string
}

func init() {
	gob.Register(string(""))
	gob.Register(bool(false))
	gob.Register(int(0))
	gob.Register(uint(0))
	gob.Register(int8(0))
	gob.Register(uint8(0))
	gob.Register(int16(0))
	gob.Register(uint16(0))
	gob.Register(int32(0))
	gob.Register(uint32(0))
	gob.Register(int64(0))
	gob.Register(uint64(0))
	gob.Register(float32(0))
	gob.Register(float64(0))

	gob.Register([]string{})
	gob.Register([]bool{})
	gob.Register([]int{})
	gob.Register([]uint{})
	gob.Register([]int8{})
	gob.Register([]uint8{})
	gob.Register([]int16{})
	gob.Register([]uint16{})
	gob.Register([]int32{})
	gob.Register([]uint32{})
	gob.Register([]int64{})
	gob.Register([]uint64{})
	gob.Register([]float32{})
	gob.Register([]float64{})
}

func (p *DefaultPersister) Load() (map[string]Item, error) {
	r, err := os.Open(p.PersistFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if w, err := os.Create(p.PersistFilePath); err != nil {
				return nil, err
			} else {
				w.Close()
				return make(map[string]Item), nil
			}
		}
		return nil, err
	}
	defer r.Close()

	dec := gob.NewDecoder(r)
	items := make(map[string]Item)
	dec.Decode(&items)

	for key, item := range items {
		if item.expired() {
			delete(items, key)
		}
	}

	return items, nil
}

func (p *DefaultPersister) Save(items map[string]Item) error {
	w, err := os.Create(p.PersistFilePath)
	if err != nil {
		return err
	}
	defer w.Close()

	return gob.NewEncoder(w).Encode(&items)
}
