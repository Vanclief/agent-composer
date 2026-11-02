package agents

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"

	"github.com/vanclief/agent-composer/models/agent"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// fromSystem reports whether agc itself posted m.
func fromSystem(m *msgmodels.Message) bool {
	return msgmodels.FromSystem(m)
}

// forManager reports whether agc's post m may wake the manager of its
// task: a delivery to review, or an accept to act on (the reviewer merges
// after the owner accepts; TurnFor leaves out the manager's own accept).
func forManager(m *msgmodels.Message) bool {
	return strings.HasPrefix(m.Body, msgmodels.DeliveredPrefix) || strings.HasPrefix(m.Body, msgmodels.AcceptedPrefix)
}

// onPost is the wake hook (ADR 0016 item 6): every asleep agent that reads
// the message's channel is woken when the message is a turn for it
// (msgmodels.TurnFor): a person's post, another agent's question or post
// naming it, never its own post; agc's own posts wake only the manager of
// the task whose channel they are in, and only for a delivery or an accept
// (forManager). A reply in a thread an agent started wakes that agent in
// any channel, read or not (msgmodels.AnswerFor), so the asker hears the
// answer. Wakes run in the background so a post never waits on a VM.
func (api *API) onPost(ctx context.Context, m *msgmodels.Message) {
	if fromSystem(m) && !forManager(m) {
		return
	}

	woken := api.toWake(ctx, m)

	for i := range woken {
		record := woken[i]

		api.wakes.Add(1)
		go func() {
			defer api.wakes.Done()

			err := api.wake(context.WithoutCancel(ctx), &record)
			if err != nil {
				log.Warn().Err(err).Str("agent", record.Name).Str("channel", m.Channel).Msg("cannot wake the agent for a message")
			}
		}()
	}
}

// toWake returns the asleep agents for which the message is a turn: in a
// channel they read, by msgmodels.TurnFor, or anywhere, a reply in a
// thread they started (msgmodels.AnswerFor).
func (api *API) toWake(ctx context.Context, m *msgmodels.Message) []agent.Agent {
	records, err := agent.ListAgents(ctx, api.db)
	if err != nil {
		log.Warn().Err(err).Msg("cannot list agents for the wake hook")
		return nil
	}

	root, err := api.threadRoot(ctx, m)
	if err != nil {
		log.Warn().Err(err).Int64("message", m.ID).Msg("cannot read the thread's root for the wake hook")
		root = nil
	}

	woken := []agent.Agent{}
	for i := range records {
		record := &records[i]

		reader := &msgmodels.Participant{Name: record.Name, Agent: record.Home}
		if msgmodels.AnswerFor(m, root, reader) {
			woken = api.wakeIfAsleep(ctx, woken, record)
			continue
		}

		if !msgmodels.TurnFor(m, reader) {
			continue
		}

		tmpl, err := LoadTemplate(api.dir, record.Role)
		if err != nil {
			log.Warn().Err(err).Str("agent", record.Name).Msg("the wake hook cannot read the agent's template; it reads only its own channels")
			tmpl = nil
		}

		patterns, err := api.Reads(ctx, record, tmpl)
		if err != nil {
			log.Warn().Err(err).Str("agent", record.Name).Msg("cannot read the agent's channels for the wake hook")
			continue
		}

		if !matchesAny(patterns, m.Channel) {
			continue
		}

		if fromSystem(m) {
			manages, err := api.managesChannel(ctx, record, m)
			if err != nil {
				log.Warn().Err(err).Str("agent", record.Name).Msg("cannot read the agent's tasks for the wake hook")
				continue
			}

			if !manages {
				continue
			}
		}

		woken = api.wakeIfAsleep(ctx, woken, record)
	}

	return woken
}

// wakeIfAsleep appends record to woken when it is asleep.
func (api *API) wakeIfAsleep(ctx context.Context, woken []agent.Agent, record *agent.Agent) []agent.Agent {
	state, _, err := api.state(ctx, record)
	if err != nil {
		log.Warn().Err(err).Str("agent", record.Name).Msg("cannot read the agent's state for the wake hook")
		return woken
	}

	if state != StateAsleep {
		return woken
	}

	return append(woken, *record)
}

// maxThreadDepth bounds following reply_to to a thread's root.
const maxThreadDepth = 100

// threadRoot is the message that starts m's thread, following reply_to;
// nil when m is no reply.
func (api *API) threadRoot(ctx context.Context, m *msgmodels.Message) (*msgmodels.Message, error) {
	if m.ReplyTo == nil {
		return nil, nil
	}

	id := *m.ReplyTo
	for depth := 0; depth < maxThreadDepth; depth++ {
		parent := &msgmodels.Message{}

		err := api.db.NewSelect().Model(parent).Where("id = ?", id).Scan(ctx)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		if parent.ReplyTo == nil {
			return parent, nil
		}

		id = *parent.ReplyTo
	}

	return nil, ez.New(ez.EINVALID, "the thread is deeper than the wake hook follows", nil)
}

// managesChannel reports whether agc's post m is in #<task> of a task
// record manages.
func (api *API) managesChannel(ctx context.Context, record *agent.Agent, m *msgmodels.Message) (bool, error) {
	managed, err := api.managed(ctx, record)
	if err != nil {
		return false, ez.Wrap(err)
	}

	for i := range managed {
		if messaging.TaskChannel(managed[i].Name) == m.Channel {
			return true, nil
		}
	}

	return false, nil
}

// isMain is messaging's check for the main role (ADR 0016 item 9): home
// is the caller's home sandbox.
func (api *API) isMain(home string) bool {
	record, err := agent.GetAgentByHome(context.Background(), api.db, home)
	if err != nil {
		return false
	}

	return strings.EqualFold(record.Role, MainRole)
}
