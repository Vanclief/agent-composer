package cli

import (
	"fmt"
	"io"
	"strings"

	cli "github.com/urfave/cli/v2"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/triggers"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
)

func triggersCommand() *cli.Command {
	return &cli.Command{
		Name:   "triggers",
		Usage:  "Outside events posted as messages (ADR 0014); agc rest polls the sources in triggers: of the config file",
		Before: quietLogs,
		Subcommands: []*cli.Command{
			{
				Name:  "show",
				Usage: "List the trigger sources with their cursors, when each last polled and what it posted",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "json", Usage: "Print the raw JSON view"},
				},
				Action: triggersShow,
			},
		},
	}
}

// triggersShowView is the JSON of agc triggers show.
type triggersShowView struct {
	Enabled   bool                  `json:"enabled"`
	Repos     []string              `json:"repos"`
	Poll      string                `json:"poll"`
	StateFile string                `json:"state_file"`
	Sources   []triggers.SourceView `json:"sources"`
}

// triggersShow reads the config and the cursor file only: it needs no
// database and works while agc rest runs.
func triggersShow(c *cli.Context) error {
	ctrl, err := controller.Load(nil)
	if err != nil {
		return err
	}

	home, err := workflowruntime.ResolveHomeDir()
	if err != nil {
		return err
	}

	github := ctrl.Config.Triggers.GitHub
	api := triggers.New(github, nil, nil, "", triggers.StatePath(home))

	sources, err := api.Show()
	if err != nil {
		return err
	}

	poll := "invalid"
	every, err := github.PollInterval()
	if err == nil {
		poll = every.String()
	}

	view := triggersShowView{
		Enabled:   github.Enabled(),
		Repos:     github.RepoList(),
		Poll:      poll,
		StateFile: api.StorePath(),
		Sources:   sources,
	}

	return output(c, view, func(w io.Writer) {
		if view.Enabled {
			fmt.Fprintf(w, "github: on, every %s, repos %s\n", view.Poll, strings.Join(view.Repos, ", "))
		} else {
			fmt.Fprintln(w, "github: off (no triggers.github.repos in the config file)")
		}
		fmt.Fprintf(w, "cursors: %s\n\n", view.StateFile)

		if len(view.Sources) == 0 {
			return
		}

		fmt.Fprintln(w, "SOURCE\tCURSOR\tLAST POLLED\tPOSTED\tLAST ERROR")
		for _, source := range view.Sources {
			name := source.Source
			if !source.Configured {
				name += " (not configured)"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", name, formatTime(source.Cursor), formatTime(source.LastPolled), source.Posted, orDash(source.LastError))
		}
	})
}
