package gcache

import "errors"

var (
	ErrNotExists   = errors.New("key not exists")
	ErrInvalidType = errors.New("invalid type")
)
