package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/vanclief/agent-composer/runtime"
	"github.com/vanclief/agent-composer/server"
	"github.com/vanclief/agent-composer/server/controller"
	"github.com/vanclief/agent-composer/server/interfaces/rest"
	"github.com/vanclief/compose/components/logger"
	"github.com/vanclief/compose/components/scheduler"
)

const (
	TICK_TIME = 1 * time.Minute
)

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ctrl, err := controller.New()
	if err != nil {
		log.Err(err).Msg("Error creating controller")
		os.Exit(1)
	}

	var opts []scheduler.Option
	if ctrl.Config.App.Debug {
		l := logger.NewZero(log.Logger)
		opts = append(opts, scheduler.WithLogger(l))
	}

	sch, err := scheduler.New(TICK_TIME, opts...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create scheduler")
		os.Exit(1)
	}

	rt, err := runtime.New(rootCtx, ctrl, sch)
	if err != nil {
		log.Err(err).Msg("Error creating runtime")
		os.Exit(1)
	}

	server := server.New(ctrl, rt)

	group, gctx := errgroup.WithContext(rootCtx)

	// HTTP server
	group.Go(func() error {
		rest.Start(gctx, server, log.Logger) // no return
		return nil
	})

	// Scheduler
	group.Go(func() error {
		sch.Start(gctx) // blocks; returns after ctx cancel + waitForJobs
		return nil
	})

	// Block until we receive a signal
	<-rootCtx.Done()

	err = group.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error().Err(err).Msg("Background error")
	}
}
