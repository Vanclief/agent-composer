package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/interfaces/cli/client"
	"github.com/vanclief/agent-composer/task"
)

func taskCommand() *cli.Command {
	return withRemoteFlags(&cli.Command{
		Name:   "task",
		Usage:  "Task commands: work to be done, allocated compute as sandboxes (ADR 0008); --server or $AGC_SERVER calls a remote agc",
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
					&cli.BoolFlag{Name: "wait", Usage: "Against a server, poll until the sandbox is running or failed (locally create always waits)"},
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
	})
}

// valueFlags are the flags leafArgs reads with a value after them.
var valueFlags = map[string]string{
	"--server": "server",
	"--file":   "file",
	"-f":       "file",
}

// leafArgs returns the positional arguments. urfave/cli stops reading
// flags at the first argument, so --yes, --json, --server <url> and
// -f <file> written after a name are picked out here and set on c.
func leafArgs(c *cli.Context) ([]string, bool, error) {
	yes := c.Bool("yes")
	names := []string{}
	args := c.Args().Slice()
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--yes" || arg == "-y" {
			yes = true
			continue
		}

		if arg == "--json" {
			err := c.Set("json", "true")
			if err != nil {
				return nil, false, ez.New(ez.EINVALID, "--json is not a flag of agc "+c.Command.FullName(), err)
			}
			continue
		}

		flagName, value, found := splitValueFlag(arg, args, i)
		if found {
			if value == nil {
				return nil, false, ez.New(ez.EINVALID, arg+" needs a value", nil)
			}
			err := c.Set(flagName, *value)
			if err != nil {
				return nil, false, ez.New(ez.EINVALID, arg+" is not a flag of agc "+c.Command.FullName(), err)
			}
			if !strings.Contains(arg, "=") {
				i++
			}
			continue
		}

		names = append(names, arg)
	}

	return names, yes, nil
}

// splitValueFlag reads args[i] as one of valueFlags, written as "--flag v"
// or "--flag=v". value is nil when the value is missing.
func splitValueFlag(arg string, args []string, i int) (string, *string, bool) {
	key, inline, hasInline := strings.Cut(arg, "=")
	flagName, found := valueFlags[key]
	if !found {
		return "", nil, false
	}

	if hasInline {
		return flagName, &inline, true
	}

	if i+1 >= len(args) {
		return flagName, nil, true
	}

	return flagName, &args[i+1], true
}

// refArg returns the single name argument and whether --yes was given.
func refArg(c *cli.Context, command string) (string, bool, error) {
	names, yes, err := leafArgs(c)
	if err != nil {
		return "", false, err
	}

	if len(names) != 1 {
		return "", false, ez.New(ez.EINVALID, "usage: agc "+command+" "+c.Command.Name+" <name>", nil)
	}

	return names[0], yes, nil
}

// noArgs rejects positional arguments, after picking out trailing flags.
func noArgs(c *cli.Context) error {
	names, _, err := leafArgs(c)
	if err != nil {
		return err
	}

	if len(names) != 0 {
		return ez.New(ez.EINVALID, "agc "+c.Command.FullName()+" takes no arguments, got "+strings.Join(names, " "), nil)
	}

	return nil
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

// taskCreate creates the task. Locally its sandbox boots in this process,
// so it waits for the boot. Against a server it returns as soon as the
// server answers, with the sandbox starting, unless --wait is given.
func taskCreate(c *cli.Context) error {
	err := noArgs(c)
	if err != nil {
		return err
	}

	filePath := c.String("file")
	from := strings.TrimSpace(c.String("from"))
	server := serverURL(c)

	return withService(c, func(ctx context.Context, svc taskService) error {
		parsed, err := loadTaskFile(ctx, svc, filePath, from)
		if err != nil {
			return err
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
			return ez.New(ez.EINVALID, "the task needs a title: pass --title or set title: in the file", nil)
		}

		defaults, local := svc.Defaults()
		applied, err := fillTaskDefaults(request, defaults, local)
		if err != nil {
			return err
		}

		if len(applied) > 0 {
			fmt.Fprintf(os.Stderr, "Defaults applied: %s\n", strings.Join(applied, ", "))
		}

		created, err := svc.CreateTask(ctx, request)
		if err != nil {
			return err
		}

		pageURL := ""
		if server != "" {
			pageURL = client.New(server).TaskURL(created.Name)
		}

		waitBoot := server == "" || c.Bool("wait")
		if len(created.Sandboxes) == 0 || !waitBoot {
			return printCreatedTask(c, created, pageURL, false)
		}

		sandboxName := created.Sandboxes[0].Name
		fmt.Fprintf(os.Stderr, "Task %s created, booting sandbox %s…\n", created.Name, sandboxName)
		if pageURL != "" {
			fmt.Fprintf(os.Stderr, "Task page: %s\n", pageURL)
		}

		booted, err := svc.WaitSandbox(ctx, sandboxName, func(view *sandboxes.SandboxView) {
			if server != "" {
				fmt.Fprintf(os.Stderr, "sandbox %s: %s\n", view.Name, sandboxStatus(*view))
			}
		})
		if err != nil {
			return err
		}

		current, err := svc.GetTask(ctx, created.Name)
		if err != nil {
			return err
		}

		err = printCreatedTask(c, current, pageURL, true)
		if err != nil {
			return err
		}

		if server != "" && booted.Status == sandboxes.StatusFailed {
			return ez.New(ez.EINTERNAL, "sandbox "+booted.Name+" failed to boot: "+booted.StatusError, nil)
		}

		return nil
	})
}

// printCreatedTask prints the task, and its panel page when there is one:
// a last line in human output, stderr under --json so stdout stays JSON
// (unless shownOnStderr says --wait already printed it there).
func printCreatedTask(c *cli.Context, view *tasks.TaskView, pageURL string, shownOnStderr bool) error {
	if jsonOutput(c) && pageURL != "" && !shownOnStderr {
		fmt.Fprintf(os.Stderr, "Task page: %s\n", pageURL)
	}

	return output(c, view, func(w io.Writer) {
		writeTask(w, view)
		if pageURL != "" {
			fmt.Fprintf(w, "Task page: %s\n", pageURL)
		}
	})
}

// fillTaskDefaults fills in what the file leaves empty and says what it
// filled: the manager is the caller's messaging name (agc msg join), else
// the configured owner; assignee, model and owner come from the defaults
// (ADR 0009 item 1).
// fillTaskDefaults fills the manager from this machine's messaging name
// (or the configured owner), and, when the defaults are known because the
// task is created in this process, the assignee, model and owner too. Against
// a server only the manager is filled here: the server applies its own
// defaults to the rest.
func fillTaskDefaults(request *tasks.CreateRequest, defaults controller.DefaultsSettings, local bool) ([]string, error) {
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

	if !local {
		return applied, nil
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
// from as its sandbox. Either may be empty, not both. Against a server the
// saved spec is the server's.
func loadTaskFile(ctx context.Context, svc taskService, filePath string, from string) (task.File, error) {
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

	saved, err := svc.GetSpec(ctx, from)
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
	err := noArgs(c)
	if err != nil {
		return err
	}

	return withService(c, func(ctx context.Context, svc taskService) error {
		list, err := svc.ListSpecs(ctx)
		if err != nil {
			return err
		}

		return output(c, list, func(w io.Writer) {
			if len(list.Specs) == 0 {
				fmt.Fprintln(w, "No saved specs.")
			}
			for _, saved := range list.Specs {
				fmt.Fprintln(w, saved.Name)
			}
		})
	})
}

// taskSpecSave takes -f before or after the name.
func taskSpecSave(c *cli.Context) error {
	names, _, err := leafArgs(c)
	if err != nil {
		return err
	}

	filePath := c.String("file")
	if len(names) != 1 || filePath == "" {
		return ez.New(ez.EINVALID, "usage: agc task spec save <name> -f <file>", nil)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return ez.New(ez.EINVALID, "cannot read spec file "+filePath, err)
	}

	return withService(c, func(ctx context.Context, svc taskService) error {
		saved, err := svc.SaveSpec(ctx, names[0], string(data))
		if err != nil {
			return err
		}

		return output(c, saved, func(w io.Writer) {
			fmt.Fprintf(w, "Saved spec %s\n", saved.Name)
		})
	})
}

func taskSpecRemove(c *cli.Context) error {
	name, _, err := refArg(c, "task spec")
	if err != nil {
		return err
	}

	return withService(c, func(ctx context.Context, svc taskService) error {
		removed, err := svc.DeleteSpec(ctx, name)
		if err != nil {
			return err
		}

		return output(c, removed, func(w io.Writer) {
			fmt.Fprintf(w, "Deleted spec %s\n", removed.Name)
		})
	})
}

func taskList(c *cli.Context) error {
	err := noArgs(c)
	if err != nil {
		return err
	}

	return withService(c, func(ctx context.Context, svc taskService) error {
		list, err := svc.ListTasks(ctx)
		if err != nil {
			return err
		}

		return output(c, list, func(w io.Writer) {
			writeTasks(w, list)
		})
	})
}

func taskGet(c *cli.Context) error {
	name, _, err := refArg(c, "task")
	if err != nil {
		return err
	}

	return withService(c, func(ctx context.Context, svc taskService) error {
		view, err := svc.GetTask(ctx, name)
		if err != nil {
			return err
		}

		return output(c, view, func(w io.Writer) {
			writeTask(w, view)
		})
	})
}

func taskDone(c *cli.Context) error {
	name, _, err := refArg(c, "task")
	if err != nil {
		return err
	}

	return withService(c, func(ctx context.Context, svc taskService) error {
		view, err := svc.DoneTask(ctx, name)
		if err != nil {
			return err
		}

		return output(c, view, func(w io.Writer) {
			writeTask(w, view)
		})
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

	return withService(c, func(ctx context.Context, svc taskService) error {
		deleted, err := svc.DeleteTask(ctx, name)
		if err != nil {
			return err
		}

		return output(c, deleted, func(w io.Writer) {
			fmt.Fprintf(w, "Deleted task %s\n", deleted.Name)
		})
	})
}
