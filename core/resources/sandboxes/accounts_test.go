package sandboxes

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vanclief/agent-composer/executor/sandbox"
	accountmodels "github.com/vanclief/agent-composer/models/account"
	agentmodels "github.com/vanclief/agent-composer/models/agent"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
	"gopkg.in/yaml.v3"
)

// Made-up values: no test holds a real credential.
const accountSecrets = "GITHUB_TOKEN=ghp_value\nACCOUNT_WORK='made-up-work'\nACCOUNT_HOME='made-up-home'\nACCOUNT_CODEX='made-up-codex'\nMY_TOKEN=made-up-mine\n"

// insertAccount inserts an account row; its value is in accountSecrets.
func insertAccount(t *testing.T, env *testEnv, name string, provider accountmodels.Provider, kind accountmodels.Kind, isDefault bool) {
	t.Helper()

	record := &accountmodels.Account{Name: name, Provider: provider, Kind: kind, Secret: accountmodels.SecretName(name), Default: isDefault}
	err := record.Insert(context.Background(), env.db)
	if err != nil {
		t.Fatal(err)
	}
}

func newAccountsEnv(t *testing.T) *testEnv {
	t.Helper()

	env := newTestEnv(t)
	err := os.WriteFile(env.api.secretsPath, []byte(accountSecrets), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	insertAccount(t, env, "work", accountmodels.ProviderAnthropic, accountmodels.KindOAuth, true)
	insertAccount(t, env, "home", accountmodels.ProviderAnthropic, accountmodels.KindAPIKey, false)
	insertAccount(t, env, "codex", accountmodels.ProviderOpenAI, accountmodels.KindAPIKey, true)

	return env
}

func insertAssigned(t *testing.T, env *testEnv, name string, assignee string) *taskmodels.Task {
	t.Helper()

	record := &taskmodels.Task{Title: "Title of " + name, Name: name, Brief: "Do " + name + ".", Assignee: assignee}
	err := record.Insert(context.Background(), env.db)
	if err != nil {
		t.Fatal(err)
	}

	return record
}

// storedSecrets is the secrets list on the sandbox's row.
func storedSecrets(t *testing.T, env *testEnv, name string) []string {
	t.Helper()

	record, err := taskmodels.GetSandboxByName(context.Background(), env.db, name)
	if err != nil {
		t.Fatal(err)
	}

	var spec task.Spec
	err = json.Unmarshal(record.Spec, &spec)
	if err != nil {
		t.Fatal(err)
	}

	return spec.Secrets
}

// ADR 0018: a task whose spec names no credential of its harness gets its
// provider's default account, as "<secret> as <variable>".
func TestAllocateBindsTheProviderDefault(t *testing.T) {
	cases := []struct {
		name     string
		assignee string
		secrets  []string
		stored   []string
		guest    []sandbox.Secret
	}{
		{
			name:     "claude",
			assignee: "claude_code",
			secrets:  []string{"GITHUB_TOKEN"},
			stored:   []string{"GITHUB_TOKEN", "ACCOUNT_WORK as CLAUDE_CODE_OAUTH_TOKEN"},
			guest: []sandbox.Secret{
				{Name: "GITHUB_TOKEN", Value: "ghp_value", Hosts: []string{"api.github.com", "github.com"}},
				{Name: "CLAUDE_CODE_OAUTH_TOKEN", Value: "made-up-work", Hosts: []string{"api.anthropic.com"}},
			},
		},
		{
			name:     "codex",
			assignee: "codex_cli",
			secrets:  nil,
			stored:   []string{"ACCOUNT_CODEX as CODEX_API_KEY"},
			guest:    []sandbox.Secret{{Name: "CODEX_API_KEY", Value: "made-up-codex", Hosts: []string{"api.openai.com"}}},
		},
		{
			// An explicit entry for a variable the harness reads wins.
			name:     "explicit",
			assignee: "claude_code",
			secrets:  []string{"MY_TOKEN as ANTHROPIC_API_KEY"},
			stored:   []string{"MY_TOKEN as ANTHROPIC_API_KEY"},
			guest:    []sandbox.Secret{{Name: "ANTHROPIC_API_KEY", Value: "made-up-mine", Hosts: []string{"api.anthropic.com"}}},
		},
		{
			// Another account named explicitly wins too.
			name:     "other-account",
			assignee: "claude_code",
			secrets:  []string{"ACCOUNT_HOME as ANTHROPIC_API_KEY"},
			stored:   []string{"ACCOUNT_HOME as ANTHROPIC_API_KEY"},
			guest:    []sandbox.Secret{{Name: "ANTHROPIC_API_KEY", Value: "made-up-home", Hosts: []string{"api.anthropic.com"}}},
		},
		{
			// A person's task runs no harness: nothing is bound.
			name:     "person",
			assignee: "franco",
			secrets:  []string{"GITHUB_TOKEN"},
			stored:   []string{"GITHUB_TOKEN"},
			guest:    []sandbox.Secret{{Name: "GITHUB_TOKEN", Value: "ghp_value", Hosts: []string{"api.github.com", "github.com"}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newAccountsEnv(t)
			owner := insertAssigned(t, env, "fix", tc.assignee)

			_, err := env.api.AllocateFor(context.Background(), owner, task.Sandbox{Spec: task.Spec{Secrets: tc.secrets}})
			if err != nil {
				t.Fatal(err)
			}
			env.api.Wait()

			got := storedSecrets(t, env, "fix")
			if !reflect.DeepEqual(got, tc.stored) {
				t.Fatalf("stored secrets = %q, want %q", got, tc.stored)
			}

			created := env.rt.created[0]
			if !reflect.DeepEqual(created.Secrets, tc.guest) {
				t.Fatalf("guest secrets = %+v", created.Secrets)
			}
		})
	}
}

// No default for the provider: the spec stays as it was and the sandbox
// gets no credential, as before accounts.
func TestAllocateWithoutDefaultChangesNothing(t *testing.T) {
	env := newAccountsEnv(t)
	_, err := env.db.NewUpdate().Model((*accountmodels.Account)(nil)).Set("is_default = ?", false).Where("1 = 1").Exec(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	owner := insertAssigned(t, env, "fix", "claude_code")
	_, err = env.api.AllocateFor(context.Background(), owner, task.Sandbox{Spec: task.Spec{Secrets: []string{"GITHUB_TOKEN"}}})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()

	got := storedSecrets(t, env, "fix")
	if !reflect.DeepEqual(got, []string{"GITHUB_TOKEN"}) {
		t.Fatalf("stored secrets = %q", got)
	}
	if len(env.rt.created[0].Secrets) != 1 {
		t.Fatalf("guest secrets = %+v", env.rt.created[0].Secrets)
	}
}

// A persistent agent's home gets its harness's provider default.
func TestAllocateHomeBindsTheProviderDefault(t *testing.T) {
	env := newAccountsEnv(t)

	_, err := env.api.AllocateHome(context.Background(), HomeRequest{Agent: "reviewer-1", Harness: agentmodels.HarnessCodexCLI, Command: "agc agent run --agent reviewer-1"})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()

	got := storedSecrets(t, env, "reviewer-1")
	if !reflect.DeepEqual(got, []string{"ACCOUNT_CODEX as CODEX_API_KEY"}) {
		t.Fatalf("stored secrets = %q", got)
	}
}

// The shipped templates (agents/*.yaml) name no credential, so a home
// allocated from one gets the anthropic oauth default next to GITHUB_TOKEN.
func TestAllocateHomeFromShippedTemplateBindsTheDefault(t *testing.T) {
	for _, role := range []string{"reviewer", "main", "developer", "designer"} {
		t.Run(role, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "..", "agents", role+".yaml"))
			if err != nil {
				t.Fatal(err)
			}

			var tmpl struct {
				Harness agentmodels.Harness `yaml:"harness"`
				Sandbox task.Sandbox        `yaml:"sandbox"`
			}
			err = yaml.Unmarshal(data, &tmpl)
			if err != nil {
				t.Fatal(err)
			}
			if tmpl.Harness != agentmodels.HarnessClaudeCode {
				t.Fatalf("harness = %q", tmpl.Harness)
			}
			if !reflect.DeepEqual(tmpl.Sandbox.Secrets, []string{"GITHUB_TOKEN"}) {
				t.Fatalf("template secrets = %q", tmpl.Sandbox.Secrets)
			}

			env := newAccountsEnv(t)
			name := role + "-1"
			_, err = env.api.AllocateHome(context.Background(), HomeRequest{Agent: name, Harness: tmpl.Harness, Sandbox: tmpl.Sandbox, Command: "agc agent run --agent " + name})
			if err != nil {
				t.Fatal(err)
			}
			env.api.Wait()

			got := storedSecrets(t, env, name)
			if !reflect.DeepEqual(got, []string{"GITHUB_TOKEN", "ACCOUNT_WORK as CLAUDE_CODE_OAUTH_TOKEN"}) {
				t.Fatalf("stored secrets = %q", got)
			}
		})
	}
}

// A sandbox allocated before its provider had a default gets it on start,
// recorded on the row; the account then counts as in use while it runs.
func TestStartBindsADefaultSetSinceAllocation(t *testing.T) {
	env := newAccountsEnv(t)
	ctx := context.Background()
	_, err := env.db.NewUpdate().Model((*accountmodels.Account)(nil)).Set("is_default = ?", false).Where("1 = 1").Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	owner := insertAssigned(t, env, "fix", "claude_code")
	_, err = env.api.AllocateFor(ctx, owner, task.Sandbox{Spec: task.Spec{}})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()

	used, err := env.api.SandboxesUsingSecret(ctx, "ACCOUNT_WORK")
	if err != nil {
		t.Fatal(err)
	}
	if len(used) != 0 {
		t.Fatalf("in use by %q before any default", used)
	}

	_, err = env.api.Stop(ctx, nil, &StopRequest{Name: "fix"})
	if err != nil {
		t.Fatal(err)
	}

	work, err := accountmodels.GetByName(ctx, env.db, "work")
	if err != nil {
		t.Fatal(err)
	}
	err = work.SetDefault(ctx, env.db, true)
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.api.Start(ctx, nil, &StartRequest{Name: "fix"})
	if err != nil {
		t.Fatal(err)
	}

	want := []sandbox.Secret{{Name: "CLAUDE_CODE_OAUTH_TOKEN", Value: "made-up-work", Hosts: []string{"api.anthropic.com"}}}
	if !reflect.DeepEqual(env.rt.started[0], want) {
		t.Fatalf("started with %+v", env.rt.started[0])
	}

	got := storedSecrets(t, env, "fix")
	if !reflect.DeepEqual(got, []string{"ACCOUNT_WORK as CLAUDE_CODE_OAUTH_TOKEN"}) {
		t.Fatalf("stored secrets = %q", got)
	}

	used, err = env.api.SandboxesUsingSecret(ctx, "ACCOUNT_WORK")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(used, []string{"fix"}) {
		t.Fatalf("in use by %q", used)
	}

	_, err = env.api.Stop(ctx, nil, &StopRequest{Name: "fix"})
	if err != nil {
		t.Fatal(err)
	}

	used, err = env.api.SandboxesUsingSecret(ctx, "ACCOUNT_WORK")
	if err != nil {
		t.Fatal(err)
	}
	if len(used) != 0 {
		t.Fatalf("a stopped sandbox counts as using the account: %q", used)
	}
}
