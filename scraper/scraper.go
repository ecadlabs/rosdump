package scraper

import (
	"context"
	"errors"
	"sync"
	"time"

	"git.ecadlabs.com/ecad/rostools/rosdump/config"
	"git.ecadlabs.com/ecad/rostools/rosdump/devices"
	"git.ecadlabs.com/ecad/rostools/rosdump/storage"
	"github.com/sirupsen/logrus"
)

type Exporter struct {
	Device  devices.Exporter
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
	if err != nil {
		return err
	}

	defer func() {
		e := data.Close()
		if err == nil {
			err = e
		}
	}()

	l.Infoln("adding stream to transaction...")

	metadata["time"] = tx.Timestamp()

	if err := tx.Add(s.storageCtx(ctx), metadata, data); err != nil {
		return err
	}

	return nil
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

	return tx.Commit(s.storageCtx(ctx))
}

func New(c *config.Config, logger *logrus.Logger) (*Scraper, error) {
	// Init drivers
	var (
		devCommon  config.Options
		devTimeout time.Duration
		devDriver  string
	)

	if c.Devices.Common != nil {
		devCommon = c.Devices.Common.Options
		devTimeout, _ = time.ParseDuration(c.Devices.Common.Timeout)
		devDriver = c.Devices.Common.Driver
	}

	exporters := make([]*Exporter, 0, len(c.Devices.List))

	for _, dev := range c.Devices.List {
		options := make(config.Options, len(dev.Options)+len(devCommon))
		for k, v := range devCommon {
			options[k] = v
		}

		// Override with per-device options
		for k, v := range dev.Options {
			options[k] = v
		}

		driver := devDriver
		if dev.Driver != "" {
			driver = dev.Driver
		}

		logger.WithField("driver", driver).Info("initializing device...")

		drv, err := devices.NewExporter(driver, options, logger)
		if err != nil {
			return nil, err
		}

		timeout := devTimeout
		if t, _ := time.ParseDuration(dev.Timeout); t != 0 {
			timeout = t
		}

		e := Exporter{
			Device:  drv,
			Timeout: timeout,
		}

		exporters = append(exporters, &e)
	}

	if len(exporters) == 0 {
		return nil, errors.New("No devices specified")
	}

	logger.Infof("%d devices found", len(exporters))

	storageTimeout, _ := time.ParseDuration(c.Storage.Timeout)

	ctx := context.Background()
	if storageTimeout != 0 {
		ctx, _ = context.WithTimeout(ctx, storageTimeout)
	}

	logger.WithField("driver", c.Storage.Driver).Infoln("initializing storage...")

	s, err := storage.NewStorage(ctx, c.Storage.Driver, c.Storage.Options, logger)
	if err != nil {
		return nil, err
	}

	return &Scraper{
		MaxGoroutines:  c.MaxGoroutines,
		Devices:        exporters,
		Storage:        s,
		StorageTimeout: storageTimeout,
		Logger:         logger,
	}, nil
}
