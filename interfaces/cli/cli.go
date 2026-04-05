package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun/migrate"
	cli "github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"

	"github.com/vanclief/agent-composer/core"
	"github.com/vanclief/agent-composer/core/controller"
	workflowexecutions "github.com/vanclief/agent-composer/core/resources/workflow/executions"
	"github.com/vanclief/agent-composer/interfaces/rest"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
	appmigrations "github.com/vanclief/agent-composer/models/migrations"
	"github.com/vanclief/ez"
)

const version = "0.2.15"

// Run starts the CLI entrypoint.
func Run(ctx context.Context, args []string) error {
	app := &cli.App{
		Name:    "agc",
		Usage:   "Agent Composer interfaces",
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "shell-root",
				Usage:   "Root directory for shell tool access (defaults to current directory)",
				Value:   "",
				EnvVars: []string{"AGC_SHELL_ROOT"},
			},
		},
		Action: func(c *cli.Context) error {
			return cli.ShowAppHelp(c)
		},
		Commands: []*cli.Command{
			workflowRunCommand(),
			{
				Name:  "rest",
				Usage: "Start the REST server",
				Action: func(c *cli.Context) error {
					return runServer(c.Context, c.String("shell-root"))
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

func workflowRunCommand() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Compile and run a workflow blueprint from the registry or disk",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "id",
				Usage: "Workflow blueprint id from the workflow registry",
			},
			&cli.StringFlag{
				Name:  "file",
				Usage: "Path to the workflow blueprint YAML file",
				Value: "specs/dsl/examples/pipeline-summary-critique-revise.yaml",
			},
			&cli.StringFlag{
				Name:  "input-file",
				Usage: "Path to a JSON file containing workflow inputs",
			},
			&cli.StringFlag{
				Name:  "input-json",
				Usage: "Inline JSON object containing workflow inputs",
			},
		},
		Action: func(c *cli.Context) error {
			return runWorkflow(c.Context, c.String("id"), c.String("file"), c.String("input-file"), c.String("input-json"), c.String("shell-root"))
		},
	}
}

func runServer(ctx context.Context, shellRoot string) error {
	stack, err := core.NewStack(ctx, core.StackOptions{ShellRoot: shellRoot})
	if err != nil {
		return err
	}

	app := restserver.New(stack.Controller, stack.HooksAPI, stack.WorkflowAPI)
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
	err = fullMigrator.Init(ctx)
	if err != nil {
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

	err = ctrl.DB.RunMigrations(single)
	if err != nil {
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

func runWorkflow(ctx context.Context, workflowID string, filePath string, inputFile string, inputJSON string, shellRoot string) error {
	stack, err := core.NewStack(ctx, core.StackOptions{ShellRoot: shellRoot})
	if err != nil {
		return err
	}

	defer stack.Controller.DB.Close() // nolint:errcheck // Close errors are not actionable here.

	input, err := loadWorkflowInput(strings.TrimSpace(inputFile), strings.TrimSpace(inputJSON))
	if err != nil {
		return err
	}

	response, err := stack.WorkflowAPI.Executions.Create(ctx, nil, &workflowexecutions.CreateRequest{
		WorkflowID: strings.TrimSpace(workflowID),
		File:       strings.TrimSpace(filePath),
		Input:      input,
		ShellRoot:  strings.TrimSpace(shellRoot),
	})
	if err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(response.Output, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(encoded))

	return nil
}

func loadWorkflowInput(inputFile string, inputJSON string) (map[string]any, error) {
	const op = "cli.loadWorkflowInput"

	if inputFile == "" && inputJSON == "" {
		return nil, ez.New(op, ez.EINVALID, "one of --input-file or --input-json is required", nil)
	}

	var raw []byte
	if inputFile != "" {
		content, err := os.ReadFile(inputFile)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
		raw = content
	} else {
		raw = []byte(inputJSON)
	}

	var input map[string]any
	err := json.Unmarshal(raw, &input)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return input, nil
}
