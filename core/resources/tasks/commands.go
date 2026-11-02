package tasks

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// The channel commands of ADR 0010 items 4 and 5.
const (
	// AcceptCommand accepts the task when its manager or owner posts it in
	// #<task>, or when the owner posts "/accept <task>" in their inbox.
	AcceptCommand = "/accept"
	// RejectCommand, "/reject <why>", sends the task back when its manager
	// or owner posts it in #<task>.
	RejectCommand = "/reject"
)

// commandTimeout bounds what a command does after the post that carried
// it, stopping the task's sandboxes included.
const commandTimeout = 5 * time.Minute

// Command is a channel command read from a message body.
type Command struct {
	// Verb is AcceptCommand or RejectCommand.
	Verb string
	// Arg is the rest of the body, trimmed: the reason of a reject, the
	// task of an accept in an inbox.
	Arg string
}

// ParseCommand reads body as a command. The verb must be the whole first
// word, so "/accepted" and "/reject" with no reason are not commands.
func ParseCommand(body string) (Command, bool) {
	trimmed := strings.TrimSpace(body)

	verb := trimmed
	arg := ""
	end := strings.IndexFunc(trimmed, unicode.IsSpace)
	if end >= 0 {
		verb = trimmed[:end]
		arg = strings.TrimSpace(trimmed[end:])
	}

	switch verb {
	case AcceptCommand:
		return Command{Verb: verb, Arg: arg}, true
	case RejectCommand:
		if arg == "" {
			return Command{}, false
		}
		return Command{Verb: verb, Arg: arg}, true
	}

	return Command{}, false
}

// onCommand runs /accept and /reject. In #<task> they count from the
// task's manager or owner; "/accept <task>" in #<owner> counts from that
// owner. Messages from a task's sandboxes never count, and from anyone
// else a command is an ordinary message.
func (api *API) onCommand(ctx context.Context, m *msgmodels.Message) {
	command, ok := ParseCommand(m.Body)
	if !ok || m.Task != "" || m.From == msgmodels.SystemName {
		return
	}

	channel := strings.TrimPrefix(m.Channel, "#")

	// The command's effect outlives the post that carried it.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), commandTimeout)
	defer cancel()

	if command.Verb == AcceptCommand && command.Arg != "" {
		api.acceptFromInbox(ctx, m, channel, command.Arg)
		return
	}

	record := api.commandTask(ctx, m, channel)
	if record == nil {
		return
	}

	if m.From != record.Manager && m.From != record.Owner {
		return
	}

	var err error
	if command.Verb == AcceptCommand {
		err = api.accept(ctx, record, m.From)
	} else {
		err = api.reject(ctx, record, m.From, command.Arg)
	}
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Str("command", command.Verb).Msg("cannot run the channel command")
	}
}

// acceptFromInbox runs "/accept <task>" posted by the owner in their own
// inbox, #<owner>.
func (api *API) acceptFromInbox(ctx context.Context, m *msgmodels.Message, channel string, name string) {
	if channel != m.From {
		return
	}

	record := api.commandTask(ctx, m, strings.TrimPrefix(name, "#"))
	if record == nil || record.Owner != m.From {
		return
	}

	err := api.accept(ctx, record, m.From)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot accept the task from the owner's inbox")
	}
}

// commandTask is the task called name, or nil when there is none.
func (api *API) commandTask(ctx context.Context, m *msgmodels.Message, name string) *taskmodels.Task {
	err := taskmodels.ValidateName(name)
	if err != nil {
		return nil
	}

	record, err := taskmodels.GetTaskByName(ctx, api.db, name)
	if ez.ErrorCode(err) == ez.ENOTFOUND {
		return nil
	}
	if err != nil {
		log.Warn().Err(err).Str("channel", m.Channel).Msg("cannot read the task for a channel command")
		return nil
	}

	return record
}
