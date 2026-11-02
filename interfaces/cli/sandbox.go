package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/task"
)

func sandboxCommand() *cli.Command {
	return withRemoteFlags(&cli.Command{
		Name:   "sandbox",
		Usage:  "Sandbox commands: the VMs tasks are allocated (ADR 0008); --server or $AGC_SERVER calls a remote agc",
		Before: quietLogs,
		Subcommands: []*cli.Command{
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List sandboxes with their task and live status",
				Action:  sandboxList,
			},
			{
				Name:      "get",
				Usage:     "Show a sandbox with its task and live status",
				ArgsUsage: "<name>",
				Action:    sandboxGet,
			},
			{
				Name:      "allocate",
				Usage:     "Allocate a sandbox to a task from a sandbox spec file",
				ArgsUsage: "<task> -f <sandbox.yaml>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Path to the sandbox spec: the environment block plus command"},
					&cli.BoolFlag{Name: "wait", Usage: "Against a server, poll until the sandbox is running or failed (locally allocate always waits)"},
				},
				Action: sandboxAllocate,
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
			{
				Name:  "gc",
				Usage: "Delete the stopped sandboxes of tasks done longer than keep_done ago (default 24h); volumes and messages are kept",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "dry-run", Usage: "Only list what would be deleted"},
					&cli.StringFlag{Name: "keep", Usage: "Grace period after the task is done, such as 48h (default keep_done)"},
				},
				Action: sandboxGC,
			},
		},
	})
}

func sandboxList(c *cli.Context) error {
	err := noArgs(c)
	if err != nil {
		return err
	}

	return withService(c, func(ctx context.Context, svc taskService) error {
		list, err := svc.ListSandboxes(ctx)
		if err != nil {
			return err
		}

		return output(c, list, func(w io.Writer) {
			writeSandboxes(w, list)
		})
	})
}

// sandboxCall runs one call on the named sandbox and prints the view.
func sandboxCall(c *cli.Context, call func(ctx context.Context, svc taskService, name string) (*sandboxes.SandboxView, error)) error {
	name, _, err := refArg(c, "sandbox")
	if err != nil {
		return err
	}

	return withService(c, func(ctx context.Context, svc taskService) error {
		view, err := call(ctx, svc, name)
		if err != nil {
			return err
		}

		return output(c, view, func(w io.Writer) {
			writeSandbox(w, view)
		})
	})
}

func sandboxGet(c *cli.Context) error {
	return sandboxCall(c, func(ctx context.Context, svc taskService, name string) (*sandboxes.SandboxView, error) {
		return svc.GetSandbox(ctx, name)
	})
}

func sandboxStop(c *cli.Context) error {
	return sandboxCall(c, func(ctx context.Context, svc taskService, name string) (*sandboxes.SandboxView, error) {
		return svc.StopSandbox(ctx, name)
	})
}

func sandboxStart(c *cli.Context) error {
	return sandboxCall(c, func(ctx context.Context, svc taskService, name string) (*sandboxes.SandboxView, error) {
		return svc.StartSandbox(ctx, name)
	})
}

// sandboxAllocate allocates a sandbox to the task. Like task create, it
// waits for the boot locally and, against a server, only with --wait.
func sandboxAllocate(c *cli.Context) error {
	names, _, err := leafArgs(c)
	if err != nil {
		return err
	}

	filePath := c.String("file")
	if len(names) != 1 || filePath == "" {
		return ez.New(ez.EINVALID, "usage: agc sandbox allocate <task> -f <sandbox.yaml>", nil)
	}
	taskName := names[0]

	data, err := os.ReadFile(filePath)
	if err != nil {
		return ez.New(ez.EINVALID, "cannot read sandbox file "+filePath, err)
	}

	spec, err := task.ParseSandbox(data)
	if err != nil {
		return ez.New(ez.EINVALID, filePath+": "+ez.ErrorMessage(err), err)
	}

	server := serverURL(c)

	return withService(c, func(ctx context.Context, svc taskService) error {
		allocated, err := svc.Allocate(ctx, taskName, spec)
		if err != nil {
			return err
		}

		if server != "" && !c.Bool("wait") {
			return output(c, allocated, func(w io.Writer) {
				writeSandbox(w, allocated)
			})
		}

		fmt.Fprintf(os.Stderr, "Sandbox %s allocated to task %s, booting…\n", allocated.Name, taskName)

		booted, err := svc.WaitSandbox(ctx, allocated.Name, func(view *sandboxes.SandboxView) {
			if server != "" {
				fmt.Fprintf(os.Stderr, "sandbox %s: %s\n", view.Name, sandboxStatus(*view))
			}
		})
		if err != nil {
			return err
		}

		err = output(c, booted, func(w io.Writer) {
			writeSandbox(w, booted)
		})
		if err != nil {
			return err
		}

		if server != "" && booted.Status == sandboxes.StatusFailed {
			return ez.New(ez.EINTERNAL, "sandbox "+booted.Name+" failed to boot: "+booted.StatusError, nil)
		}

		return nil
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

	return withService(c, func(ctx context.Context, svc taskService) error {
		deleted, err := svc.DeleteSandbox(ctx, name)
		if err != nil {
			return err
		}

		return output(c, deleted, func(w io.Writer) {
			fmt.Fprintf(w, "Deleted sandbox %s\n", deleted.Name)
		})
	})
}

// sandboxGC runs the collector by hand (ADR 0008 "Garbage collection").
func sandboxGC(c *cli.Context) error {
	err := noArgs(c)
	if err != nil {
		return err
	}

	request := &sandboxes.GCRequest{DryRun: c.Bool("dry-run"), Keep: c.String("keep")}

	return withService(c, func(ctx context.Context, svc taskService) error {
		collected, err := svc.GCSandboxes(ctx, request)
		if err != nil {
			return err
		}

		err = output(c, collected, func(w io.Writer) {
			writeGC(w, collected)
		})
		if err != nil {
			return err
		}

		for _, item := range collected.Sandboxes {
			if item.Error != "" {
				return ez.New(ez.EINTERNAL, "cannot delete sandbox "+item.Name+": "+item.Error, nil)
			}
		}

		return nil
	})
}
