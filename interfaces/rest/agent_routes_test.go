package rest

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/executor/sandbox"
)

// homeRuntime is fakeRuntime with a status per sandbox: running once
// created or started, stopped once stopped, gone once removed. A non-nil
// gate holds every Create until it is closed, so a test sees a home
// still booting.
type homeRuntime struct {
	fakeRuntime

	mu       sync.Mutex
	statuses map[string]string
	gate     chan struct{}
}

func (f *homeRuntime) set(name string, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses[name] = status
}

func (f *homeRuntime) Create(ctx context.Context, spec sandbox.Spec) (sandboxes.Sandbox, error) {
	if f.gate != nil {
		<-f.gate
	}
	f.set(spec.Name, "running")
	return f.sb, nil
}

func (f *homeRuntime) Start(ctx context.Context, name string, secrets []sandbox.Secret) error {
	f.set(name, "running")
	return nil
}

func (f *homeRuntime) Stop(ctx context.Context, name string) error {
	f.set(name, "stopped")
	return nil
}

func (f *homeRuntime) Remove(ctx context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.statuses, name)
	return nil
}

func (f *homeRuntime) Status(ctx context.Context, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, ok := f.statuses[name]
	if !ok {
		return "", ez.New(ez.ENOTFOUND, "cannot find sandbox "+name, nil)
	}
	return status, nil
}

// ADR 0016: the agents API, over the templates the repo ships.
func TestAgentRoutes(t *testing.T) {
	secretsPath := filepath.Join(t.TempDir(), "secrets.env")
	err := os.WriteFile(secretsPath, []byte("CLAUDE_CODE_OAUTH_TOKEN=token\nGITHUB_TOKEN=ghp_value\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// The gate holds the home's boot until the wake has answered
	// "waking".
	rt := &homeRuntime{fakeRuntime: fakeRuntime{sb: &fakeSandbox{}}, statuses: map[string]string{}, gate: make(chan struct{})}
	routes := newTaskRoutes(t, rt, secretsPath)
	var release sync.Once
	openGate := func() {
		release.Do(func() {
			close(rt.gate)
		})
	}
	// Registered after the routes' cleanup, so it runs first and a
	// failed check never leaves that cleanup waiting on a held boot.
	t.Cleanup(openGate)

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/agents", "", http.StatusOK, `{"agents":[]}`},
		{http.MethodPost, "/api/agents", `{}`, http.StatusBadRequest, "role is required"},
		{http.MethodPost, "/api/agents", `{"role":"Writer"}`, http.StatusBadRequest, "writer.yaml"},
		{http.MethodPost, "/api/agents", `{"role":"Reviewer","harness":"vim"}`, http.StatusBadRequest, "unknown harness vim"},
		{http.MethodPost, "/api/agents", `{"role":"Reviewer"}`, http.StatusCreated, `"name":"reviewer-1","role":"Reviewer","kept":false,"harness":"claude_code","model":"claude-opus-5-5","effort":"","owner":"franco","state":"asleep","home":null,"tasks":[],"conversations":[{"channel":"@franco/reviewer-1"`},
		{http.MethodPost, "/api/agents", `{"role":"reviewer","harness":"codex_cli","model":"gpt-5"}`, http.StatusCreated, `"name":"reviewer-2","role":"reviewer","kept":false,"harness":"codex_cli","model":"gpt-5"`},
		{http.MethodGet, "/api/agents", "", http.StatusOK, `"name":"reviewer-2"`},
		{http.MethodGet, "/api/agents/reviewer-1", "", http.StatusOK, `"last_active_at":null`},
		{http.MethodGet, "/api/agents/nobody", "", http.StatusNotFound, "agent nobody not found"},

		{http.MethodPost, "/api/agents/reviewer-1/keep", `{"name":"Ada"}`, http.StatusBadRequest, "lowercase letters, digits and dashes"},
		{http.MethodPost, "/api/agents/reviewer-1/keep", `{"name":"reviewer-2"}`, http.StatusConflict, "agent reviewer-2 already exists"},
		{http.MethodPost, "/api/agents/reviewer-1/keep", `{"name":"ada","role":"main"}`, http.StatusOK, `"name":"ada","role":"main","kept":true`},
		{http.MethodGet, "/api/agents/reviewer-1", "", http.StatusNotFound, ""},
		{http.MethodPost, "/api/agents", `{"role":"main"}`, http.StatusCreated, `"name":"main-1"`},

		{http.MethodPost, "/api/agents/ada/sleep", "", http.StatusConflict, "only an awake agent can sleep"},
		{http.MethodPost, "/api/agents/ada/wake", "", http.StatusOK, `"state":"waking","home":"ada"`},
	})

	openGate()
	routes.server.SandboxesAPI.Wait()

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/agents/ada", "", http.StatusOK, `"state":"awake"`},
		// Wake on an awake agent is a no-op.
		{http.MethodPost, "/api/agents/ada/wake", "", http.StatusOK, `"state":"awake"`},
		// The home is a sandbox with no task, linking back to its agent.
		{http.MethodGet, "/api/sandboxes/ada", "", http.StatusOK, `"task":"","agent":"ada","name":"ada"`},
		{http.MethodPost, "/api/agents/ada/sleep", "", http.StatusOK, `"state":"asleep","home":"ada"`},
		{http.MethodPost, "/api/agents/ada/wake", "", http.StatusOK, `"state":"awake"`},
		{http.MethodDelete, "/api/agents/ada", "", http.StatusOK, `{"deleted":true,"name":"ada"}`},
		{http.MethodGet, "/api/agents/ada", "", http.StatusNotFound, ""},
		{http.MethodGet, "/api/sandboxes/ada", "", http.StatusNotFound, ""},
		{http.MethodDelete, "/api/agents/ada", "", http.StatusNotFound, ""},
		{http.MethodGet, "/api/agents", "", http.StatusOK, `"name":"reviewer-2"`},
	})
}
