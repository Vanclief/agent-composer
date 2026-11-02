package rest

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/executor/sandbox"
)

// statefulRuntime boots sandboxes as running, stops them for real and
// records the stops.
type statefulRuntime struct {
	sb *fakeSandbox

	mu       sync.Mutex
	statuses map[string]string
	stops    []string
}

func (f *statefulRuntime) Create(ctx context.Context, spec sandbox.Spec) (sandboxes.Sandbox, error) {
	f.mu.Lock()
	f.statuses[spec.Name] = "running"
	f.mu.Unlock()
	return f.sb, nil
}

func (f *statefulRuntime) Connect(ctx context.Context, name string) (sandboxes.Sandbox, error) {
	return f.sb, nil
}

func (f *statefulRuntime) Start(ctx context.Context, name string, secrets []sandbox.Secret) error {
	f.mu.Lock()
	f.statuses[name] = "running"
	f.mu.Unlock()
	return nil
}

func (f *statefulRuntime) Stop(ctx context.Context, name string) error {
	f.mu.Lock()
	f.statuses[name] = "stopped"
	f.stops = append(f.stops, name)
	f.mu.Unlock()
	return nil
}

func (f *statefulRuntime) Remove(ctx context.Context, name string) error {
	f.mu.Lock()
	delete(f.statuses, name)
	f.mu.Unlock()
	return nil
}

func (f *statefulRuntime) Status(ctx context.Context, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, ok := f.statuses[name]
	if !ok {
		return "", ez.New(ez.ENOTFOUND, "cannot find sandbox "+name, nil)
	}
	return status, nil
}

func (f *statefulRuntime) Stops() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	stops := append([]string(nil), f.stops...)
	slices.Sort(stops)
	return stops
}

// ADR 0010: criteria, deliver, review, reject, and accept stopping the
// task's sandboxes.
func TestTaskAcceptanceRoutes(t *testing.T) {
	rt := &statefulRuntime{sb: &fakeSandbox{}, statuses: map[string]string{}}
	routes := newTaskRoutes(t, rt, "")

	routes.check(t, []routeCase{
		{http.MethodPost, "/api/tasks", `{"title":"Stop icon","manager":"orchestrator","accept":["the icon is filled"," "],"sandbox":{}}`, http.StatusBadRequest, "accept[1] is empty"},
		{http.MethodPost, "/api/tasks", `{"title":"Call the bank","brief":"Ask.","manager":"orchestrator"}`, http.StatusOK, `"accept":[]`},
		{http.MethodPost, "/api/tasks", `{"title":"Stop icon","manager":"orchestrator","accept":["the icon is filled","npm run check passes"],"sandbox":{}}`, http.StatusOK, `"accept":["the icon is filled","npm run check passes"]`},
		{http.MethodPost, "/api/tasks/stop-icon/sandboxes", `{}`, http.StatusCreated, `"name":"stop-icon-2"`},
	})

	routes.server.SandboxesAPI.Wait()

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/sandboxes/stop-icon", "", http.StatusOK, `## Accepted when\n\n- the icon is filled\n- npm run check passes\n\n## How this task works`},
		{http.MethodGet, "/api/tasks/stop-icon", "", http.StatusOK, `"status":"running"`},
		{http.MethodPost, "/api/tasks/stop-icon/deliver", `{"summary":" "}`, http.StatusBadRequest, "summary is required"},
		{http.MethodPost, "/api/tasks/nope/deliver", `{"summary":"x"}`, http.StatusNotFound, "task nope not found"},
		{http.MethodPost, "/api/tasks/stop-icon/deliver", `{"summary":"Filled the icon."}`, http.StatusOK, `"delivered_at":"20`},
		{http.MethodGet, "/api/tasks/stop-icon", "", http.StatusOK, `"status":"review"`},
		{http.MethodGet, "/api/tasks/stop-icon", "", http.StatusOK, `"summary":"Filled the icon."`},
		{http.MethodPost, "/api/tasks/stop-icon/reject", `{"by":"orchestrator"}`, http.StatusBadRequest, "why is required"},
		{http.MethodPost, "/api/tasks/stop-icon/reject", `{"by":"orchestrator","why":"still hollow"}`, http.StatusOK, `"delivered_at":null,"summary":"Filled the icon."`},
		{http.MethodGet, "/api/tasks/stop-icon", "", http.StatusOK, `"status":"running"`},
		{http.MethodPost, "/api/tasks/stop-icon/deliver", `{"summary":"Filled it, really."}`, http.StatusOK, `"status":"review"`},
		// No body: by is the owner.
		{http.MethodPost, "/api/tasks/stop-icon/accept", "", http.StatusOK, `"accepted_by":"franco"`},
		{http.MethodGet, "/api/tasks/stop-icon", "", http.StatusOK, `"status":"done"`},
		{http.MethodGet, "/api/sandboxes/stop-icon", "", http.StatusOK, `"status":"stopped"`},
		{http.MethodGet, "/api/sandboxes/stop-icon-2", "", http.StatusOK, `"status":"stopped"`},
		{http.MethodPost, "/api/tasks/stop-icon/reject", `{"why":"late"}`, http.StatusConflict, "is done"},
		{http.MethodPost, "/api/tasks/stop-icon/done", "", http.StatusNotFound, ""},
		{http.MethodGet, "/api/msg/read?channel=%23stop-icon&after=2", "", http.StatusOK, `"body":"Delivered: Filled the icon."`},
		{http.MethodGet, "/api/msg/read?channel=%23stop-icon&after=2", "", http.StatusOK, `"body":"Rejected by orchestrator: still hollow"`},
		{http.MethodGet, "/api/msg/read?channel=%23stop-icon&after=2", "", http.StatusOK, `"body":"Accepted by franco."`},
		// A stopped sandbox of an accepted task can be started again.
		{http.MethodPost, "/api/sandboxes/stop-icon/start", "", http.StatusOK, `"status":"running"`},
	})

	if !slices.Equal(rt.Stops(), []string{"stop-icon", "stop-icon-2"}) {
		t.Fatalf("stops = %q", rt.Stops())
	}
}
