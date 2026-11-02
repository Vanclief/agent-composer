package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
)

// fakeServer answers the task routes the CLI calls and records them.
type fakeServer struct {
	mu       sync.Mutex
	requests []string
	created  tasks.CreateRequest
}

func (f *fakeServer) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		f.mu.Unlock()

		view := tasks.TaskView{
			Name:      "fix-login",
			Title:     "Fix login",
			Assignee:  "claude_code",
			Status:    tasks.StatusOpen,
			Sandboxes: []sandboxes.SandboxView{{Name: "fix-login", Task: "fix-login", Status: sandboxes.StatusStarting}},
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /api/tasks":
			err := json.NewDecoder(r.Body).Decode(&f.created)
			if err != nil {
				t.Error(err)
			}
			_ = json.NewEncoder(w).Encode(view)
		case "GET /api/tasks/fix-login":
			_ = json.NewEncoder(w).Encode(view)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"task nope not found"}}`))
		}
	})
}

func runAgc(t *testing.T, args ...string) (string, error) {
	t.Helper()

	return runAgcErr(t, &bytes.Buffer{}, args...)
}

func runAgcErr(t *testing.T, errOut *bytes.Buffer, args ...string) (string, error) {
	t.Helper()

	app := newApp()
	out := &bytes.Buffer{}
	app.Writer = out
	app.ErrWriter = errOut

	err := app.RunContext(context.Background(), append([]string{"agc"}, args...))

	return out.String(), err
}

func TestTaskCreateAgainstAServerReturnsAtOnce(t *testing.T) {
	fake := &fakeServer{}
	server := httptest.NewServer(fake.handler(t))
	defer server.Close()

	file := filepath.Join(t.TempDir(), "task.yaml")
	err := os.WriteFile(file, []byte("title: Fix login\nbrief: Make 2FA work.\naccept:\n  - 2FA works\n  - tests pass\nassignee: claude_code\nmanager: orchestrator\nsandbox:\n  repo: github.com/o/n\n  command: claude\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	out, err := runAgc(t, "task", "create", "--server", server.URL, "-f", file)
	if err != nil {
		t.Fatal(err)
	}

	if fake.created.Title != "Fix login" || fake.created.Sandbox == nil || fake.created.Sandbox.Repo != "github.com/o/n" || fake.created.Sandbox.Command != "claude" {
		t.Fatalf("created %+v", fake.created)
	}
	// The file's accept list must reach the server (ADR 0010): it was dropped once.
	if len(fake.created.Accept) != 2 || fake.created.Accept[0] != "2FA works" || fake.created.Accept[1] != "tests pass" {
		t.Fatalf("created accept %v, want the two lines from the file", fake.created.Accept)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("requests %v: create must not poll without --wait", fake.requests)
	}
	for _, want := range []string{"fix-login  open  claude_code  Fix login", "fix-login  starting", server.URL + "/panel/tasks/fix-login"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output %q lacks %q", out, want)
		}
	}
}

func TestTaskGetFromEnvWithTrailingJSON(t *testing.T) {
	fake := &fakeServer{}
	server := httptest.NewServer(fake.handler(t))
	defer server.Close()
	t.Setenv(serverEnv, server.URL)

	out, err := runAgc(t, "task", "get", "fix-login", "--json")
	if err != nil {
		t.Fatal(err)
	}

	view := tasks.TaskView{}
	err = json.Unmarshal([]byte(out), &view)
	if err != nil {
		t.Fatalf("output %q is not JSON: %v", out, err)
	}
	if view.Name != "fix-login" || view.Sandboxes[0].Status != "starting" {
		t.Fatalf("view %+v", view)
	}
}

func TestServerErrorIsTheMessage(t *testing.T) {
	fake := &fakeServer{}
	server := httptest.NewServer(fake.handler(t))
	defer server.Close()

	errOut := &bytes.Buffer{}
	_, err := runAgcErr(t, errOut, "task", "--server", server.URL, "accept", "nope")
	printed := &PrintedError{}
	if !errors.As(err, &printed) || ez.ErrorMessage(printed.Err) != "task nope not found" {
		t.Fatalf("err %v", err)
	}
	if errOut.String() != "task nope not found\n" {
		t.Fatalf("stderr %q", errOut.String())
	}
	if len(fake.requests) != 1 || fake.requests[0] != "POST /api/tasks/nope/accept" {
		t.Fatalf("requests %v", fake.requests)
	}
}

func TestGlobalServerFlag(t *testing.T) {
	fake := &fakeServer{}
	server := httptest.NewServer(fake.handler(t))
	defer server.Close()

	out, err := runAgc(t, "--server", server.URL, "task", "get", "fix-login")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "fix-login  open  claude_code  Fix login\n") {
		t.Fatalf("output %q", out)
	}
}
