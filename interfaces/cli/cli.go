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

// Run starts the CLI entrypoint.
func Run(ctx context.Context, args []string) error {
	app := &cli.App{
		Name:  "agc",
		Usage: "Agent Composer interfaces",
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
	stack, err := core.New(ctx)
	if err != nil {
		return err
	}

	app := restserver.New(stack.Controller, stack.AgentsAPI, stack.HooksAPI)

	group, gctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		rest.Start(gctx, app, log.Logger)
		return nil
	})

	group.Go(func() error {
		stack.StartScheduler(gctx)
		return nil
	})

	<-ctx.Done()

	err = group.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

func runTUI(ctx context.Context) error {
	stack, err := core.New(ctx)
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
		return nil
	})

	err = group.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
