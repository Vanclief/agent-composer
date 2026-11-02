package tasks

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// OpeningMessage is what agc posts in #<task> when the task is created
// (ADR 0009 item 4), addressing the manager and the owner. allocated is
// the task's sandbox, nil when it has none. Without an owner the owner
// parts are left out.
func OpeningMessage(record *taskmodels.Task, allocated *sandboxes.SandboxView) string {
	var b strings.Builder

	b.WriteString("@" + record.Manager + " (manager)")
	if record.Owner != "" {
		b.WriteString(", @" + record.Owner + " (owner)")
	}
	b.WriteString(": task `" + record.Name + "` is open: " + record.Title + ".\n")

	worker := record.Assignee
	if record.Model != "" {
		worker += " (" + record.Model + ")"
	}

	if allocated == nil {
		b.WriteString("Assigned to " + worker + ", no sandbox.\n")
		b.WriteString("Post /done to close the task.")
		return b.String()
	}

	b.WriteString("Agent: " + worker + " on sandbox `" + allocated.Name + "`, starting.\n")

	if record.Owner == "" {
		b.WriteString("The agent's instructions are the next message. Reply here to talk to it.\n")
		b.WriteString("Post /done to close the task.")
		return b.String()
	}

	b.WriteString("The agent's instructions are the next message. Reply here to talk to it;\n")
	b.WriteString("it escalates to #" + record.Owner + " when only the owner can decide. Post /done to\n")
	b.WriteString("close the task.")

	return b.String()
}

// open posts the opening message in #<task> as agc, then the sandbox's
// instructions verbatim when there is a sandbox. The task exists either
// way, so a failure is logged, not returned.
func (api *API) open(ctx context.Context, record *taskmodels.Task, allocated *sandboxes.SandboxView) {
	channel := messaging.TaskChannel(record.Name)

	_, err := api.msg.PostAsSystem(ctx, channel, OpeningMessage(record, allocated))
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot post the opening message")
		return
	}

	if allocated == nil {
		return
	}

	_, err = api.msg.PostAsSystem(ctx, channel, allocated.Instructions)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot post the agent's instructions")
	}
}
