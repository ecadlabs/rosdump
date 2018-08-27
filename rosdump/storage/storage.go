package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"git.ecadlabs.com/ecad/rostools/rosdump/config"
	"github.com/sirupsen/logrus"
)

type Tx interface {
	Add(ctx context.Context, metadata map[string]interface{}, stream io.Reader) error
	Timestamp() time.Time
	Commit(ctx context.Context) error
}

type Storage interface {
	Begin(ctx context.Context) (Tx, error)
}

type NewStorageFunc func(context.Context, config.Options, *logrus.Logger) (Storage, error)

var registry = make(map[string]NewStorageFunc)

func registerStorage(name string, fn NewStorageFunc) {
	registry[name] = fn
}

func NewStorage(ctx context.Context, name string, options config.Options, logger *logrus.Logger) (Storage, error) {
	if fn, ok := registry[name]; ok {
		return fn(ctx, options, logger)
	}

	return nil, fmt.Errorf("Unknown storage driver: `%s'", name)
}
