package cli

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"
	cli "github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"

	"github.com/vanclief/agent-composer/core"
	"github.com/vanclief/agent-composer/interfaces/rest"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
	"github.com/vanclief/agent-composer/interfaces/tui"
)

const version = "0.1.1"

// Run starts the CLI entrypoint.
func Run(ctx context.Context, args []string) error {
	app := &cli.App{
		Name:    "agc",
		Usage:   "Agent Composer interfaces",
		Version: version,
		Action: func(c *cli.Context) error {
			return runTUI(c.Context)
		},
		Commands: []*cli.Command{
			{
				Name:  "rest",
				Usage: "Start the REST server",
				Action: func(c *cli.Context) error {
					return runServer(c.Context)
				},
			},
			{
				Name:  "tui",
				Usage: "Start the terminal UI",
				Action: func(c *cli.Context) error {
					return runTUI(c.Context)
				},
			},
		},
	}

	return app.RunContext(ctx, args)
}

func runServer(ctx context.Context) error {
	stack, err := core.NewStack(ctx)
	if err != nil {
		return err
	}

	app := restserver.New(stack.Controller, stack.AgentsAPI, stack.HooksAPI)

	group, gctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		stack.StartScheduler(gctx)
		return nil
	})

	group.Go(func() error {
		return rest.Start(gctx, app, log.Logger)
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- group.Wait()
	}()

	var waitErr error

	select {
	case <-ctx.Done():
		waitErr = <-errCh
	case waitErr = <-errCh:
	}

	if waitErr != nil && !errors.Is(waitErr, context.Canceled) {
		return waitErr
	}

	return nil
}

func runTUI(ctx context.Context) error {
	stack, err := core.NewStack(ctx)
	if err != nil {
		return err
	}

	group, gctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		stack.StartScheduler(gctx)
		return nil
	})

	group.Go(func() error {
		err := tui.Start(gctx, stack)
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return context.Canceled
	})

	err = group.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
