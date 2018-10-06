package scraper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ecadlabs/rosdump/config"
	"github.com/ecadlabs/rosdump/devices"
	"github.com/ecadlabs/rosdump/filter"
	"github.com/ecadlabs/rosdump/storage"
	"github.com/sirupsen/logrus"
)

const (
	DefaultExporterDriver = "ssh-command"
)

type Exporter struct {
	Device  devices.Exporter
	Filters []filter.Filter
	Timeout time.Duration
}

type Scraper struct {
	MaxGoroutines  int
	Devices        []*Exporter
	Storage        storage.Storage
	StorageTimeout time.Duration
	Logger         *logrus.Logger
}

func (s *Scraper) storageCtx(parent context.Context) context.Context {
	if s.StorageTimeout != 0 {
		ctx, _ := context.WithTimeout(parent, s.StorageTimeout)
		return ctx
	}

	return parent
}

func (s *Scraper) export(ctx context.Context, dev *Exporter, tx storage.Tx, l *logrus.Entry) (err error) {
	var exportCtx context.Context
	if dev.Timeout != 0 {
		exportCtx, _ = context.WithTimeout(ctx, dev.Timeout)
	} else {
		exportCtx = ctx
	}

	l.Infoln("exporting...")

	data, metadata, err := dev.Device.Export(exportCtx)
	if metadata == nil {
		metadata = make(devices.Metadata, 1)
	}
	metadata["time"] = tx.Timestamp()

	if err == nil {
		defer func() {
			if e := data.Close(); err == nil {
				err = e
			}
		}()
	}

	l.Infoln("adding stream to transaction...")

	wr, e := tx.Add(s.storageCtx(ctx), metadata)
	if e != nil {
		if err == nil {
			return e
		}

		l.Errorln(e)
		return err
	}

	if err != nil {
		wr.CloseWithError(err)
		return err
	}

	// Build filter chain

	var src io.Reader = data
	for _, f := range dev.Filters {
		r, w := io.Pipe()

		if err := f.Start(w, src); err != nil {
			return err
		}

		src = r
	}

	_, err = io.Copy(wr, src)

	if e := wr.CloseWithError(err); e != nil {
		if err == nil {
			return e
		}
	}

	return err
}

func (s *Scraper) exportLoop(ctx context.Context, ch <-chan *Exporter, tx storage.Tx) {
	for d := range ch {
		l := s.Logger.WithFields(logrus.Fields(d.Device.Metadata()))

		if err := s.export(ctx, d, tx, l); err != nil {
			l.Errorln(err)
		}

		select {
		case <-ctx.Done(): // Parent context canceled
			return
		default:
		}
	}
}

func (s *Scraper) Do(ctx context.Context) error {
	tx, err := s.Storage.Begin(s.storageCtx(ctx))
	if err != nil {
		return err
	}

	gnum := len(s.Devices)
	if s.MaxGoroutines > 0 && gnum > s.MaxGoroutines {
		gnum = s.MaxGoroutines
	}

	ch := make(chan *Exporter)

	var wg sync.WaitGroup
	wg.Add(gnum)

	for i := 0; i < gnum; i++ {
		go func() {
			s.exportLoop(ctx, ch, tx)
			wg.Done()
		}()
	}

jobLoop:
	for _, d := range s.Devices {
		select {
		case ch <- d:
		case <-ctx.Done():
			break jobLoop
		}
	}

	close(ch)
	wg.Wait()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.Logger.Infoln("committing...")

	if err := tx.Commit(s.storageCtx(ctx)); err != nil {
		return err
	}

	s.Logger.Infoln("done")
	return nil
}

func New(c *config.Config, logger *logrus.Logger) (*Scraper, error) {
	// Init filters
	declaredFilters := make(map[string]filter.Filter, len(c.Filters))
	for _, f := range c.Filters {
		if f.Name == "" {
			continue
		}

		filter, err := filter.NewFilter(f.Filter, f.Options, logger)
		if err != nil {
			return nil, err
		}

		declaredFilters[f.Name] = filter
	}

	// Init drivers
	var devCommon config.Options

	if c.Devices.Common != nil {
		devCommon = c.Devices.Common
	}

	exporters := make([]*Exporter, 0, len(c.Devices.List))

	for _, dev := range c.Devices.List {
		options := make(config.Options, len(dev)+len(devCommon))

		for k, v := range devCommon {
			options[k] = v
		}

		// Override with per-device options
		for k, v := range dev {
			options[k] = v
		}

		driver, _ := options.GetString("driver")
		if driver == "" {
			driver = DefaultExporterDriver
		}

		logger.WithField("driver", driver).Info("initializing device...")

		drv, err := devices.NewExporter(driver, options, logger)
		if err != nil {
			return nil, err
		}

		var timeout time.Duration
		if t, _ := options.GetString("timeout"); t != "" {
			timeout, _ = time.ParseDuration(t)
		}

		// Optional filters
		var filters []filter.Filter
		if val, ok := options["filters"]; ok {
			var names []string

			switch v := val.(type) {
			case string:
				names = []string{v}
			case []interface{}:
				names = make([]string, 0, len(v))
				for _, vv := range v {
					if s, ok := vv.(string); ok {
						names = append(names, s)
					}
				}
			}

			if len(names) != 0 {
				filters = make([]filter.Filter, len(names))

				for i, name := range names {
					f, ok := declaredFilters[name]
					if !ok {
						return nil, fmt.Errorf("Filter `%s' is not declared", name)
					}

					filters[i] = f
				}
			}
		}

		e := Exporter{
			Device:  drv,
			Timeout: timeout,
			Filters: filters,
		}

		exporters = append(exporters, &e)
	}

	if len(exporters) == 0 {
		return nil, errors.New("No devices specified")
	}

	logger.Infof("%d devices found", len(exporters))

	var timeout time.Duration
	if t, _ := c.Storage.GetString("timeout"); t != "" {
		timeout, _ = time.ParseDuration(t)
	}

	ctx := context.Background()
	if timeout != 0 {
		ctx, _ = context.WithTimeout(ctx, timeout)
	}

	driver, _ := c.Storage.GetString("driver")
	if driver == "" {
		return nil, errors.New("Storage driver is not specified")
	}

	logger.WithField("driver", driver).Infoln("initializing storage...")

	s, err := storage.NewStorage(ctx, driver, c.Storage, logger)
	if err != nil {
		return nil, err
	}

	return &Scraper{
		MaxGoroutines:  c.MaxGoroutines,
		Devices:        exporters,
		Storage:        s,
		StorageTimeout: timeout,
		Logger:         logger,
	}, nil
}
