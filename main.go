package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	appcli "github.com/vanclief/agent-composer/interfaces/cli"
)

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := appcli.Run(rootCtx, os.Args)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error().Err(err).Msg("agc exited with error")
		os.Exit(1)
	}
}
