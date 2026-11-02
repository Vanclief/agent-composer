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
	return m.From == msgmodels.SystemName && !m.Human && m.Task == "" && m.Agent == ""
}

// The posts of agc in #<task> (core/resources/tasks) that are a turn for
// the task's manager: a delivery to review, and an accept to act on (the
// reviewer merges after the owner accepts).
const (
	deliveredPrefix = "Delivered: "
	acceptedPrefix  = "Accepted by "
)

// forManager reports whether agc's post m may wake the manager of its
// task: a delivery or an accept (managesChannel then leaves out the
// manager's own accept).
func forManager(m *msgmodels.Message) bool {
	return strings.HasPrefix(m.Body, deliveredPrefix) || strings.HasPrefix(m.Body, acceptedPrefix)
}

// onPost is the wake hook (ADR 0016 item 6): every asleep agent that reads
// the message's channel is woken, except for an agent's own posts and
// agc's own posts, which wake only the manager of the task whose channel
// they are in, and only for a delivery or an accept (forManager). Wakes
// run in the background so a post never waits on a VM.
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

// toWake returns the asleep agents whose channels match the message's,
// leaving out the agent that posted it.
func (api *API) toWake(ctx context.Context, m *msgmodels.Message) []agent.Agent {
	records, err := agent.ListAgents(ctx, api.db)
	if err != nil {
		log.Warn().Err(err).Msg("cannot list agents for the wake hook")
		return nil
	}

	woken := []agent.Agent{}
	for i := range records {
		record := &records[i]

		if m.Agent != "" && m.Agent == record.Home {
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

		state, _, err := api.state(ctx, record)
		if err != nil {
			log.Warn().Err(err).Str("agent", record.Name).Msg("cannot read the agent's state for the wake hook")
			continue
		}

		if state != StateAsleep {
			continue
		}

		woken = append(woken, *record)
	}

	return woken
}

// managesChannel reports whether agc's post m is in #<task> of a task
// record manages and is not the agent's own accept ("Accepted by
// <agent>."), which needs no turn.
func (api *API) managesChannel(ctx context.Context, record *agent.Agent, m *msgmodels.Message) (bool, error) {
	if m.Body == acceptedPrefix+record.Name+"." {
		return false, nil
	}

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
