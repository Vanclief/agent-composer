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
				Usage: "Create and boot a task from a task.yaml",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Path to the task.yaml", Required: true},
					&cli.StringFlag{Name: "name", Usage: "Task name (defaults to name: in the file)"},
				},
				Action: taskCreate,
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
	fileName, spec, err := task.LoadFile(c.String("file"))
	if err != nil {
		return err
	}

	name := strings.TrimSpace(c.String("name"))
	if name == "" {
		name = fileName
	}
	if name == "" {
		return ez.New(ez.EINVALID, "the task needs a name: pass --name or set name: in the file", nil)
	}

	fmt.Fprintf(os.Stderr, "Creating task %s…\n", name)

	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		return api.Create(ctx, nil, &tasks.CreateRequest{Name: name, Spec: spec})
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
