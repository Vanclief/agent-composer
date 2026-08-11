package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/migrate"
	cli "github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"

	"github.com/vanclief/agent-composer/core"
	"github.com/vanclief/agent-composer/core/controller"
	workflowexecutions "github.com/vanclief/agent-composer/core/resources/workflow/executions"
	"github.com/vanclief/agent-composer/interfaces/rest"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
	agcmcp "github.com/vanclief/agent-composer/mcp/agc"
	appmigrations "github.com/vanclief/agent-composer/models/migrations"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

const version = "0.3.0"

type compileResult struct {
	Slug        string            `json:"slug"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Inputs      map[string]string `json:"inputs"`
	Outputs     map[string]string `json:"outputs"`
	NodeCount   int               `json:"node_count"`
}

type importResult struct {
	Slug        string            `json:"slug"`
	ID          string            `json:"id,omitempty"`
	Version     string            `json:"version,omitempty"`
	Description string            `json:"description,omitempty"`
	Inputs      map[string]string `json:"inputs"`
	Outputs     map[string]string `json:"outputs"`
}

type exportResult struct {
	Slug string `json:"slug"`
	File string `json:"file"`
}

type deleteResult struct {
	Slug    string `json:"slug"`
	Deleted bool   `json:"deleted"`
}

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
			workflowCommand(),
			{
				Name:  "mcp",
				Usage: "Start the AGC MCP stdio server",
				Action: func(c *cli.Context) error {
					return runMCPServer(c.Context, c.String("shell-root"))
				},
			},
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

func workflowCommand() *cli.Command {
	return &cli.Command{
		Name:    "workflow",
		Aliases: []string{"wf", "w"},
		Usage:   "Workflow commands",
		Subcommands: []*cli.Command{
			workflowRunCommand(),
			workflowCompileCommand(),
			workflowListCommand(),
			workflowShowCommand(),
			workflowImportCommand(),
			workflowExportCommand(),
			workflowDeleteCommand(),
			workflowVersionsCommand(),
			workflowRestoreCommand(),
		},
	}
}

func workflowRunCommand() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Compile and run a workflow from the registry or a spec file",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "slug",
				Usage: "Workflow slug from the registry",
			},
			&cli.StringFlag{
				Name:  "file",
				Usage: "Path to a workflow spec YAML file",
				Value: "examples/article_summary.yaml",
			},
			&cli.StringFlag{
				Name:  "input-file",
				Usage: "Path to a JSON file containing workflow inputs",
			},
			&cli.StringFlag{
				Name:  "input-json",
				Usage: "Inline JSON object containing workflow inputs",
			},
			&cli.StringFlag{
				Name:  "input-string",
				Usage: "Inline raw string for workflows with exactly one top-level string input",
			},
		},
		Action: func(c *cli.Context) error {
			return runWorkflow(
				c.Context,
				c.String("slug"),
				c.String("file"),
				c.String("input-file"),
				c.String("input-json"),
				c.String("input-string"),
				c.IsSet("input-file"),
				c.IsSet("input-json"),
				c.IsSet("input-string"),
				c.String("shell-root"),
			)
		},
	}
}

func workflowListCommand() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List installed workflows",
		Action: func(c *cli.Context) error {
			return listWorkflows(c.Context)
		},
	}
}

func workflowShowCommand() *cli.Command {
	return &cli.Command{
		Name:  "show",
		Usage: "Show a workflow's YAML spec from the registry or disk",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "slug",
				Usage: "Workflow slug from the registry",
			},
			&cli.StringFlag{
				Name:  "file",
				Usage: "Path to a workflow spec YAML file",
			},
		},
		Action: func(c *cli.Context) error {
			return showWorkflow(c.Context, c.String("slug"), c.String("file"))
		},
	}
}

func workflowCompileCommand() *cli.Command {
	return &cli.Command{
		Name:  "compile",
		Usage: "Compile a workflow from the registry or a spec file without running it",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "slug",
				Usage: "Workflow slug from the registry",
			},
			&cli.StringFlag{
				Name:  "file",
				Usage: "Path to a workflow spec YAML file",
			},
		},
		Action: func(c *cli.Context) error {
			return compileWorkflow(c.Context, c.String("slug"), c.String("file"))
		},
	}
}

func workflowImportCommand() *cli.Command {
	return &cli.Command{
		Name:  "import",
		Usage: "Import a workflow spec file into the registry",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Usage:    "Path to a workflow spec YAML file",
				Required: true,
			},
			&cli.BoolFlag{
				Name:  "overwrite",
				Usage: "Overwrite an existing workflow with the same slug",
			},
		},
		Action: func(c *cli.Context) error {
			return importWorkflow(c.Context, c.String("file"), c.Bool("overwrite"))
		},
	}
}

func workflowExportCommand() *cli.Command {
	return &cli.Command{
		Name:  "export",
		Usage: "Export a workflow's spec to a file",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "slug",
				Usage:    "Workflow slug from the registry",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "file",
				Usage:    "Output path for the exported spec YAML file",
				Required: true,
			},
			&cli.BoolFlag{
				Name:  "overwrite",
				Usage: "Overwrite the target file if it already exists",
			},
		},
		Action: func(c *cli.Context) error {
			return exportWorkflow(c.Context, c.String("slug"), c.String("file"), c.Bool("overwrite"))
		},
	}
}

func workflowDeleteCommand() *cli.Command {
	return &cli.Command{
		Name:  "delete",
		Usage: "Delete a workflow from the registry",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "slug",
				Usage:    "Workflow slug from the registry",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			return deleteWorkflow(c.Context, c.String("slug"))
		},
	}
}

func workflowVersionsCommand() *cli.Command {
	return &cli.Command{
		Name:  "versions",
		Usage: "List a workflow's version history",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "slug",
				Usage:    "Workflow slug from the registry",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			return listWorkflowVersions(c.Context, c.String("slug"))
		},
	}
}

func workflowRestoreCommand() *cli.Command {
	return &cli.Command{
		Name:  "restore",
		Usage: "Restore a past version of a workflow as its new head version",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "slug",
				Usage:    "Workflow slug from the registry",
				Required: true,
			},
			&cli.IntFlag{
				Name:     "version",
				Usage:    "Version number to restore, from the versions command",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			return restoreWorkflowVersion(c.Context, c.String("slug"), c.Int("version"))
		},
	}
}

func runServer(ctx context.Context, shellRoot string) error {
	stack, err := core.NewStack(ctx, core.StackOptions{ShellRoot: shellRoot})
	if err != nil {
		return err
	}

	group, gctx := errgroup.WithContext(ctx)
	app := restserver.New(gctx, stack.Controller, stack.HooksAPI, stack.WorkflowAPI)

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

func runMCPServer(ctx context.Context, shellRoot string) error {
	stack, err := core.NewStack(ctx, core.StackOptions{
		ShellRoot: shellRoot,
		LogWriter: os.Stderr,
	})
	if err != nil {
		return err
	}
	defer stack.Controller.DB.Close() // nolint:errcheck // Close errors are not actionable here.

	srv := agcmcp.NewServer(ctx, stack, shellRoot)

	return mcpserver.ServeStdio(srv)
}

// newRegistry opens the application database and returns the workflow
// registry plus a closer for the connection.
func newRegistry() (*workflowruntime.Registry, func(), error) {
	ctrl, err := controller.New()
	if err != nil {
		return nil, nil, err
	}

	closer := func() {
		ctrl.DB.Close() // nolint:errcheck // Close errors are not actionable here.
	}

	return workflowruntime.NewRegistry(ctrl.DB), closer, nil
}

func listWorkflows(ctx context.Context) error {
	registry, closeDB, err := newRegistry()
	if err != nil {
		return err
	}
	defer closeDB()

	workflows, err := registry.List(ctx)
	if err != nil {
		return err
	}

	return printJSON(workflows)
}

func compileWorkflow(ctx context.Context, slug string, filePath string) error {
	registry, closeDB, err := newRegistry()
	if err != nil {
		return err
	}
	defer closeDB()

	spec, err := loadWorkflowSpec(ctx, registry, slug, filePath)
	if err != nil {
		return err
	}

	snapshot, err := registry.Compile(ctx, spec)
	if err != nil {
		return err
	}

	return printJSON(compileResult{
		Slug:        strings.TrimSpace(spec.Workflow.Slug),
		Version:     strings.TrimSpace(spec.Workflow.Version),
		Description: strings.TrimSpace(spec.Workflow.Description),
		Inputs:      workflowInputTypes(spec),
		Outputs:     workflowOutputTypes(spec),
		NodeCount:   len(snapshot.Nodes),
	})
}

func showWorkflow(ctx context.Context, slug string, filePath string) error {
	raw, err := loadWorkflowBytes(ctx, slug, filePath)
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(raw)
	if err != nil {
		return err
	}

	return nil
}

func importWorkflow(ctx context.Context, filePath string, overwrite bool) error {
	registry, closeDB, err := newRegistry()
	if err != nil {
		return err
	}
	defer closeDB()

	summary, err := registry.ImportFile(ctx, strings.TrimSpace(filePath), overwrite)
	if err != nil {
		return err
	}

	return printJSON(importResult{
		Slug:        summary.Slug,
		ID:          summary.ID,
		Version:     summary.Version,
		Description: summary.Description,
		Inputs:      summary.Inputs,
		Outputs:     summary.Outputs,
	})
}

func exportWorkflow(ctx context.Context, slug string, filePath string, overwrite bool) error {
	trimmedSlug := strings.TrimSpace(slug)
	trimmedFilePath := strings.TrimSpace(filePath)

	registry, closeDB, err := newRegistry()
	if err != nil {
		return err
	}
	defer closeDB()

	err = registry.ExportToFile(ctx, trimmedSlug, trimmedFilePath, overwrite)
	if err != nil {
		return err
	}

	return printJSON(exportResult{
		Slug: trimmedSlug,
		File: trimmedFilePath,
	})
}

func deleteWorkflow(ctx context.Context, slug string) error {
	trimmedSlug := strings.TrimSpace(slug)

	registry, closeDB, err := newRegistry()
	if err != nil {
		return err
	}
	defer closeDB()

	err = registry.Delete(ctx, trimmedSlug)
	if err != nil {
		return err
	}

	return printJSON(deleteResult{
		Slug:    trimmedSlug,
		Deleted: true,
	})
}

func listWorkflowVersions(ctx context.Context, slug string) error {
	registry, closeDB, err := newRegistry()
	if err != nil {
		return err
	}
	defer closeDB()

	versions, err := registry.ListVersions(ctx, strings.TrimSpace(slug))
	if err != nil {
		return err
	}

	return printJSON(versions)
}

func restoreWorkflowVersion(ctx context.Context, slug string, version int) error {
	registry, closeDB, err := newRegistry()
	if err != nil {
		return err
	}
	defer closeDB()

	restored, err := registry.RestoreVersion(ctx, strings.TrimSpace(slug), version)
	if err != nil {
		return err
	}

	return printJSON(restored)
}

func runMigrationByName(ctx context.Context, name string) error {
	ctrl, err := controller.New()
	if err != nil {
		return err
	}
	defer ctrl.DB.Close() // nolint:errcheck // Close errors are not actionable here.

	// Registered migrations contain PostgreSQL-specific SQL. A SQLite
	// database is always created from the current model schema, so it has
	// nothing to migrate.
	if ctrl.DB.Dialect().Name() != dialect.PG {
		return ez.New(ez.EINVALID, "Migrations only apply to PostgreSQL. SQLite databases are created with the current schema and never need them", nil)
	}

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

func runWorkflow(ctx context.Context, slug string, filePath string, inputFile string, inputJSON string, inputString string, hasInputFile bool, hasInputJSON bool, hasInputString bool, shellRoot string) error {
	stack, err := core.NewStack(ctx, core.StackOptions{ShellRoot: shellRoot})
	if err != nil {
		return err
	}

	defer stack.Controller.DB.Close() // nolint:errcheck // Close errors are not actionable here.

	input, err := loadWorkflowInput(
		ctx,
		stack.WorkflowAPI.Registry,
		strings.TrimSpace(slug),
		strings.TrimSpace(filePath),
		strings.TrimSpace(inputFile),
		strings.TrimSpace(inputJSON),
		inputString,
		hasInputFile,
		hasInputJSON,
		hasInputString,
	)
	if err != nil {
		return err
	}

	// Run synchronously: a detached run would die when the CLI process
	// exits, before the workflow finishes.
	response, err := stack.WorkflowAPI.Executions.Run(ctx, nil, &workflowexecutions.CreateRequest{
		WorkflowSlug: strings.TrimSpace(slug),
		File:         strings.TrimSpace(filePath),
		Input:        input,
		ShellRoot:    strings.TrimSpace(shellRoot),
	})
	if err != nil {
		return err
	}

	return printJSON(response)
}

func loadWorkflowInput(ctx context.Context, registry *workflowruntime.Registry, slug string, filePath string, inputFile string, inputJSON string, inputString string, hasInputFile bool, hasInputJSON bool, hasInputString bool) (map[string]any, error) {
	inputSourceCount := 0
	if hasInputFile {
		inputSourceCount++
	}
	if hasInputJSON {
		inputSourceCount++
	}
	if hasInputString {
		inputSourceCount++
	}

	if inputSourceCount == 0 {
		return nil, ez.New(ez.EINVALID, "one of --input-file, --input-json, or --input-string is required", nil)
	}
	if inputSourceCount > 1 {
		return nil, ez.New(ez.EINVALID, "only one of --input-file, --input-json, or --input-string may be used", nil)
	}

	if hasInputString {
		return loadWorkflowStringInput(ctx, registry, slug, filePath, inputString)
	}

	var raw []byte
	if inputFile != "" {
		content, err := os.ReadFile(inputFile)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		raw = content
	} else {
		raw = []byte(inputJSON)
	}

	var input map[string]any
	err := json.Unmarshal(raw, &input)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return input, nil
}

func loadWorkflowStringInput(ctx context.Context, registry *workflowruntime.Registry, slug string, filePath string, inputString string) (map[string]any, error) {
	spec, err := loadWorkflowSpec(ctx, registry, slug, filePath)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if len(spec.Workflow.Inputs) != 1 {
		return nil, ez.New(ez.EINVALID, "--input-string requires a workflow with exactly one top-level input", nil)
	}

	for inputName, typeRef := range spec.Workflow.Inputs {
		if strings.TrimSpace(typeRef) != "string" {
			return nil, ez.New(ez.EINVALID, "--input-string requires the workflow input type to be string", nil)
		}

		return map[string]any{
			inputName: inputString,
		}, nil
	}

	return nil, ez.New(ez.EINTERNAL, "workflow input declaration is missing", nil)
}

func loadWorkflowSpec(ctx context.Context, registry *workflowruntime.Registry, slug string, filePath string) (*workflowruntime.Spec, error) {
	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug != "" {
		spec, err := registry.Load(ctx, trimmedSlug)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		return spec, nil
	}

	trimmedFilePath := strings.TrimSpace(filePath)
	if trimmedFilePath == "" {
		return nil, ez.New(ez.EINVALID, "one of --slug or --file is required", nil)
	}

	spec, err := workflowruntime.LoadSpecFile(trimmedFilePath)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return spec, nil
}

func loadWorkflowBytes(ctx context.Context, slug string, filePath string) ([]byte, error) {
	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug != "" {
		registry, closeDB, err := newRegistry()
		if err != nil {
			return nil, ez.Wrap(err)
		}
		defer closeDB()

		raw, err := registry.SpecBytes(ctx, trimmedSlug)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		return raw, nil
	}

	trimmedFilePath := strings.TrimSpace(filePath)
	if trimmedFilePath == "" {
		return nil, ez.New(ez.EINVALID, "one of --slug or --file is required", nil)
	}

	raw, err := os.ReadFile(trimmedFilePath)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return raw, nil
}

func workflowInputTypes(spec *workflowruntime.Spec) map[string]string {
	inputs := make(map[string]string, len(spec.Workflow.Inputs))
	for inputName, typeRef := range spec.Workflow.Inputs {
		inputs[inputName] = strings.TrimSpace(typeRef)
	}

	return inputs
}

func workflowOutputTypes(spec *workflowruntime.Spec) map[string]string {
	outputs := make(map[string]string, len(spec.Workflow.Outputs))
	for outputName, outputSpec := range spec.Workflow.Outputs {
		outputs[outputName] = strings.TrimSpace(outputSpec.Schema)
	}

	return outputs
}

func printJSON(value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(encoded))

	return nil
}
