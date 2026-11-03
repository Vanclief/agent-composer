package sandboxes

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/account"
	"github.com/vanclief/agent-composer/models/agent"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/runtime/harnesses"
	"github.com/vanclief/agent-composer/secrets"
	"github.com/vanclief/agent-composer/task"
)

// harnessReads is the credential variables harness h reads, none for an
// unknown harness or a person.
func harnessReads(h agent.Harness) []string {
	for _, entry := range harnesses.ListHarnessReads() {
		if entry.ID == h {
			return entry.Reads
		}
	}

	return []string{}
}

// withDefaultAccount binds the default account of the provider serving
// harness when the spec's secrets set none of the variables the harness
// reads (ADR 0018): it appends "<account.secret> as <variable>", the
// variable per the account's kind. An explicit entry wins; no default, or
// a harness no provider serves (a person's task, pi), leaves the spec as
// it is. The spec returned is the one to store and boot from.
func (api *API) withDefaultAccount(ctx context.Context, spec task.Spec, harness agent.Harness) (task.Spec, error) {
	provider, ok := account.ProviderOf(harness)
	if !ok {
		return spec, nil
	}

	reads := harnessReads(harness)

	requests, err := spec.SecretRequests()
	if err != nil {
		return spec, ez.Wrap(err)
	}

	for _, request := range requests {
		if slices.Contains(reads, request.Env) {
			return spec, nil
		}
	}

	def, err := account.GetDefault(ctx, api.db, provider)
	if err != nil {
		return spec, ez.Wrap(err)
	}

	if def == nil {
		return spec, nil
	}

	variable := def.Provider.Variable(def.Kind)
	if variable == "" {
		return spec, nil
	}

	bound := spec
	bound.Secrets = append(slices.Clone(spec.Secrets), def.Secret+" as "+variable)

	return bound, nil
}

// harnessOf is the harness a sandbox runs: its agent's for a home, the
// task's assignee when that is a harness, else none.
func (api *API) harnessOf(ctx context.Context, record *taskmodels.Sandbox, owner *taskmodels.Task) (agent.Harness, error) {
	if record.Agent == "" {
		return taskHarness(owner), nil
	}

	found, err := agent.GetAgentByName(ctx, api.db, record.Agent)
	if ez.ErrorCode(err) == ez.ENOTFOUND {
		return "", nil
	}
	if err != nil {
		return "", ez.Wrap(err)
	}

	return found.Harness, nil
}

// taskHarness is the task's assignee when it is a harness, else none.
func taskHarness(owner *taskmodels.Task) agent.Harness {
	if !IsHarness(owner.Assignee) {
		return ""
	}

	return agent.Harness(owner.Assignee)
}

// SandboxesUsingSecret lists the sandboxes running or booting now whose
// stored spec names the secret, by name, so the accounts API does not
// delete an account from under them.
func (api *API) SandboxesUsingSecret(ctx context.Context, secretName string) ([]string, error) {
	records, err := taskmodels.ListSandboxes(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	names := []string{}
	for i := range records {
		record := &records[i]

		var spec task.Spec
		err := json.Unmarshal(record.Spec, &spec)
		if err != nil {
			continue
		}

		requests, err := spec.SecretRequests()
		if err != nil {
			continue
		}

		named := slices.ContainsFunc(requests, func(r secrets.Request) bool {
			return r.Name == secretName
		})
		if !named {
			continue
		}

		if api.isBooting(record.Name) || api.running(ctx, record) {
			names = append(names, record.Name)
		}
	}

	return names, nil
}
