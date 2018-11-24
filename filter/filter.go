package filter

import (
	"fmt"
	"io"

	"github.com/ecadlabs/rosdump/config"
	"github.com/sirupsen/logrus"
)

type closerWithError interface {
	CloseWithError(error) error // implemented by io.PipeWriter
}

type Filter interface {
	Start(dst io.WriteCloser, src io.Reader) error
}

type NewFilterFunc func(config.Options, *logrus.Logger) (Filter, error)

var registry = make(map[string]NewFilterFunc)

func registerFilter(name string, fn NewFilterFunc) {
	registry[name] = fn
}

func NewFilter(name string, options config.Options, logger *logrus.Logger) (Filter, error) {
	if fn, ok := registry[name]; ok {
		return fn(options, logger)
	}

	return nil, fmt.Errorf("Unknown filter: `%s'", name)
}
