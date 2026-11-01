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
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/task"
)

func taskCommand() *cli.Command {
	return &cli.Command{
		Name:   "task",
		Usage:  "Task commands: isolated VMs agents work in (ADR 0003)",
		Before: quietLogs,
		Subcommands: []*cli.Command{
			{
				Name:  "create",
				Usage: "Create and boot a task from a task.yaml or a saved spec",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Path to the task.yaml"},
					&cli.StringFlag{Name: "from", Usage: "Name of a saved spec (agc task spec ls)"},
					&cli.StringFlag{Name: "title", Usage: "Task title (defaults to title: in the file)"},
					&cli.StringFlag{Name: "name", Usage: "Task name (defaults to name: in the file, else made from the title)"},
				},
				Action: taskCreate,
			},
			{
				Name:  "spec",
				Usage: "Saved specs: reusable task.yaml files in ~/.agent_composer/config/specs",
				Subcommands: []*cli.Command{
					{
						Name:    "ls",
						Aliases: []string{"list"},
						Usage:   "List saved specs",
						Action:  taskSpecList,
					},
					{
						Name:      "save",
						Usage:     "Save a task.yaml as a spec, replacing one of the same name",
						ArgsUsage: "<name> -f <file>",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Path to the task.yaml"},
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
				Name:    "ls",
				Aliases: []string{"list"},
				Usage:   "List tasks with their live status",
				Action:  taskList,
			},
			{
				Name:      "get",
				Usage:     "Show a task with its live status",
				ArgsUsage: "<name>",
				Action:    taskGet,
			},
			{
				Name:      "stop",
				Usage:     "Stop a task and keep its disk",
				ArgsUsage: "<name>",
				Action:    taskStop,
			},
			{
				Name:      "start",
				Usage:     "Boot a stopped task from its disk",
				ArgsUsage: "<name>",
				Action:    taskStart,
			},
			{
				Name:      "rm",
				Usage:     "Delete a task: its sandbox disk and uncommitted work; volumes and messages are kept",
				ArgsUsage: "<name>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "Do not ask for confirmation"},
				},
				Action: taskRemove,
			},
		},
	}
}

// withTasks runs fn against an in-process stack, like workflow run.
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

// taskArg returns the single task name. urfave/cli stops reading flags at
// the first argument, so a --yes written after the name is picked out here.
func taskArg(c *cli.Context) (string, bool, error) {
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
		return "", false, ez.New(ez.EINVALID, "usage: agc task "+c.Command.Name+" <name>", nil)
	}

	return names[0], yes, nil
}

func taskCreate(c *cli.Context) error {
	filePath := c.String("file")
	from := strings.TrimSpace(c.String("from"))
	if (filePath == "") == (from == "") {
		return ez.New(ez.EINVALID, "pass exactly one of -f <file> or --from <saved spec>", nil)
	}

	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		parsed, err := loadTaskFile(ctx, api, filePath, from)
		if err != nil {
			return nil, err
		}

		title := strings.TrimSpace(c.String("title"))
		if title == "" {
			title = parsed.Title
		}

		name := strings.TrimSpace(c.String("name"))
		if name == "" {
			name = parsed.Name
		}

		if title == "" && name == "" {
			return nil, ez.New(ez.EINVALID, "the task needs a title: pass --title or set title: in the file", nil)
		}

		label := name
		if label == "" {
			label = title
		}
		fmt.Fprintf(os.Stderr, "Creating task %s…\n", label)

		return api.Create(ctx, nil, &tasks.CreateRequest{Title: title, Name: name, Spec: parsed.Spec})
	})
}

// loadTaskFile reads the task.yaml at filePath, or the saved spec called
// from when filePath is empty.
func loadTaskFile(ctx context.Context, api *tasks.API, filePath string, from string) (task.File, error) {
	if filePath != "" {
		return task.LoadFile(filePath)
	}

	saved, err := api.GetSpec(ctx, nil, &tasks.GetSpecRequest{Name: from})
	if err != nil {
		return task.File{}, err
	}

	parsed, err := task.ParseFile([]byte(saved.YAML))
	if err != nil {
		return task.File{}, ez.New(ez.EINVALID, "saved spec "+from+": "+ez.ErrorMessage(err), err)
	}

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
		return ez.New(ez.EINVALID, "cannot read task file "+filePath, err)
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
	name, _, err := taskArg(c)
	if err != nil {
		return err
	}

	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		return api.Get(ctx, nil, &tasks.GetRequest{Name: name})
	})
}

func taskStop(c *cli.Context) error {
	name, _, err := taskArg(c)
	if err != nil {
		return err
	}

	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		return api.Stop(ctx, nil, &tasks.StopRequest{Name: name})
	})
}

func taskStart(c *cli.Context) error {
	name, _, err := taskArg(c)
	if err != nil {
		return err
	}

	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		return api.Start(ctx, nil, &tasks.StartRequest{Name: name})
	})
}

func taskRemove(c *cli.Context) error {
	name, yes, err := taskArg(c)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Deleting task %s deletes the sandbox disk and uncommitted work; volumes and messages are kept.\n", name)

	if !yes {
		fmt.Fprint(os.Stderr, "Delete it? [y/N] ")

		// A read error such as EOF leaves no answer, which is a no.
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			return ez.New(ez.EINVALID, "task "+name+" was not deleted: answer y or pass --yes", nil)
		}
	}

	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		return api.Delete(ctx, nil, &tasks.DeleteRequest{Name: name})
	})
}
