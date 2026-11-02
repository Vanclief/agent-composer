package cli

import (
	"context"
	"fmt"
	"os"

	cli "github.com/urfave/cli/v2"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
)

func sandboxCommand() *cli.Command {
	return &cli.Command{
		Name:   "sandbox",
		Usage:  "Sandbox commands: the VMs tasks are allocated (ADR 0008)",
		Before: quietLogs,
		Subcommands: []*cli.Command{
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List sandboxes with their task and live status",
				Action:  sandboxList,
			},
			{
				Name:      "stop",
				Usage:     "Stop a sandbox and keep its disk",
				ArgsUsage: "<name>",
				Action:    sandboxStop,
			},
			{
				Name:      "start",
				Usage:     "Boot a stopped sandbox from its disk",
				ArgsUsage: "<name>",
				Action:    sandboxStart,
			},
			{
				Name:      "delete",
				Aliases:   []string{"rm"},
				Usage:     "Delete a sandbox: its disk and uncommitted work; volumes and messages are kept",
				ArgsUsage: "<name>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "Do not ask for confirmation"},
				},
				Action: sandboxDelete,
			},
		},
	}
}

func withSandboxes(c *cli.Context, fn func(ctx context.Context, api *sandboxes.API) (any, error)) error {
	return withTasks(c, func(ctx context.Context, api *tasks.API) (any, error) {
		return fn(ctx, api.Sandboxes)
	})
}

func sandboxList(c *cli.Context) error {
	return withSandboxes(c, func(ctx context.Context, api *sandboxes.API) (any, error) {
		return api.List(ctx, nil, &sandboxes.ListRequest{})
	})
}

func sandboxStop(c *cli.Context) error {
	name, _, err := refArg(c, "sandbox")
	if err != nil {
		return err
	}

	return withSandboxes(c, func(ctx context.Context, api *sandboxes.API) (any, error) {
		return api.Stop(ctx, nil, &sandboxes.StopRequest{Name: name})
	})
}

func sandboxStart(c *cli.Context) error {
	name, _, err := refArg(c, "sandbox")
	if err != nil {
		return err
	}

	return withSandboxes(c, func(ctx context.Context, api *sandboxes.API) (any, error) {
		return api.Start(ctx, nil, &sandboxes.StartRequest{Name: name})
	})
}

func sandboxDelete(c *cli.Context) error {
	name, yes, err := refArg(c, "sandbox")
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Deleting sandbox %s deletes its disk and uncommitted work; volumes and messages are kept.\n", name)

	if !yes {
		err = confirm("sandbox " + name)
		if err != nil {
			return err
		}
	}

	return withSandboxes(c, func(ctx context.Context, api *sandboxes.API) (any, error) {
		return api.Delete(ctx, nil, &sandboxes.DeleteRequest{Name: name})
	})
}
