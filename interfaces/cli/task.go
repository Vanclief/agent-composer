package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core"
	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/task"
)

func taskCommand() *cli.Command {
	return &cli.Command{
		Name:   "task",
		Usage:  "Task commands: work to be done, allocated compute as sandboxes (ADR 0008)",
		Before: quietLogs,
		Subcommands: []*cli.Command{
			{
				Name:  "create",
				Usage: "Create a task from a task.yaml, booting its sandbox: when it has one",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Path to the task.yaml"},
					&cli.StringFlag{Name: "from", Usage: "Name of a saved spec to boot as the task's sandbox (agc task spec ls)"},
					&cli.StringFlag{Name: "title", Usage: "Task title (defaults to title: in the file)"},
					&cli.StringFlag{Name: "name", Usage: "Task name (defaults to name: in the file, else made from the title)"},
					&cli.StringFlag{Name: "brief", Usage: "What to do and what done looks like (defaults to brief: in the file)"},
				},
				Action: taskCreate,
			},
			{
				Name:  "spec",
				Usage: "Saved specs: reusable sandbox specs in ~/.agent_composer/config/specs",
				Subcommands: []*cli.Command{
					{
						Name:    "ls",
						Aliases: []string{"list"},
						Usage:   "List saved specs",
						Action:  taskSpecList,
					},
					{
						Name:      "save",
						Usage:     "Save a sandbox spec, replacing one of the same name",
						ArgsUsage: "<name> -f <file>",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Path to the sandbox spec"},
						},
						Action: taskSpecSave,
					},
					{
						Name:      "rm",
						Usage:     "Delete a saved spec",
						ArgsUsage: "<name>",
						Action:    taskSpecRemove,
					},
				},
			},
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List tasks with their status and sandboxes",
				Action:  taskList,
			},
			{
				Name:      "get",
				Usage:     "Show a task with its status and sandboxes",
				ArgsUsage: "<name>",
				Action:    taskGet,
			},
			{
				Name:      "done",
				Usage:     "Mark a task done; its sandboxes are left as they are",
				ArgsUsage: "<name>",
				Action:    taskDone,
			},
			{
				Name:      "delete",
				Aliases:   []string{"rm"},
				Usage:     "Delete a task and its sandboxes: their disks and uncommitted work; volumes and messages are kept",
				ArgsUsage: "<name>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "Do not ask for confirmation"},
				},
				Action: taskDelete,
			},
		},
	}
}

// withTasks runs fn against an in-process stack, like workflow run. A
// sandbox boots in this process, so fn waits for it before returning.
func withTasks(c *cli.Context, fn func(ctx context.Context, api *tasks.API) (any, error)) error {
	stack, err := core.NewStack(c.Context, core.StackOptions{ProjectDir: c.String("project")})
	if err != nil {
		return err
	}
	defer stack.Controller.DB.Close() // nolint:errcheck // Close errors are not actionable here.

	result, err := fn(c.Context, stack.TasksAPI)
	if err != nil {
		return err
	}

	return printJSON(result)
}

// refArg returns the single name argument. urfave/cli stops reading flags
// at the first argument, so a --yes written after the name is picked out
// here.
func refArg(c *cli.Context, command string) (string, bool, error) {
	yes := c.Bool("yes")
	names := []string{}
	for _, arg := range c.Args().Slice() {
		if arg == "--yes" || arg == "-y" {
			yes = true
			continue
		}
		names = append(names, arg)
	}

	if len(names) != 1 {
		return "", false, ez.New(ez.EINVALID, "usage: agc "+command+" "+c.Command.Name+" <name>", nil)
	}

	return names[0], yes, nil
}

// confirm asks on stderr and returns an error unless the answer is yes.
func confirm(what string) error {
	fmt.Fprint(os.Stderr, "Delete it? [y/N] ")

	// A read error such as EOF leaves no answer, which is a no.
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		return ez.New(ez.EINVALID, what+" was not deleted: answer y or pass --yes", nil)
	}

	return nil
}

func taskCreate(c *cli.Context) error {
	filePath := c.String("file")
	from := strings.TrimSpace(c.String("from"))

	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		parsed, err := loadTaskFile(ctx, api, filePath, from)
		if err != nil {
			return nil, err
		}

		request := &tasks.CreateRequest{
			Title:    firstNonEmpty(c.String("title"), parsed.Title),
			Name:     firstNonEmpty(c.String("name"), parsed.Name),
			Brief:    firstNonEmpty(c.String("brief"), parsed.Brief),
			Assignee: parsed.Assignee,
			Model:    parsed.Model,
			Manager:  parsed.Manager,
			Owner:    parsed.Owner,
			For:      parsed.For,
			Sandbox:  parsed.Sandbox,
		}

		if request.Title == "" && request.Name == "" {
			return nil, ez.New(ez.EINVALID, "the task needs a title: pass --title or set title: in the file", nil)
		}

		applied, err := fillTaskDefaults(request, api.Defaults())
		if err != nil {
			return nil, err
		}

		if len(applied) > 0 {
			fmt.Fprintf(os.Stderr, "Defaults applied: %s\n", strings.Join(applied, ", "))
		}

		created, err := api.Create(ctx, nil, request)
		if err != nil {
			return nil, err
		}

		if len(created.Sandboxes) == 0 {
			return created, nil
		}

		fmt.Fprintf(os.Stderr, "Task %s created, booting sandbox %s…\n", created.Name, created.Sandboxes[0].Name)
		api.Sandboxes.Wait()

		return api.Get(ctx, nil, &tasks.GetRequest{ID: created.ID})
	})
}

// fillTaskDefaults fills in what the file leaves empty and says what it
// filled: the manager is the caller's messaging name (agc msg join), else
// the configured owner; assignee, model and owner come from the defaults
// (ADR 0009 item 1).
func fillTaskDefaults(request *tasks.CreateRequest, defaults controller.DefaultsSettings) ([]string, error) {
	applied := []string{}

	if strings.TrimSpace(request.Manager) == "" {
		state, err := loadMsgState()
		if err == nil && state.Name != "" {
			request.Manager = state.Name
			applied = append(applied, "manager "+state.Name+" (your messaging name)")
		} else if defaults.Owner != "" {
			request.Manager = defaults.Owner
			applied = append(applied, "manager "+defaults.Owner+" (the configured owner)")
		} else {
			msg := "the task needs a manager: set manager: in the file, run agc msg join <name>, or configure defaults.owner"
			return nil, ez.New(ez.EINVALID, msg, nil)
		}
	}

	filled, err := tasks.ApplyDefaults(request, defaults)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(request.Assignee) == "" {
		applied = append(applied, "assignee "+filled.Assignee)
	}
	if strings.TrimSpace(request.Model) == "" && filled.Model != "" {
		applied = append(applied, "model "+filled.Model)
	}
	if strings.TrimSpace(request.Owner) == "" {
		owner := filled.Owner
		if owner == "" {
			owner = "none (no owner is configured)"
		}
		applied = append(applied, "owner "+owner)
	}

	request.Assignee = filled.Assignee
	request.Model = filled.Model
	request.Owner = filled.Owner

	return applied, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

// loadTaskFile reads the task.yaml at filePath, and the saved spec called
// from as its sandbox. Either may be empty, not both.
func loadTaskFile(ctx context.Context, api *tasks.API, filePath string, from string) (task.File, error) {
	if filePath == "" && from == "" {
		return task.File{}, ez.New(ez.EINVALID, "pass -f <file>, --from <saved spec>, or both", nil)
	}

	parsed := task.File{}

	if filePath != "" {
		loaded, err := task.LoadFile(filePath)
		if err != nil {
			return task.File{}, err
		}
		parsed = loaded
	}

	if from == "" {
		return parsed, nil
	}

	if parsed.Sandbox != nil {
		return task.File{}, ez.New(ez.EINVALID, "the file already has a sandbox: drop it or --from", nil)
	}

	saved, err := api.GetSpec(ctx, nil, &tasks.GetSpecRequest{Name: from})
	if err != nil {
		return task.File{}, err
	}

	sandbox, err := task.ParseSandbox([]byte(saved.YAML))
	if err != nil {
		return task.File{}, ez.New(ez.EINVALID, "saved spec "+from+": "+ez.ErrorMessage(err), err)
	}
	parsed.Sandbox = &sandbox

	return parsed, nil
}

func taskSpecList(c *cli.Context) error {
	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		return api.ListSpecs(ctx, nil, &tasks.ListSpecsRequest{})
	})
}

// taskSpecSave takes -f before or after the name, since urfave/cli stops
// reading flags at the first argument.
func taskSpecSave(c *cli.Context) error {
	filePath := c.String("file")
	names := []string{}
	args := c.Args().Slice()
	for i := 0; i < len(args); i++ {
		if (args[i] == "-f" || args[i] == "--file") && i+1 < len(args) {
			filePath = args[i+1]
			i++
			continue
		}
		names = append(names, args[i])
	}

	if len(names) != 1 || filePath == "" {
		return ez.New(ez.EINVALID, "usage: agc task spec save <name> -f <file>", nil)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return ez.New(ez.EINVALID, "cannot read spec file "+filePath, err)
	}

	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		return api.SaveSpec(ctx, nil, &tasks.SaveSpecRequest{Name: names[0], YAML: string(data)})
	})
}

func taskSpecRemove(c *cli.Context) error {
	if c.NArg() != 1 {
		return ez.New(ez.EINVALID, "usage: agc task spec rm <name>", nil)
	}
	name := c.Args().First()

	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		return api.DeleteSpec(ctx, nil, &tasks.DeleteSpecRequest{Name: name})
	})
}

func taskList(c *cli.Context) error {
	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		return api.List(ctx, nil, &tasks.ListRequest{})
	})
}

func taskGet(c *cli.Context) error {
	name, _, err := refArg(c, "task")
	if err != nil {
		return err
	}

	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		return api.Get(ctx, nil, &tasks.GetRequest{Name: name})
	})
}

func taskDone(c *cli.Context) error {
	name, _, err := refArg(c, "task")
	if err != nil {
		return err
	}

	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		return api.Done(ctx, nil, &tasks.DoneRequest{Name: name})
	})
}

func taskDelete(c *cli.Context) error {
	name, yes, err := refArg(c, "task")
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Deleting task %s deletes its sandboxes' disks and uncommitted work; volumes and messages are kept.\n", name)

	if !yes {
		err = confirm("task " + name)
		if err != nil {
			return err
		}
	}

	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		return api.Delete(ctx, nil, &tasks.DeleteRequest{Name: name})
	})
}
