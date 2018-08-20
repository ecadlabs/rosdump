package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/e-asphyx/rosdump/config"
)

type Tx interface {
	Add(ctx context.Context, metadata map[string]interface{}, stream io.Reader) error
	Commit(ctx context.Context) error
}

type Storage interface {
	Begin(ctx context.Context) (Tx, error)
}

type NewStorageFunc func(config.Options) (Storage, error)

var registry = make(map[string]NewStorageFunc)

func registerStorage(name string, fn NewStorageFunc) {
	registry[name] = fn
}

func NewStorage(name string, options config.Options) (Storage, error) {
	if fn, ok := registry[name]; ok {
		return fn(options)
	}

	return nil, fmt.Errorf("Unknown storage driver: `%s'", name)
}
