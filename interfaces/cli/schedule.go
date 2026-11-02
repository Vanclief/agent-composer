package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core"
	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/schedules"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/interfaces/cli/client"
)

func scheduleCommand() *cli.Command {
	return withRemoteFlags(&cli.Command{
		Name:   "schedule",
		Usage:  "Scheduled tasks: a task template created every interval, or once per new item a poll command lists (ADR 0011); agc rest runs them; --server or $AGC_SERVER calls a remote agc",
		Before: quietLogs,
		Subcommands: []*cli.Command{
			{
				Name:  "create",
				Usage: "Create a schedule from a schedule.yaml; agc rest runs it at once and then every interval",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Path to the schedule.yaml"},
				},
				Action: scheduleCreate,
			},
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List schedules with their interval, last run and next run",
				Action:  scheduleList,
			},
			{
				Name:      "show",
				Usage:     "Show a schedule, its template and the items it has seen",
				ArgsUsage: "<name>",
				Action:    scheduleShow,
			},
			{
				Name:      "run",
				Usage:     "Run a schedule now, enabled or not, and list the tasks it created; locally this runs in this process",
				ArgsUsage: "<name>",
				Action:    scheduleRun,
			},
			{
				Name:      "enable",
				Usage:     "Enable a schedule",
				ArgsUsage: "<name>",
				Action: func(c *cli.Context) error {
					return scheduleSetEnabled(c, true)
				},
			},
			{
				Name:      "disable",
				Usage:     "Disable a schedule; a tick running now finishes",
				ArgsUsage: "<name>",
				Action: func(c *cli.Context) error {
					return scheduleSetEnabled(c, false)
				},
			},
			{
				Name:      "delete",
				Aliases:   []string{"rm"},
				Usage:     "Delete a schedule and the items it has seen; the tasks it created stay",
				ArgsUsage: "<name>",
				Action:    scheduleDelete,
			},
		},
	})
}

// scheduleService is what agc schedule needs, served in-process
// (localSchedules) or by a remote agc server (client.Client).
type scheduleService interface {
	Defaults() (controller.DefaultsSettings, bool)
	ListSchedules(ctx context.Context) (*schedules.ListResponse, error)
	CreateSchedule(ctx context.Context, request *schedules.CreateRequest) (*schedules.ScheduleView, error)
	GetSchedule(ctx context.Context, ref string) (*schedules.ScheduleView, error)
	UpdateSchedule(ctx context.Context, ref string, request *schedules.UpdateRequest) (*schedules.ScheduleView, error)
	DeleteSchedule(ctx context.Context, ref string) (*schedules.DeleteResponse, error)
	RunSchedule(ctx context.Context, ref string) (*schedules.RunResponse, error)
	ListScheduleItems(ctx context.Context, ref string) (*schedules.ListItemsResponse, error)
}

// withScheduleService runs fn against the server when one is set, else
// against an in-process stack, whose scheduler is never started: locally
// a schedule runs only with agc schedule run.
func withScheduleService(c *cli.Context, fn func(ctx context.Context, svc scheduleService) error) error {
	server := serverURL(c)
	if server != "" {
		err := fn(c.Context, client.New(server))
		if err != nil {
			fmt.Fprintln(c.App.ErrWriter, ez.ErrorMessage(err))
			return &PrintedError{Err: err}
		}

		return nil
	}

	stack, err := core.NewStack(c.Context, core.StackOptions{ProjectDir: c.String("project")})
	if err != nil {
		return err
	}
	defer stack.Controller.DB.Close() // nolint:errcheck // Close errors are not actionable here.

	return fn(c.Context, localSchedules{api: stack.SchedulesAPI, tasks: stack.TasksAPI})
}

// localSchedules serves scheduleService from the in-process APIs.
type localSchedules struct {
	api   *schedules.API
	tasks *tasks.API
}

func (s localSchedules) Defaults() (controller.DefaultsSettings, bool) {
	return s.tasks.Defaults(), true
}

func (s localSchedules) ListSchedules(ctx context.Context) (*schedules.ListResponse, error) {
	return s.api.List(ctx, nil, &schedules.ListRequest{})
}

func (s localSchedules) CreateSchedule(ctx context.Context, request *schedules.CreateRequest) (*schedules.ScheduleView, error) {
	return s.api.Create(ctx, nil, request)
}

func (s localSchedules) GetSchedule(ctx context.Context, ref string) (*schedules.ScheduleView, error) {
	return s.api.Get(ctx, nil, &schedules.GetRequest{Name: ref})
}

func (s localSchedules) UpdateSchedule(ctx context.Context, ref string, request *schedules.UpdateRequest) (*schedules.ScheduleView, error) {
	request.ID = uuid.Nil
	request.Name = ref
	return s.api.Update(ctx, nil, request)
}

func (s localSchedules) DeleteSchedule(ctx context.Context, ref string) (*schedules.DeleteResponse, error) {
	return s.api.Delete(ctx, nil, &schedules.DeleteRequest{Name: ref})
}

// RunSchedule waits for the boots of the sandboxes it allocated, which run
// in this process.
func (s localSchedules) RunSchedule(ctx context.Context, ref string) (*schedules.RunResponse, error) {
	response, err := s.api.Run(ctx, nil, &schedules.RunRequest{Name: ref})
	if err != nil {
		return nil, err
	}

	s.tasks.Sandboxes.Wait()

	return response, nil
}

func (s localSchedules) ListScheduleItems(ctx context.Context, ref string) (*schedules.ListItemsResponse, error) {
	return s.api.ListItems(ctx, nil, &schedules.ListItemsRequest{Name: ref})
}

func scheduleCreate(c *cli.Context) error {
	err := noArgs(c)
	if err != nil {
		return err
	}

	filePath := c.String("file")
	if filePath == "" {
		return ez.New(ez.EINVALID, "usage: agc schedule create -f <schedule.yaml>", nil)
	}

	request, err := schedules.LoadFile(filePath)
	if err != nil {
		return err
	}

	return withScheduleService(c, func(ctx context.Context, svc scheduleService) error {
		defaults, _ := svc.Defaults()
		applied, err := fillManager(&request.Task.Manager, defaults, "task.manager:")
		if err != nil {
			return err
		}
		if applied != "" {
			fmt.Fprintf(os.Stderr, "Defaults applied: %s\n", applied)
		}

		view, err := svc.CreateSchedule(ctx, request)
		if err != nil {
			return err
		}

		return output(c, view, func(w io.Writer) {
			writeSchedule(w, view)
		})
	})
}

func scheduleList(c *cli.Context) error {
	err := noArgs(c)
	if err != nil {
		return err
	}

	return withScheduleService(c, func(ctx context.Context, svc scheduleService) error {
		list, err := svc.ListSchedules(ctx)
		if err != nil {
			return err
		}

		return output(c, list, func(w io.Writer) {
			writeSchedules(w, list)
		})
	})
}

// scheduleShowView is agc schedule show --json: the schedule and its items.
type scheduleShowView struct {
	Schedule *schedules.ScheduleView `json:"schedule"`
	Items    []schedules.ItemView    `json:"items"`
}

func scheduleShow(c *cli.Context) error {
	name, _, err := refArg(c, "schedule")
	if err != nil {
		return err
	}

	return withScheduleService(c, func(ctx context.Context, svc scheduleService) error {
		view, err := svc.GetSchedule(ctx, name)
		if err != nil {
			return err
		}

		items, err := svc.ListScheduleItems(ctx, name)
		if err != nil {
			return err
		}

		shown := scheduleShowView{Schedule: view, Items: items.Items}

		return output(c, shown, func(w io.Writer) {
			writeSchedule(w, view)
			writeScheduleDetail(w, view, items.Items)
		})
	})
}

// scheduleRun prints the tasks the tick created and fails when the tick
// did.
func scheduleRun(c *cli.Context) error {
	name, _, err := refArg(c, "schedule")
	if err != nil {
		return err
	}

	return withScheduleService(c, func(ctx context.Context, svc scheduleService) error {
		response, err := svc.RunSchedule(ctx, name)
		if err != nil {
			return err
		}

		err = output(c, response, func(w io.Writer) {
			if len(response.Created) == 0 {
				fmt.Fprintln(w, "No tasks created.")
			}
			for i := range response.Created {
				writeTask(w, &response.Created[i])
			}
		})
		if err != nil {
			return err
		}

		if response.Error != "" {
			return ez.New(ez.EINTERNAL, "schedule "+name+": "+response.Error, nil)
		}

		return nil
	})
}

// scheduleSetEnabled reads the schedule and puts it back with enabled
// changed.
func scheduleSetEnabled(c *cli.Context, enabled bool) error {
	name, _, err := refArg(c, "schedule")
	if err != nil {
		return err
	}

	return withScheduleService(c, func(ctx context.Context, svc scheduleService) error {
		current, err := svc.GetSchedule(ctx, name)
		if err != nil {
			return err
		}

		view, err := svc.UpdateSchedule(ctx, name, &schedules.UpdateRequest{
			Every:   current.Every,
			Enabled: &enabled,
			Task:    current.Task,
			Poll:    current.Poll,
		})
		if err != nil {
			return err
		}

		return output(c, view, func(w io.Writer) {
			writeSchedule(w, view)
		})
	})
}

func scheduleDelete(c *cli.Context) error {
	name, _, err := refArg(c, "schedule")
	if err != nil {
		return err
	}

	return withScheduleService(c, func(ctx context.Context, svc scheduleService) error {
		deleted, err := svc.DeleteSchedule(ctx, name)
		if err != nil {
			return err
		}

		return output(c, deleted, func(w io.Writer) {
			fmt.Fprintf(w, "Deleted schedule %s; the tasks it created are kept\n", deleted.Name)
		})
	})
}

// scheduleState is enabled, disabled or running.
func scheduleState(view *schedules.ScheduleView) string {
	if view.Running {
		return "running"
	}

	if view.Enabled {
		return "enabled"
	}

	return "disabled"
}

func formatTime(at *time.Time) string {
	if at == nil {
		return "-"
	}

	return at.Local().Format("2006-01-02 15:04:05")
}

func writeSchedule(w io.Writer, view *schedules.ScheduleView) {
	fmt.Fprintln(w, "NAME\tEVERY\tSTATE\tLAST RUN\tNEXT RUN\tLAST ERROR")
	writeScheduleRow(w, view)
}

func writeScheduleRow(w io.Writer, view *schedules.ScheduleView) {
	lastError := strings.SplitN(view.LastError, "\n", 2)[0]
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", view.Name, view.Every, scheduleState(view), formatTime(view.LastRunAt), formatTime(view.NextRunAt), orDash(lastError))
}

func writeSchedules(w io.Writer, list *schedules.ListResponse) {
	if len(list.Schedules) == 0 {
		fmt.Fprintln(w, "No schedules.")
		return
	}

	fmt.Fprintln(w, "NAME\tEVERY\tSTATE\tLAST RUN\tNEXT RUN\tLAST ERROR")
	for i := range list.Schedules {
		writeScheduleRow(w, &list.Schedules[i])
	}
}

// writeScheduleDetail prints the poll, the template's title, the whole
// last error and the items seen.
func writeScheduleDetail(w io.Writer, view *schedules.ScheduleView, items []schedules.ItemView) {
	fmt.Fprintln(w)
	if view.Poll != nil {
		fmt.Fprintf(w, "Poll:\t%s\n", view.Poll.Command)
		if len(view.Poll.Secrets) > 0 {
			fmt.Fprintf(w, "Secrets:\t%s\n", strings.Join(view.Poll.Secrets, ", "))
		}
	}
	fmt.Fprintf(w, "Task title:\t%s\n", orDash(view.Task.Title))
	if view.Task.Name != "" {
		fmt.Fprintf(w, "Task name:\t%s\n", view.Task.Name)
	}
	if view.LastError != "" {
		fmt.Fprintf(w, "Last error:\t%s\n", strings.ReplaceAll(view.LastError, "\n", "\n\t"))
	}

	fmt.Fprintln(w)
	if len(items) == 0 {
		fmt.Fprintln(w, "No items yet.")
		return
	}

	fmt.Fprintln(w, "SEEN\tTASK\tKEY")
	for _, item := range items {
		seen := item.SeenAt
		fmt.Fprintf(w, "%s\t%s\t%s\n", formatTime(&seen), item.Task, item.Key)
	}
}
