package devices

import (
	"context"
	"fmt"
	"io"

	"github.com/ecadlabs/rosdump/config"
	"github.com/sirupsen/logrus"
)

type Metadata map[string]interface{}

func (m Metadata) Append(add Metadata) Metadata {
	out := make(Metadata, len(m)+len(add))

	for k, v := range m {
		out[k] = v
	}

	for k, v := range add {
		out[k] = v
	}

	return out
}

type Exporter interface {
	Export(context.Context) (io.ReadCloser, Metadata, error)
	Metadata() Metadata // For logging purposes
}

type NewExporterFunc func(config.Options, *logrus.Logger) (Exporter, error)

var registry = make(map[string]NewExporterFunc)

func registerExporter(name string, fn NewExporterFunc) {
	registry[name] = fn
}

func NewExporter(name string, options config.Options, logger *logrus.Logger) (Exporter, error) {
	if fn, ok := registry[name]; ok {
		return fn(options, logger)
	}

	return nil, fmt.Errorf("Unknown exporter driver: `%s'", name)
}
