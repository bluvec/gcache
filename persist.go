package gcache

import (
	"encoding/gob"
	"os"
)

type Persister interface {
	Load() (map[string]Item, error)
	Save(items map[string]Item) error
}

type DefaultPersister struct {
	PersistFilePath string
}

func (p *DefaultPersister) Load() (map[string]Item, error) {
	r, err := os.Open(p.PersistFilePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	dec := gob.NewDecoder(r)
	items := make(map[string]Item)

	if err := dec.Decode(&items); err != nil {
		return nil, err
	}

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

	enc := gob.NewEncoder(w)
	for _, item := range items {
		gob.Register(item.Object)
	}
	return enc.Encode(&items)
}
