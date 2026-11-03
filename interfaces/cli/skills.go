package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core"
	"github.com/vanclief/agent-composer/core/resources/skills"
	"github.com/vanclief/agent-composer/interfaces/cli/client"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
)

// skillCommand is agc skill: the skills repository a sandbox spec names
// in skills, written into every sandbox agc prepares (ADR 0008 "Skills").
func skillCommand() *cli.Command {
	cmd := &cli.Command{
		Name:  "skill",
		Usage: "Skills agc writes into sandboxes (<agc home>/skills or $AGC_SKILLS_DIR); --server or $AGC_SERVER calls a remote agc",
		Subcommands: []*cli.Command{
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List the skills with their descriptions",
				Before:  quietLogs,
				Action:  skillList,
			},
			{
				Name:      "show",
				Usage:     "Print a skill's SKILL.md",
				ArgsUsage: "<name>",
				Before:    quietLogs,
				Action:    skillShow,
			},
		},
	}

	return withRemoteFlags(cmd)
}

// skillService is what the skill commands need, served from the local
// skills directory (localSkills) or by a remote agc server
// (client.Client).
type skillService interface {
	ListSkills(ctx context.Context) (*skills.ListResponse, error)
	GetSkill(ctx context.Context, name string) (*skills.SkillText, error)
}

// withSkillService runs fn against the server when one is set, else
// against the local skills directory; no database is opened.
func withSkillService(c *cli.Context, fn func(ctx context.Context, svc skillService) error) error {
	server := serverURL(c)
	if server != "" {
		err := fn(c.Context, client.New(server))
		if err != nil {
			fmt.Fprintln(c.App.ErrWriter, ez.ErrorMessage(err))
			return &PrintedError{Err: err}
		}

		return nil
	}

	home, err := workflowruntime.ResolveHomeDir()
	if err != nil {
		return err
	}

	return fn(c.Context, localSkills{api: skills.NewAPI(core.SkillsDir(home))})
}

// localSkills serves skillService from the skills API in process.
type localSkills struct {
	api *skills.API
}

func (s localSkills) ListSkills(ctx context.Context) (*skills.ListResponse, error) {
	return s.api.List(ctx, nil, &skills.ListRequest{})
}

func (s localSkills) GetSkill(ctx context.Context, name string) (*skills.SkillText, error) {
	return s.api.Get(ctx, nil, &skills.GetRequest{Name: name})
}

func skillList(c *cli.Context) error {
	err := noArgs(c)
	if err != nil {
		return err
	}

	return withSkillService(c, func(ctx context.Context, svc skillService) error {
		list, err := svc.ListSkills(ctx)
		if err != nil {
			return err
		}

		return output(c, list, func(w io.Writer) {
			if len(list.Skills) == 0 {
				fmt.Fprintln(w, "No skills.")
				return
			}

			for _, skill := range list.Skills {
				fmt.Fprintf(w, "%s\t%s\n", skill.Name, firstSentence(skill.Description))
			}
		})
	})
}

func skillShow(c *cli.Context) error {
	name, _, err := refArg(c, "skill")
	if err != nil {
		return err
	}

	return withSkillService(c, func(ctx context.Context, svc skillService) error {
		skill, err := svc.GetSkill(ctx, name)
		if err != nil {
			return err
		}

		if jsonOutput(c) {
			return output(c, skill, func(w io.Writer) {})
		}

		_, err = io.WriteString(c.App.Writer, skill.Text)

		return err
	})
}

// firstSentence is description up to its first ". ", for one line in a
// list.
func firstSentence(description string) string {
	before, _, found := strings.Cut(description, ". ")
	if found {
		return before + "."
	}

	return description
}
