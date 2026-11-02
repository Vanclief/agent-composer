// Package agents manages persistent agents (ADR 0016): rows, not
// processes, born numbered from a role template under agents/, asleep by
// default, woken in a home sandbox when someone talks to them. Their state
// is computed from the home sandbox, never stored.
package agents

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/models/agent"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// An agent's state, computed from its home sandbox (ADR 0016 item 4). Any
// other sandbox status (failed, stopping, unknown) is shown verbatim.
const (
	StateAsleep = "asleep"
	StateWaking = "waking"
	StateAwake  = "awake"
	// StateDeleted is only ever sent in agent.state, for an agent that is
	// gone.
	StateDeleted = "deleted"
)

// MainRole reads, waits on and posts in every channel (ADR 0016 item 9).
const MainRole = "main"

type API struct {
	db       bun.IDB
	tasks    *tasks.API
	sbx      *sandboxes.API
	msg      *messaging.Service
	defaults controller.DefaultsSettings
	// dir holds the templates, <role>.yaml, read on every request.
	dir string

	// naming serializes numbering and renaming agents.
	naming sync.Mutex
	// waking serializes wakes and sleeps, so two messages do not boot two
	// homes for one agent.
	waking sync.Mutex
	// wakes counts wakes started by the hook, for Wait.
	wakes sync.WaitGroup

	// events gets agent.state, nil drops them.
	events *events.Bus
}

// StateEvent is the data of agent.state.
type StateEvent struct {
	Agent *AgentView `json:"agent"`
}

// NewAPI returns the agents API over the templates in dir.
func NewAPI(ctrl *controller.Controller, tasksAPI *tasks.API, msg *messaging.Service, dir string) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	return New(ctrl.DB, tasksAPI, msg, ctrl.Defaults(), dir)
}

// New is NewAPI over a bare database, for tests. It wakes agents on posts
// (the wake hook), answers messaging's main check, serves the agent routes
// on homes' sockets and follows their sandbox.status.
func New(db bun.IDB, tasksAPI *tasks.API, msg *messaging.Service, defaults controller.DefaultsSettings, dir string) *API {
	if tasksAPI == nil {
		panic("Tasks reference is nil")
	}
	if msg == nil {
		panic("Messaging reference is nil")
	}

	api := &API{
		db:       db,
		tasks:    tasksAPI,
		sbx:      tasksAPI.Sandboxes,
		msg:      msg,
		defaults: defaults,
		dir:      dir,
	}

	msg.OnPost(api.onPost)
	msg.SetMainAgent(api.isMain)
	api.sbx.SetHomes(homes{api: api})
	api.sbx.OnStatus(api.onSandboxStatus)

	return api
}

// SetEvents publishes agent.state on bus from now on.
func (api *API) SetEvents(bus *events.Bus) {
	api.events = bus
}

// Wait blocks until every wake the hook started has returned.
func (api *API) Wait() {
	api.wakes.Wait()
}

// AgentTask is a task the agent manages, its status verbatim.
type AgentTask struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Conversation is a channel the agent talks in and its last message's
// time.
type Conversation struct {
	Channel string    `json:"channel"`
	LastAt  time.Time `json:"last_at"`
}

// AgentView is an agent with its computed state, tasks and conversations,
// the shape of panel/src/api/agents.ts.
type AgentView struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Kept    bool   `json:"kept"`
	Harness string `json:"harness"`
	Model   string `json:"model"`
	// Effort is the harness's reasoning effort, empty for its default.
	Effort string `json:"effort"`
	Owner  string `json:"owner"`
	// State is asleep, waking or awake, or the home sandbox's own status
	// when it is anything else.
	State string `json:"state"`
	// Home is the home sandbox's name, null before the first wake.
	Home *string `json:"home"`
	// Tasks are the tasks whose manager is the agent.
	Tasks []AgentTask `json:"tasks"`
	// Conversations are the DM with the owner, then #<task> for each
	// task, then any other channel the agent has posted in.
	Conversations []Conversation `json:"conversations"`
	LastActiveAt  *time.Time     `json:"last_active_at"`
	CreatedAt     time.Time      `json:"created_at"`
}

// DMChannel is the agent's direct message channel with its owner,
// @<owner>/<name> (ADR 0011), empty when it has no owner.
func DMChannel(record *agent.Agent) string {
	if record.Owner == "" {
		return ""
	}

	return "@" + record.Owner + "/" + record.Name
}

// stateOf maps the home sandbox's status to the agent's state.
func stateOf(record *agent.Agent, home *sandboxes.SandboxView) string {
	if record.Home == "" || home == nil {
		return StateAsleep
	}

	switch home.Status {
	case sandboxes.StatusStopped:
		return StateAsleep
	case sandboxes.StatusStarting:
		return StateWaking
	case sandboxes.StatusRunning:
		return StateAwake
	}

	return home.Status
}

// home reads the agent's home sandbox, nil when it has none or it is gone.
func (api *API) home(ctx context.Context, record *agent.Agent) (*sandboxes.SandboxView, error) {
	if record.Home == "" {
		return nil, nil
	}

	view, err := api.sbx.Home(ctx, record.Home)
	if ez.ErrorCode(err) == ez.ENOTFOUND {
		return nil, nil
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return view, nil
}

// state is the agent's state, read from its home now.
func (api *API) state(ctx context.Context, record *agent.Agent) (string, *sandboxes.SandboxView, error) {
	home, err := api.home(ctx, record)
	if err != nil {
		return "", nil, ez.Wrap(err)
	}

	return stateOf(record, home), home, nil
}

func (api *API) find(ctx context.Context, name string) (*agent.Agent, error) {
	if strings.TrimSpace(name) == "" {
		return nil, ez.New(ez.EINVALID, "name is required", nil)
	}

	return agent.GetAgentByName(ctx, api.db, name)
}

// managed returns the tasks whose manager is the agent, oldest first,
// done ones included: a task stays in its manager's Reads for good, so the
// "Accepted by" agc posts when the owner accepts wakes the manager (the
// reviewer merges then), and a later message in #<task> still reaches it.
func (api *API) managed(ctx context.Context, record *agent.Agent) ([]taskmodels.Task, error) {
	records := []taskmodels.Task{}

	err := api.db.NewSelect().Model(&records).Where("manager = ?", record.Name).Order("created_at ASC", "id ASC").Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return records, nil
}

// view builds the agent's view with its state read from the home now.
func (api *API) view(ctx context.Context, record *agent.Agent) (*AgentView, error) {
	state, _, err := api.state(ctx, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.viewWith(ctx, record, state)
}

// viewWith builds the agent's view with a known state.
func (api *API) viewWith(ctx context.Context, record *agent.Agent, state string) (*AgentView, error) {
	managed, err := api.managed(ctx, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	agentTasks := []AgentTask{}
	for i := range managed {
		taskView, err := api.tasks.Get(ctx, nil, &tasks.GetRequest{ID: managed[i].ID})
		if err != nil {
			return nil, ez.Wrap(err)
		}

		agentTasks = append(agentTasks, AgentTask{Name: taskView.Name, Status: taskView.Status})
	}

	conversations, lastActive, err := api.conversations(ctx, record, managed)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if record.LastActiveAt != nil && (lastActive == nil || record.LastActiveAt.After(*lastActive)) {
		lastActive = record.LastActiveAt
	}

	var home *string
	if record.Home != "" {
		name := record.Home
		home = &name
	}

	return &AgentView{
		Name:          record.Name,
		Role:          record.Role,
		Kept:          record.Kept,
		Harness:       string(record.Harness),
		Model:         record.Model,
		Effort:        record.Effort,
		Owner:         record.Owner,
		State:         state,
		Home:          home,
		Tasks:         agentTasks,
		Conversations: conversations,
		LastActiveAt:  lastActive,
		CreatedAt:     record.CreatedAt,
	}, nil
}

// channelStat is a channel's last message time.
type channelStat struct {
	Channel string    `bun:"channel"`
	LastAt  time.Time `bun:"last_at"`
}

// conversations lists the DM, then #<task> for each managed task, then the
// other channels the agent's participants posted in, newest first. A
// channel with no message yet gets the agent's or the task's creation time.
// lastActive is the newest message among them, nil when there is none.
func (api *API) conversations(ctx context.Context, record *agent.Agent, managed []taskmodels.Task) ([]Conversation, *time.Time, error) {
	type entry struct {
		channel string
		since   time.Time
	}

	fixed := []entry{}
	dm := DMChannel(record)
	if dm != "" {
		fixed = append(fixed, entry{channel: dm, since: record.CreatedAt})
	}
	for i := range managed {
		fixed = append(fixed, entry{channel: messaging.TaskChannel(managed[i].Name), since: managed[i].CreatedAt})
	}

	names := []string{}
	for _, e := range fixed {
		names = append(names, e.channel)
	}

	last := map[string]time.Time{}

	if len(names) > 0 {
		stats := []channelStat{}

		err := api.db.NewSelect().
			Model((*msgmodels.Message)(nil)).
			ColumnExpr("channel, MAX(created_at) AS last_at").
			Where("channel IN (?)", bun.In(names)).
			Group("channel").
			Scan(ctx, &stats)
		if err != nil {
			return nil, nil, ez.Wrap(err)
		}

		for _, stat := range stats {
			last[stat.Channel] = stat.LastAt
		}
	}

	others := []channelStat{}
	if record.Home != "" {
		posted := api.db.NewSelect().
			Model((*msgmodels.Message)(nil)).
			Column("channel").
			Where("agent = ?", record.Home)

		err := api.db.NewSelect().
			Model((*msgmodels.Message)(nil)).
			ColumnExpr("channel, MAX(created_at) AS last_at").
			Where("channel IN (?)", posted).
			Group("channel").
			Scan(ctx, &others)
		if err != nil {
			return nil, nil, ez.Wrap(err)
		}
	}

	var lastActive *time.Time
	mark := func(at time.Time) {
		if lastActive == nil || at.After(*lastActive) {
			copied := at
			lastActive = &copied
		}
	}

	conversations := []Conversation{}
	seen := map[string]bool{}
	for _, e := range fixed {
		if seen[e.channel] {
			continue
		}
		seen[e.channel] = true

		at, ok := last[e.channel]
		if ok {
			mark(at)
		} else {
			at = e.since
		}

		conversations = append(conversations, Conversation{Channel: e.channel, LastAt: at.UTC()})
	}

	sort.Slice(others, func(i, j int) bool {
		return others[i].LastAt.After(others[j].LastAt)
	})
	for _, stat := range others {
		if seen[stat.Channel] {
			continue
		}
		seen[stat.Channel] = true

		mark(stat.LastAt)
		conversations = append(conversations, Conversation{Channel: stat.Channel, LastAt: stat.LastAt.UTC()})
	}

	return conversations, lastActive, nil
}

// publish sends view as agent.state.
func (api *API) publish(view *AgentView) {
	if view == nil {
		return
	}

	api.events.Publish(events.Event{Type: events.AgentState, Data: StateEvent{Agent: view}})
}

// publishCurrent reads the agent's view and sends it.
func (api *API) publishCurrent(ctx context.Context, record *agent.Agent) {
	if api.events == nil {
		return
	}

	view, err := api.view(ctx, record)
	if err != nil {
		log.Warn().Err(err).Str("agent", record.Name).Msg("cannot read the agent for agent.state")
		return
	}

	api.publish(view)
}

// onSandboxStatus publishes agent.state whenever a home's sandbox.status
// is, with the state that status gives.
func (api *API) onSandboxStatus(view sandboxes.SandboxView) {
	if view.Agent == "" || api.events == nil || view.Status == sandboxes.StatusDeleted {
		return
	}

	ctx := context.Background()

	record, err := agent.GetAgentByName(ctx, api.db, view.Agent)
	if err != nil {
		return
	}

	if record.Home != view.Name {
		return
	}

	agentView, err := api.viewWith(ctx, record, stateOf(record, &view))
	if err != nil {
		log.Warn().Err(err).Str("agent", record.Name).Msg("cannot read the agent for agent.state")
		return
	}

	api.publish(agentView)
}
