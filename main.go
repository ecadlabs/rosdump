package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.ecadlabs.com/ecad/rostools/rosdump/config"
	"git.ecadlabs.com/ecad/rostools/rosdump/scraper"
	log "github.com/sirupsen/logrus"
)

func runScraper(ctx context.Context, s *scraper.Scraper, timeout time.Duration) error {
	log.Info("collecting data...")

	if timeout != 0 {
		ctx, _ = context.WithTimeout(ctx, timeout)
	}

	return s.Do(ctx)
}

func main() {
	var (
		configFile string
		once       bool
		nowait     bool
	)

	flag.StringVar(&configFile, "c", "", "Config")
	flag.BoolVar(&once, "once", false, "Run once and exit")
	flag.BoolVar(&nowait, "n", false, "Don't wait before first run")
	flag.Parse()

	if configFile == "" {
		flag.Usage()
		os.Exit(0)
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		log.Fatal(err)
	}

	sc, err := scraper.New(cfg, log.StandardLogger())
	if err != nil {
		log.Fatal(err)
	}

	timeout, _ := time.ParseDuration(cfg.Timeout)
	interval, _ := time.ParseDuration(cfg.Interval)

	if !once && interval == 0 {
		log.Fatal("Interval must not be zero")
	}

	ctx, cancel := context.WithCancel(context.Background())
	sem := make(chan struct{})

	go func() {
		defer close(sem)

		if once || nowait {
			if err := runScraper(ctx, sc, timeout); err != nil {
				if once {
					log.Fatal(err)
				} else {
					log.Error(err)
				}
			}

			if once {
				os.Exit(0)
			}
		}

		tick := time.Tick(interval)
		for {
			select {
			case <-tick:
				if err := runScraper(ctx, sc, timeout); err != nil {
					log.Error(err)
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case s := <-signalChan:
		log.Printf("captured %v\n", s)
		cancel()
	}
	<-sem

	log.Info("done")
}
