package agents

import (
	"context"
	"slices"
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

// onPost is the wake hook (ADR 0016 item 6): every asleep agent the
// message is a turn for (turnFor, already stored as turn_for before the
// message was published) is woken. Wakes run in the background so a post
// never waits on a VM.
func (api *API) onPost(ctx context.Context, m *msgmodels.Message) {
	for i := range m.TurnFor {
		name := m.TurnFor[i]

		record, err := agent.GetAgentByName(ctx, api.db, name)
		if err != nil {
			log.Warn().Err(err).Str("agent", name).Msg("cannot read the agent for the wake hook")
			continue
		}

		if !api.asleep(ctx, record) {
			continue
		}

		api.wakes.Add(1)
		go func() {
			defer api.wakes.Done()

			err := api.wake(context.WithoutCancel(ctx), record)
			if err != nil {
				log.Warn().Err(err).Str("agent", record.Name).Str("channel", m.Channel).Msg("cannot wake the agent for a message")
			}
		}()
	}
}

// turnNames is messaging's TurnsFunc: the names of the agents m is a
// turn for, in name order, stored as the message's turn_for.
func (api *API) turnNames(ctx context.Context, m *msgmodels.Message) []string {
	records := api.turnFor(ctx, m)

	names := []string{}
	for i := range records {
		names = append(names, records[i].Name)
	}

	slices.Sort(names)

	return names
}

// turnFor returns the agents, asleep or awake, for which the message is a
// turn: in a channel they read, by msgmodels.TurnFor, a person's post,
// another agent's question or post naming them, never their own post; or
// anywhere, a reply in a thread they started (msgmodels.AnswerFor), so the
// asker hears the answer. agc's own posts are a turn only for the manager
// of the task whose channel they are in, and only for a delivery or an
// accept (forManager).
func (api *API) turnFor(ctx context.Context, m *msgmodels.Message) []agent.Agent {
	if fromSystem(m) && !forManager(m) {
		return nil
	}

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

	turns := []agent.Agent{}
	for i := range records {
		record := &records[i]

		reader := &msgmodels.Participant{Name: record.Name, Agent: record.Home}
		if msgmodels.AnswerFor(m, root, reader) {
			turns = append(turns, *record)
			continue
		}

		if !msgmodels.TurnFor(m, reader) {
			continue
		}

		tmpl, err := api.template(record.Role)
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

		turns = append(turns, *record)
	}

	return turns
}

// asleep reports whether record's home is asleep.
func (api *API) asleep(ctx context.Context, record *agent.Agent) bool {
	state, _, err := api.state(ctx, record)
	if err != nil {
		log.Warn().Err(err).Str("agent", record.Name).Msg("cannot read the agent's state for the wake hook")
		return false
	}

	return state == StateAsleep
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
