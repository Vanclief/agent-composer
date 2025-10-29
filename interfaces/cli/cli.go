package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun/migrate"
	cli "github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"

	"github.com/vanclief/agent-composer/core"
	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/interfaces/rest"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
	"github.com/vanclief/agent-composer/interfaces/tui"
	appmigrations "github.com/vanclief/agent-composer/models/migrations"
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
			{
				Name:  "migrate",
				Usage: "Run database migrations",
				Subcommands: []*cli.Command{
					{
						Name:  "run",
						Usage: "Run a migration by its name or identifier (e.g. 26092025)",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Aliases:  []string{"n"},
								Usage:    "Migration identifier or name",
								Required: true,
							},
						},
						Action: func(c *cli.Context) error {
							return runMigrationByName(c.Context, c.String("name"))
						},
					},
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

func runMigrationByName(ctx context.Context, name string) error {
	ctrl, err := controller.New()
	if err != nil {
		return err
	}
	defer ctrl.DB.Close() // nolint:errcheck // Close errors are not actionable here.

	fullMigrator := migrate.NewMigrator(ctrl.DB.DB, appmigrations.Migrations)
	if err := fullMigrator.Init(ctx); err != nil {
		return err
	}

	migs, err := fullMigrator.MigrationsWithStatus(ctx)
	if err != nil {
		return err
	}

	targetName := strings.TrimSpace(name)
	var target migrate.Migration
	found := false

	for _, m := range migs {
		if migrationMatches(targetName, m) {
			target = m
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("migration %q not found", name)
	}

	if target.IsApplied() {
		log.Info().
			Str("migration", target.String()).
			Msg("Migration already applied")
		return nil
	}

	single := migrate.NewMigrations()
	single.Add(target)

	if err := ctrl.DB.RunMigrations(single); err != nil {
		return err
	}

	log.Info().
		Str("migration", target.String()).
		Msg("Migration applied successfully")

	return nil
}

func migrationMatches(target string, migration migrate.Migration) bool {
	if target == migration.Name {
		return true
	}
	if strings.EqualFold(target, migration.Comment) {
		return true
	}
	if strings.EqualFold(target, migration.String()) {
		return true
	}
	return false
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
