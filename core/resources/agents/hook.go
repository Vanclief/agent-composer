package agents

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/vanclief/agent-composer/models/agent"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// fromSystem reports whether agc itself posted m.
func fromSystem(m *msgmodels.Message) bool {
	return m.From == msgmodels.SystemName && !m.Human && m.Task == "" && m.Agent == ""
}

// onPost is the wake hook (ADR 0016 item 6): every asleep agent that reads
// the message's channel is woken, except by an agent's own posts. agc's
// own posts wake only the main role (ADR 0015: triggers land as agc's
// messages, ADR 0014), and agc's sleep notice wakes nobody, or main's own
// sleep would wake it. Wakes run in the background so a post never waits
// on a VM.
func (api *API) onPost(ctx context.Context, m *msgmodels.Message) {
	system := fromSystem(m)
	if system && strings.TrimSpace(m.Body) == SleepNotice {
		return
	}

	woken := api.toWake(ctx, m, system)

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
// leaving out the agent that posted it, and, for one of agc's own posts
// (system), every agent but main.
func (api *API) toWake(ctx context.Context, m *msgmodels.Message, system bool) []agent.Agent {
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

		if system && !strings.EqualFold(record.Role, MainRole) {
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

// isMain is messaging's check for the main role (ADR 0016 item 9): home
// is the caller's home sandbox.
func (api *API) isMain(home string) bool {
	record, err := agent.GetAgentByHome(context.Background(), api.db, home)
	if err != nil {
		return false
	}

	return strings.EqualFold(record.Role, MainRole)
}
