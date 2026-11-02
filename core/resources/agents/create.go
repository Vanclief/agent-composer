package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/agent"
)

// CreateRequest makes a numbered agent from the template of role. An
// empty harness or model takes the template's, then the defaults (ADR
// 0009).
type CreateRequest struct {
	Role    string `json:"role"`
	Harness string `json:"harness"`
	Model   string `json:"model"`
}

func (r CreateRequest) Validate() error {
	role := strings.TrimSpace(r.Role)
	if role == "" {
		return ez.New(ez.EINVALID, "role is required", nil)
	}

	err := ValidateRole(role)
	if err != nil {
		return ez.Wrap(err)
	}

	harness := strings.TrimSpace(r.Harness)
	if harness != "" {
		err = agent.Harness(harness).Validate()
		if err != nil {
			return ez.New(ez.EINVALID, "unknown harness "+harness+": use claude_code, codex_cli or pi", err)
		}
	}

	return nil
}

// Create inserts <role lowercased>-<n>, n one more than the highest number
// drawn for the role, so a kept agent's number is never handed out again,
// even after it is renamed or given another role. It is asleep with no home; its owner is the default owner.
func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*AgentView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	role := strings.TrimSpace(request.Role)

	tmpl, err := LoadTemplate(api.dir, role)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	harness := firstNonEmpty(request.Harness, tmpl.Harness, api.defaults.Harness)

	model := strings.TrimSpace(request.Model)
	if model == "" && harness == tmpl.Harness {
		model = strings.TrimSpace(tmpl.Model)
	}
	if model == "" && harness == api.defaults.Harness {
		model = api.defaults.Model
	}

	idle := strings.TrimSpace(tmpl.Idle)
	if idle == "" {
		idle = "15m"
	}

	api.naming.Lock()
	defer api.naming.Unlock()

	series := strings.ToLower(role)

	number, err := agent.MaxNumber(ctx, api.db, series)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	// A name may already be taken by an agent kept under it.
	var record *agent.Agent
	for record == nil {
		number++
		name := fmt.Sprintf("%s-%d", series, number)

		err = agent.ValidateName(name)
		if err != nil {
			return nil, ez.New(ez.EINVALID, "role "+role+" is too long to name an agent from: "+ez.ErrorMessage(err), err)
		}

		_, err = agent.GetAgentByName(ctx, api.db, name)
		if err == nil {
			continue
		}
		if ez.ErrorCode(err) != ez.ENOTFOUND {
			return nil, ez.Wrap(err)
		}

		record = &agent.Agent{
			Name:    name,
			Role:    role,
			Number:  number,
			Series:  series,
			Harness: agent.Harness(harness),
			Model:   model,
			Owner:   api.defaults.Owner,
			Idle:    idle,
		}
	}

	err = record.Insert(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	view, err := api.view(ctx, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.publish(view)

	return view, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}
