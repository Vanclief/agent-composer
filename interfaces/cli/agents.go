package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core"
	"github.com/vanclief/agent-composer/core/resources/agents"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/interfaces/cli/client"
)

// withAgentAdmin adds the persistent agent commands (ADR 0016) to agc
// agent, next to agc agent run.
func withAgentAdmin(cmd *cli.Command) *cli.Command {
	admin := []*cli.Command{
		{
			Name:    "list",
			Aliases: []string{"ls"},
			Usage:   "List persistent agents with their state, read from their homes",
			Action:  agentList,
		},
		{
			Name:      "show",
			Usage:     "Show a persistent agent, its tasks and its conversations",
			ArgsUsage: "<name>",
			Action:    agentShow,
		},
		{
			Name:      "create",
			Usage:     "Make a numbered agent, <role>-<n>, from the template agents/<role>.yaml; it starts asleep",
			ArgsUsage: "<role>",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "harness", Usage: "claude_code, codex_cli or pi (default the template's, else the configured default)"},
				&cli.StringFlag{Name: "model", Usage: "Model the harness runs (default the template's, else the configured default)"},
			},
			Action: agentCreate,
		},
		{
			Name:      "keep",
			Usage:     "Keep an agent: give it a name (or a new one) and maybe another role; its home and volume keep theirs",
			ArgsUsage: "<name> --as <new name>",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "as", Usage: "The name to keep it under: lowercase letters, digits and dashes"},
				&cli.StringFlag{Name: "role", Usage: "The role it runs from now on (default its role)"},
			},
			Action: agentKeep,
		},
		{
			Name:      "wake",
			Usage:     "Wake an agent: boot its home the first time, start it when stopped",
			ArgsUsage: "<name>",
			Action:    agentWake,
		},
		{
			Name:      "sleep",
			Usage:     "Put an awake agent to sleep: stop its home and post \"Asleep.\" in its DM",
			ArgsUsage: "<name>",
			Action:    agentSleep,
		},
		{
			Name:      "delete",
			Aliases:   []string{"rm"},
			Usage:     "Delete an agent and its home sandbox; its volume (memory and transcripts), messages and tasks are kept",
			ArgsUsage: "<name>",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "Do not ask for confirmation"},
			},
			Action: agentDelete,
		},
	}

	for _, sub := range admin {
		sub.Before = quietLogs
		withRemoteFlags(sub)
	}

	cmd.Usage = "Persistent agents (ADR 0016), and agc agent run, the agent loop inside a sandbox (ADR 0009); --server or $AGC_SERVER calls a remote agc"
	cmd.Subcommands = append(cmd.Subcommands, admin...)

	return cmd
}

// agentService is what the agent commands need, served in-process
// (localAgents) or by a remote agc server (client.Client).
type agentService interface {
	ListAgents(ctx context.Context) (*agents.ListResponse, error)
	GetAgent(ctx context.Context, name string) (*agents.AgentView, error)
	CreateAgent(ctx context.Context, request *agents.CreateRequest) (*agents.AgentView, error)
	KeepAgent(ctx context.Context, name string, request *agents.KeepRequest) (*agents.AgentView, error)
	WakeAgent(ctx context.Context, name string) (*agents.AgentView, error)
	SleepAgent(ctx context.Context, name string) (*agents.AgentView, error)
	DeleteAgent(ctx context.Context, name string) (*agents.DeleteResponse, error)
}

// withAgentService runs fn against the server when one is set, else
// against an in-process stack. In-process a first wake boots the home in
// this process, so it waits for the boot before returning.
func withAgentService(c *cli.Context, fn func(ctx context.Context, svc agentService) error) error {
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

	return fn(c.Context, localAgents{api: stack.AgentsAPI, sandboxes: stack.TasksAPI.Sandboxes})
}

// localAgents serves agentService from the in-process agents API.
type localAgents struct {
	api       *agents.API
	sandboxes *sandboxes.API
}

func (s localAgents) ListAgents(ctx context.Context) (*agents.ListResponse, error) {
	return s.api.List(ctx, nil, &agents.ListRequest{})
}

func (s localAgents) GetAgent(ctx context.Context, name string) (*agents.AgentView, error) {
	return s.api.Get(ctx, nil, &agents.GetRequest{Name: name})
}

func (s localAgents) CreateAgent(ctx context.Context, request *agents.CreateRequest) (*agents.AgentView, error) {
	return s.api.Create(ctx, nil, request)
}

func (s localAgents) KeepAgent(ctx context.Context, name string, request *agents.KeepRequest) (*agents.AgentView, error) {
	request.Agent = name
	return s.api.Keep(ctx, nil, request)
}

func (s localAgents) WakeAgent(ctx context.Context, name string) (*agents.AgentView, error) {
	_, err := s.api.Wake(ctx, nil, &agents.WakeRequest{Name: name})
	if err != nil {
		return nil, err
	}

	s.sandboxes.Wait()

	return s.api.Get(ctx, nil, &agents.GetRequest{Name: name})
}

func (s localAgents) SleepAgent(ctx context.Context, name string) (*agents.AgentView, error) {
	return s.api.Sleep(ctx, nil, &agents.SleepRequest{Name: name})
}

func (s localAgents) DeleteAgent(ctx context.Context, name string) (*agents.DeleteResponse, error) {
	return s.api.Delete(ctx, nil, &agents.DeleteRequest{Name: name})
}

func agentList(c *cli.Context) error {
	err := noArgs(c)
	if err != nil {
		return err
	}

	return withAgentService(c, func(ctx context.Context, svc agentService) error {
		list, err := svc.ListAgents(ctx)
		if err != nil {
			return err
		}

		return output(c, list, func(w io.Writer) {
			writeAgents(w, list.Agents)
		})
	})
}

func agentShow(c *cli.Context) error {
	name, _, err := refArg(c, "agent")
	if err != nil {
		return err
	}

	return withAgentService(c, func(ctx context.Context, svc agentService) error {
		view, err := svc.GetAgent(ctx, name)
		if err != nil {
			return err
		}

		return output(c, view, func(w io.Writer) {
			writeAgent(w, view)
		})
	})
}

func agentCreate(c *cli.Context) error {
	role, _, err := refArg(c, "agent")
	if err != nil {
		return err
	}

	request := &agents.CreateRequest{Role: role, Harness: c.String("harness"), Model: c.String("model")}

	return withAgentService(c, func(ctx context.Context, svc agentService) error {
		view, err := svc.CreateAgent(ctx, request)
		if err != nil {
			return err
		}

		return output(c, view, func(w io.Writer) {
			writeAgent(w, view)
		})
	})
}

func agentKeep(c *cli.Context) error {
	name, _, err := refArg(c, "agent")
	if err != nil {
		return err
	}

	as := strings.TrimSpace(c.String("as"))
	if as == "" {
		return ez.New(ez.EINVALID, "usage: agc agent keep <name> --as <new name> [--role <role>]", nil)
	}

	request := &agents.KeepRequest{Name: as, Role: c.String("role")}

	return withAgentService(c, func(ctx context.Context, svc agentService) error {
		view, err := svc.KeepAgent(ctx, name, request)
		if err != nil {
			return err
		}

		return output(c, view, func(w io.Writer) {
			writeAgent(w, view)
		})
	})
}

func agentWake(c *cli.Context) error {
	return agentStateCommand(c, func(ctx context.Context, svc agentService, name string) (*agents.AgentView, error) {
		return svc.WakeAgent(ctx, name)
	})
}

func agentSleep(c *cli.Context) error {
	return agentStateCommand(c, func(ctx context.Context, svc agentService, name string) (*agents.AgentView, error) {
		return svc.SleepAgent(ctx, name)
	})
}

func agentStateCommand(c *cli.Context, fn func(ctx context.Context, svc agentService, name string) (*agents.AgentView, error)) error {
	name, _, err := refArg(c, "agent")
	if err != nil {
		return err
	}

	return withAgentService(c, func(ctx context.Context, svc agentService) error {
		view, err := fn(ctx, svc, name)
		if err != nil {
			return err
		}

		return output(c, view, func(w io.Writer) {
			writeAgent(w, view)
		})
	})
}

func agentDelete(c *cli.Context) error {
	name, yes, err := refArg(c, "agent")
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Deleting agent %s deletes its home sandbox and its disk; its volume, messages and tasks are kept.\n", name)

	if !yes {
		err = confirm("agent " + name)
		if err != nil {
			return err
		}
	}

	return withAgentService(c, func(ctx context.Context, svc agentService) error {
		deleted, err := svc.DeleteAgent(ctx, name)
		if err != nil {
			return err
		}

		return output(c, deleted, func(w io.Writer) {
			fmt.Fprintf(w, "Deleted agent %s\n", deleted.Name)
		})
	})
}

// writeAgents prints one line per agent: name, state, role, harness and
// model, home, last active.
func writeAgents(w io.Writer, views []agents.AgentView) {
	if len(views) == 0 {
		fmt.Fprintln(w, "No agents.")
		return
	}

	for i := range views {
		view := &views[i]
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", agentName(view), view.State, view.Role, harnessModel(view), orDash(homeName(view)), agentLastActive(view))
	}
}

// writeAgent prints one agent with its tasks and conversations.
func writeAgent(w io.Writer, view *agents.AgentView) {
	fmt.Fprintf(w, "Agent:\t%s\n", agentName(view))
	fmt.Fprintf(w, "State:\t%s\n", view.State)
	fmt.Fprintf(w, "Role:\t%s\n", view.Role)
	fmt.Fprintf(w, "Harness:\t%s\n", harnessModel(view))
	fmt.Fprintf(w, "Owner:\t%s\n", orDash(view.Owner))
	fmt.Fprintf(w, "Home:\t%s\n", orDash(homeName(view)))
	fmt.Fprintf(w, "Last active:\t%s\n", agentLastActive(view))

	for _, t := range view.Tasks {
		fmt.Fprintf(w, "Task:\t%s (%s)\n", t.Name, t.Status)
	}

	for _, conversation := range view.Conversations {
		fmt.Fprintf(w, "Conversation:\t%s (%s)\n", conversation.Channel, conversation.LastAt.Local().Format(time.DateTime))
	}
}

// agentName marks an agent that is still numbered.
func agentName(view *agents.AgentView) string {
	if view.Kept {
		return view.Name
	}

	return view.Name + " (numbered)"
}

func harnessModel(view *agents.AgentView) string {
	if view.Model == "" {
		return view.Harness
	}

	return view.Harness + " (" + view.Model + ")"
}

func homeName(view *agents.AgentView) string {
	if view.Home == nil {
		return ""
	}

	return *view.Home
}

func agentLastActive(view *agents.AgentView) string {
	if view.LastActiveAt == nil {
		return "never"
	}

	return view.LastActiveAt.Local().Format(time.DateTime)
}
