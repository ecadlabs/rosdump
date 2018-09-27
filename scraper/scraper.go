package scraper

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/ecadlabs/rosdump/config"
	"github.com/ecadlabs/rosdump/devices"
	"github.com/ecadlabs/rosdump/storage"
	"github.com/sirupsen/logrus"
)

const (
	DefaultExporterDriver = "ssh-command"
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
		if e := data.Close(); err == nil {
			err = e
		}
	}()

	l.Infoln("adding stream to transaction...")

	metadata["time"] = tx.Timestamp()

	wr, err := tx.Add(s.storageCtx(ctx), metadata)
	if err != nil {
		return err
	}

	_, err = io.Copy(wr, data)

	if e := wr.Close(); err == nil {
		err = e
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
