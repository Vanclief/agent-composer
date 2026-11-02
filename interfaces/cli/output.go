package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	cli "github.com/urfave/cli/v2"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
)

// output prints value as JSON under --json, else as the lines human
// writes. Both modes, local and server, print the same.
func output(c *cli.Context, value any, human func(w io.Writer)) error {
	if jsonOutput(c) {
		encoded, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(c.App.Writer, string(encoded))

		return err
	}

	w := tabwriter.NewWriter(c.App.Writer, 0, 0, 2, ' ', 0)
	human(w)

	return w.Flush()
}

func orDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}

	return value
}

// sandboxStatus is the status, with the error when there is one.
func sandboxStatus(view sandboxes.SandboxView) string {
	if view.StatusError == "" {
		return view.Status
	}

	return view.Status + ": " + view.StatusError
}

// writeTask prints one task: a line with its name, status, assignee and
// title, then a line per sandbox, indented, with its name and status.
func writeTask(w io.Writer, view *tasks.TaskView) {
	writeTaskPadded(w, view, taskWidths([]tasks.TaskView{*view}))
}

// writeTasks prints every task as writeTask does, columns aligned across
// the tasks.
func writeTasks(w io.Writer, list *tasks.ListResponse) {
	if len(list.Tasks) == 0 {
		fmt.Fprintln(w, "No tasks.")
		return
	}

	widths := taskWidths(list.Tasks)
	for i := range list.Tasks {
		writeTaskPadded(w, &list.Tasks[i], widths)
	}
}

// taskWidths are the widest name, status and assignee among views. Task
// lines are padded by hand: a tabwriter would align the indented sandbox
// lines with them.
func taskWidths(views []tasks.TaskView) [3]int {
	widths := [3]int{}
	for _, view := range views {
		cells := [3]string{view.Name, view.Status, orDash(view.Assignee)}
		for i, cell := range cells {
			widths[i] = max(widths[i], len(cell))
		}
	}

	return widths
}

func writeTaskPadded(w io.Writer, view *tasks.TaskView, widths [3]int) {
	fmt.Fprintf(w, "%-*s  %-*s  %-*s  %s\n", widths[0], view.Name, widths[1], view.Status, widths[2], orDash(view.Assignee), view.Title)
	for _, sandbox := range view.Sandboxes {
		fmt.Fprintf(w, "  %s  %s\n", sandbox.Name, sandboxStatus(sandbox))
	}
}

func writeSandbox(w io.Writer, view *sandboxes.SandboxView) {
	fmt.Fprintf(w, "%s\t%s\ttask %s\n", view.Name, sandboxStatus(*view), view.Task)
}

func writeSandboxes(w io.Writer, list *sandboxes.ListResponse) {
	if len(list.Sandboxes) == 0 {
		fmt.Fprintln(w, "No sandboxes.")
		return
	}

	fmt.Fprintln(w, "NAME\tSTATUS\tTASK")
	for _, view := range list.Sandboxes {
		fmt.Fprintf(w, "%s\t%s\t%s\n", view.Name, sandboxStatus(view), view.Task)
	}
}
