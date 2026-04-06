package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog/log"
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

const version = "0.2.15"

type compileResult struct {
	ID          string            `json:"id"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Inputs      map[string]string `json:"inputs"`
	Outputs     map[string]string `json:"outputs"`
	NodeCount   int               `json:"node_count"`
}

type importResult struct {
	ID          string            `json:"id"`
	File        string            `json:"file"`
	Description string            `json:"description,omitempty"`
	Inputs      map[string]string `json:"inputs"`
	Outputs     map[string]string `json:"outputs"`
}

type exportResult struct {
	ID   string `json:"id"`
	File string `json:"file"`
}

type deleteResult struct {
	ID      string `json:"id"`
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
		Usage:   "Workflow blueprint commands",
		Subcommands: []*cli.Command{
			workflowRunCommand(),
			workflowCompileCommand(),
			workflowListCommand(),
			workflowShowCommand(),
			workflowImportCommand(),
			workflowExportCommand(),
			workflowDeleteCommand(),
		},
	}
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
			&cli.StringFlag{
				Name:  "input-string",
				Usage: "Inline raw string for workflows with exactly one top-level string input",
			},
		},
		Action: func(c *cli.Context) error {
			return runWorkflow(
				c.Context,
				c.String("id"),
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
		Usage: "List installed workflow blueprints",
		Action: func(c *cli.Context) error {
			return listWorkflows()
		},
	}
}

func workflowShowCommand() *cli.Command {
	return &cli.Command{
		Name:  "show",
		Usage: "Show the raw YAML contents of a workflow blueprint from the registry or disk",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "id",
				Usage: "Workflow blueprint id from the workflow registry",
			},
			&cli.StringFlag{
				Name:  "file",
				Usage: "Path to the workflow blueprint YAML file",
			},
		},
		Action: func(c *cli.Context) error {
			return showWorkflow(c.String("id"), c.String("file"))
		},
	}
}

func workflowCompileCommand() *cli.Command {
	return &cli.Command{
		Name:  "compile",
		Usage: "Compile a workflow blueprint from the registry or disk without running it",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "id",
				Usage: "Workflow blueprint id from the workflow registry",
			},
			&cli.StringFlag{
				Name:  "file",
				Usage: "Path to the workflow blueprint YAML file",
			},
		},
		Action: func(c *cli.Context) error {
			return compileWorkflow(c.String("id"), c.String("file"))
		},
	}
}

func workflowImportCommand() *cli.Command {
	return &cli.Command{
		Name:  "import",
		Usage: "Import a workflow blueprint file into the workflow registry",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Usage:    "Path to the workflow blueprint YAML file",
				Required: true,
			},
			&cli.BoolFlag{
				Name:  "overwrite",
				Usage: "Overwrite an existing workflow with the same id",
			},
		},
		Action: func(c *cli.Context) error {
			return importWorkflow(c.String("file"), c.Bool("overwrite"))
		},
	}
}

func workflowExportCommand() *cli.Command {
	return &cli.Command{
		Name:  "export",
		Usage: "Export a workflow blueprint from the registry to a file path",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "id",
				Usage:    "Workflow blueprint id from the workflow registry",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "file",
				Usage:    "Output path for the exported workflow blueprint YAML file",
				Required: true,
			},
			&cli.BoolFlag{
				Name:  "overwrite",
				Usage: "Overwrite the target file if it already exists",
			},
		},
		Action: func(c *cli.Context) error {
			return exportWorkflow(c.String("id"), c.String("file"), c.Bool("overwrite"))
		},
	}
}

func workflowDeleteCommand() *cli.Command {
	return &cli.Command{
		Name:  "delete",
		Usage: "Delete a workflow blueprint from the workflow registry",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "id",
				Usage:    "Workflow blueprint id from the workflow registry",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			return deleteWorkflow(c.String("id"))
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

func listWorkflows() error {
	workflows, err := workflowruntime.ListBlueprints()
	if err != nil {
		return err
	}

	return printJSON(workflows)
}

func compileWorkflow(workflowID string, filePath string) error {
	blueprint, err := loadWorkflowBlueprint(workflowID, filePath)
	if err != nil {
		return err
	}

	snapshot, err := workflowruntime.Compile(blueprint)
	if err != nil {
		return err
	}

	return printJSON(compileResult{
		ID:          strings.TrimSpace(blueprint.Workflow.ID),
		Version:     strings.TrimSpace(blueprint.Workflow.Version),
		Description: strings.TrimSpace(blueprint.Workflow.Description),
		Inputs:      workflowInputTypes(blueprint),
		Outputs:     workflowOutputTypes(blueprint),
		NodeCount:   len(snapshot.Nodes),
	})
}

func showWorkflow(workflowID string, filePath string) error {
	raw, err := loadWorkflowBytes(workflowID, filePath)
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(raw)
	if err != nil {
		return err
	}

	return nil
}

func importWorkflow(filePath string, overwrite bool) error {
	summary, err := workflowruntime.ImportBlueprintFile(strings.TrimSpace(filePath), overwrite)
	if err != nil {
		return err
	}

	workflowDir, err := workflowruntime.ResolveWorkflowDir()
	if err != nil {
		return err
	}

	return printJSON(importResult{
		ID:          summary.ID,
		File:        filepath.Join(workflowDir, summary.ID+".yaml"),
		Description: summary.Description,
		Inputs:      summary.Inputs,
		Outputs:     summary.Outputs,
	})
}

func exportWorkflow(workflowID string, filePath string, overwrite bool) error {
	trimmedWorkflowID := strings.TrimSpace(workflowID)
	trimmedFilePath := strings.TrimSpace(filePath)

	err := workflowruntime.ExportBlueprintByWorkflowID(trimmedWorkflowID, trimmedFilePath, overwrite)
	if err != nil {
		return err
	}

	return printJSON(exportResult{
		ID:   trimmedWorkflowID,
		File: trimmedFilePath,
	})
}

func deleteWorkflow(workflowID string) error {
	trimmedWorkflowID := strings.TrimSpace(workflowID)
	err := workflowruntime.DeleteBlueprintByWorkflowID(trimmedWorkflowID)
	if err != nil {
		return err
	}

	return printJSON(deleteResult{
		ID:      trimmedWorkflowID,
		Deleted: true,
	})
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

func runWorkflow(ctx context.Context, workflowID string, filePath string, inputFile string, inputJSON string, inputString string, hasInputFile bool, hasInputJSON bool, hasInputString bool, shellRoot string) error {
	input, err := loadWorkflowInput(
		strings.TrimSpace(workflowID),
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

	stack, err := core.NewStack(ctx, core.StackOptions{ShellRoot: shellRoot})
	if err != nil {
		return err
	}

	defer stack.Controller.DB.Close() // nolint:errcheck // Close errors are not actionable here.

	response, err := stack.WorkflowAPI.Executions.Create(ctx, nil, &workflowexecutions.CreateRequest{
		WorkflowID: strings.TrimSpace(workflowID),
		File:       strings.TrimSpace(filePath),
		Input:      input,
		ShellRoot:  strings.TrimSpace(shellRoot),
	})
	if err != nil {
		return err
	}

	return printJSON(response.Output)
}

func loadWorkflowInput(workflowID string, filePath string, inputFile string, inputJSON string, inputString string, hasInputFile bool, hasInputJSON bool, hasInputString bool) (map[string]any, error) {
	const op = "cli.loadWorkflowInput"

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
		return nil, ez.New(op, ez.EINVALID, "one of --input-file, --input-json, or --input-string is required", nil)
	}
	if inputSourceCount > 1 {
		return nil, ez.New(op, ez.EINVALID, "only one of --input-file, --input-json, or --input-string may be used", nil)
	}

	if hasInputString {
		return loadWorkflowStringInput(workflowID, filePath, inputString)
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

func loadWorkflowStringInput(workflowID string, filePath string, inputString string) (map[string]any, error) {
	const op = "cli.loadWorkflowStringInput"

	blueprint, err := loadWorkflowBlueprint(workflowID, filePath)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	if len(blueprint.Workflow.Inputs) != 1 {
		return nil, ez.New(op, ez.EINVALID, "--input-string requires a workflow with exactly one top-level input", nil)
	}

	for inputName, typeRef := range blueprint.Workflow.Inputs {
		if strings.TrimSpace(typeRef) != "string" {
			return nil, ez.New(op, ez.EINVALID, "--input-string requires the workflow input type to be string", nil)
		}

		return map[string]any{
			inputName: inputString,
		}, nil
	}

	return nil, ez.New(op, ez.EINTERNAL, "workflow input declaration is missing", nil)
}

func loadWorkflowBlueprint(workflowID string, filePath string) (*workflowruntime.Blueprint, error) {
	const op = "cli.loadWorkflowBlueprint"

	trimmedWorkflowID := strings.TrimSpace(workflowID)
	if trimmedWorkflowID != "" {
		blueprint, err := workflowruntime.LoadBlueprintByWorkflowID(trimmedWorkflowID)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		return blueprint, nil
	}

	trimmedFilePath := strings.TrimSpace(filePath)
	if trimmedFilePath == "" {
		return nil, ez.New(op, ez.EINVALID, "one of --id or --file is required", nil)
	}

	blueprint, err := workflowruntime.LoadBlueprintFile(trimmedFilePath)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return blueprint, nil
}

func loadWorkflowBytes(workflowID string, filePath string) ([]byte, error) {
	const op = "cli.loadWorkflowBytes"

	trimmedWorkflowID := strings.TrimSpace(workflowID)
	if trimmedWorkflowID != "" {
		raw, err := workflowruntime.ReadBlueprintBytesByWorkflowID(trimmedWorkflowID)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		return raw, nil
	}

	trimmedFilePath := strings.TrimSpace(filePath)
	if trimmedFilePath == "" {
		return nil, ez.New(op, ez.EINVALID, "one of --id or --file is required", nil)
	}

	raw, err := os.ReadFile(trimmedFilePath)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return raw, nil
}

func workflowInputTypes(blueprint *workflowruntime.Blueprint) map[string]string {
	inputs := make(map[string]string, len(blueprint.Workflow.Inputs))
	for inputName, typeRef := range blueprint.Workflow.Inputs {
		inputs[inputName] = strings.TrimSpace(typeRef)
	}

	return inputs
}

func workflowOutputTypes(blueprint *workflowruntime.Blueprint) map[string]string {
	outputs := make(map[string]string, len(blueprint.Workflow.Outputs))
	for outputName, outputSpec := range blueprint.Workflow.Outputs {
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
