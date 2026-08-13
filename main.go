package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	appcli "github.com/vanclief/agent-composer/interfaces/cli"
)

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := appcli.Run(rootCtx, os.Args)
	if err != nil && !errors.Is(err, context.Canceled) {
		// Lead with the human-readable message ("Workflow x was not
		// found") — the raw chain ends in driver noise like
		// "sql: no rows in result set" and stays in the error field.
		message := err.Error()
		var ezError *ez.Error
		if errors.As(err, &ezError) {
			message = ez.ErrorMessage(ezError)
		}
		log.Error().Err(err).Msg(message)
		os.Exit(1)
	}
}
