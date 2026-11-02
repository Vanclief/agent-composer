package rest

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/executor/sandbox"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// gitRuntime reports every sandbox running and answers git in each by its
// name: outputs[name][command] is the stdout. A sandbox missing from
// outputs cannot be connected to.
type gitRuntime struct {
	outputs map[string]map[string]string
}

func (g *gitRuntime) Create(ctx context.Context, spec sandbox.Spec) (sandboxes.Sandbox, error) {
	return nil, ez.New(ez.EINTERNAL, "not used", nil)
}

func (g *gitRuntime) Connect(ctx context.Context, name string) (sandboxes.Sandbox, error) {
	outputs, ok := g.outputs[name]
	if !ok {
		return nil, ez.New(ez.EUNAVAILABLE, "cannot reach sandbox "+name, nil)
	}

	return &gitSandbox{outputs: outputs}, nil
}

func (g *gitRuntime) Start(ctx context.Context, name string, secrets []sandbox.Secret) error {
	return nil
}

func (g *gitRuntime) Stop(ctx context.Context, name string) error {
	return nil
}

func (g *gitRuntime) Remove(ctx context.Context, name string) error {
	return nil
}

func (g *gitRuntime) Status(ctx context.Context, name string) (string, error) {
	return "running", nil
}

// gitSandbox implements Run and Detach.
type gitSandbox struct {
	executor.TerminalExecutor
	outputs map[string]string
}

func (s *gitSandbox) Run(ctx context.Context, cmd executor.Command) (*executor.Result, error) {
	stdout, ok := s.outputs[cmd.Name+" "+strings.Join(cmd.Args, " ")]
	if !ok {
		return &executor.Result{ExitCode: 128, Stderr: []byte("fatal: not a git repository")}, nil
	}

	return &executor.Result{Stdout: []byte(stdout)}, nil
}

func (s *gitSandbox) Detach(ctx context.Context) error {
	return nil
}

func TestSandboxWorkRoutes(t *testing.T) {
	const (
		gitStatus = "git status --porcelain"
		unpushed  = "git rev-list --count HEAD --not --remotes"
	)

	rt := &gitRuntime{outputs: map[string]map[string]string{
		"review-42":   {gitStatus: " M main.go\n?? notes.md\n", unpushed: "2\n"},
		"review-42-2": {gitStatus: "", unpushed: "0\n"},
		"not-a-clone": {},
	}}
	routes := newTaskRoutes(t, rt, "")

	owner := &taskmodels.Task{Title: "Review 42", Name: "review-42", Brief: "b"}
	err := owner.Insert(context.Background(), routes.db)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	specs := map[string]string{
		"review-42":   `{"repo":"github.com/o/n"}`,
		"review-42-2": `{"repo":"github.com/o/n"}`,
		"not-a-clone": `{"repo":"github.com/o/n"}`,
		"unreachable": `{"repo":"github.com/o/n"}`,
		"no-repo":     `{}`,
	}
	for name, spec := range specs {
		record := &taskmodels.Sandbox{TaskID: owner.ID, Name: name, Spec: []byte(spec), BootedAt: &now}
		err = record.Insert(context.Background(), routes.db)
		if err != nil {
			t.Fatal(err)
		}
	}

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/sandboxes/review-42", "", http.StatusOK, `"work":{"dirty":true,"unpushed":2,"checked_at":"20`},
		{http.MethodGet, "/api/sandboxes/review-42-2", "", http.StatusOK, `"work":{"dirty":false,"unpushed":0,"checked_at":"20`},
		{http.MethodGet, "/api/sandboxes/not-a-clone", "", http.StatusOK, `"work":null`},
		{http.MethodGet, "/api/sandboxes/unreachable", "", http.StatusOK, `"status":"running","booted_at"`},
		{http.MethodGet, "/api/sandboxes/unreachable", "", http.StatusOK, `"work":null`},
		{http.MethodGet, "/api/sandboxes/no-repo", "", http.StatusOK, `"work":null`},
		{http.MethodGet, "/api/sandboxes?work=1", "", http.StatusOK, `"work":{"dirty":true,"unpushed":2,`},
	})

	status, body := routes.do(http.MethodGet, "/api/sandboxes", "")
	if status != http.StatusOK || strings.Contains(body, `"dirty"`) {
		t.Fatalf("list without work=1 checked work: %d %s", status, body)
	}
}
