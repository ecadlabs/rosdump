package devices

import (
	"context"
	"fmt"
	"io"

	"github.com/e-asphyx/rosdump/config"
)

type Exporter interface {
	Export(context.Context) (io.ReadCloser, error)
}

type NewExporterFunc func(config.Options) (Exporter, error)

var registry = make(map[string]NewExporterFunc)

func registerExporter(name string, fn NewExporterFunc) {
	registry[name] = fn
}

func NewExporter(name string, options config.Options) (Exporter, error) {
	if fn, ok := registry[name]; ok {
		return fn(options)
	}

	return nil, fmt.Errorf("Unknown exporter driver: `%s'", name)
}
