package scraper

import (
	"context"
	"fmt"
	"sync"
	"time"

	"git.ecadlabs.com/ecad/rostools/rosdump/devices"
	"git.ecadlabs.com/ecad/rostools/rosdump/storage"
	"github.com/sirupsen/logrus"
)

type Scraper struct {
	ExportTimeout  time.Duration
	StorageTimeout time.Duration
	MaxGoroutines  int
	Devices        []devices.Exporter
	Storage        storage.Storage
	Logger         *logrus.Logger
}

func (s *Scraper) storageCtx(parent context.Context) context.Context {
	if s.StorageTimeout != 0 {
		ctx, _ := context.WithTimeout(parent, s.StorageTimeout)
		return ctx
	}

	return parent
}

func (s *Scraper) export(ctx context.Context, d devices.Exporter, tx storage.Tx, l *logrus.Entry) (err error) {
	var exportCtx context.Context
	if s.ExportTimeout != 0 {
		exportCtx, _ = context.WithTimeout(ctx, s.ExportTimeout)
	} else {
		exportCtx = ctx
	}

	l.Infoln("exporting...")

	data, metadata, err := d.Export(exportCtx)
	if err != nil {
		fmt.Printf("export: export: %v\n", err)
		return err
	}

	defer func() {
		e := data.Close()
		if e != nil {
			fmt.Printf("export: close: %v\n", e)
		}
		if err != nil {
			err = e
		}
	}()

	l.Infoln("adding stream to transaction...")

	metadata["time"] = tx.Timestamp()

	if err := tx.Add(s.storageCtx(ctx), metadata, data); err != nil {
		fmt.Printf("export: add: %v\n", err)
		return err
	}

	return nil
}

func (s *Scraper) exportLoop(ctx context.Context, ch <-chan devices.Exporter, tx storage.Tx) {
	for d := range ch {
		l := s.Logger.WithFields(logrus.Fields(d.Metadata()))

		if err := s.export(ctx, d, tx, l); err != nil {
			l.Errorln(err)
		}

		select {
		case <-ctx.Done(): // Parent context canceled
			fmt.Printf("loop: parent context done\n")
			return
		default:
		}
	}
	fmt.Printf("loop: done\n")
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

	ch := make(chan devices.Exporter)

	var wg sync.WaitGroup
	wg.Add(gnum)

	fmt.Printf("Do: starting %d goroutines\n", gnum)

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
		fmt.Printf("Do: context done\n")
		return ctx.Err()
	default:
	}

	s.Logger.Infoln("committing...")

	return tx.Commit(s.storageCtx(ctx))
}
