package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/vanclief/agent-composer/core/resources/agents"
	"github.com/vanclief/agent-composer/core/resources/tasks"
)

// agentServer answers the agent routes and records requests and bodies.
type agentServer struct {
	mu       sync.Mutex
	requests []string
}

func (f *agentServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		f.mu.Lock()
		f.requests = append(f.requests, strings.TrimSpace(r.Method+" "+r.URL.Path+" "+string(body)))
		f.mu.Unlock()

		home := "reviewer-1"
		view := agents.AgentView{Name: "reviewer-1", Role: "reviewer", Harness: "claude_code", State: agents.StateAsleep, Tasks: []agents.AgentTask{{Name: "review-42", Status: "open"}}}

		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/agents":
			_ = json.NewEncoder(w).Encode(agents.ListResponse{Agents: []agents.AgentView{view}})
		case "POST /api/agents":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(view)
		case "POST /api/agents/reviewer-1/keep":
			view.Name = "ada"
			view.Kept = true
			_ = json.NewEncoder(w).Encode(view)
		case "POST /api/agents/reviewer-1/wake":
			view.State = agents.StateWaking
			view.Home = &home
			_ = json.NewEncoder(w).Encode(view)
		case "DELETE /api/agents/reviewer-1":
			_ = json.NewEncoder(w).Encode(agents.DeleteResponse{Deleted: true, Name: "reviewer-1"})
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"agent nope not found"}}`))
		}
	})
}

func TestAgentCommandsAgainstAServer(t *testing.T) {
	fake := &agentServer{}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	out, err := runAgc(t, "agent", "list", "--server", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "reviewer-1 (numbered)") || !strings.Contains(out, "asleep") {
		t.Fatalf("list = %q", out)
	}

	out, err = runAgc(t, "agent", "create", "Reviewer", "--harness", "codex_cli", "--server", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Task:") || !strings.Contains(out, "review-42 (open)") {
		t.Fatalf("create = %q", out)
	}

	out, err = runAgc(t, "agent", "keep", "reviewer-1", "--as", "ada", "--role", "main", "--server", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Agent:") || !strings.Contains(out, "ada") || strings.Contains(out, "numbered") {
		t.Fatalf("keep = %q", out)
	}

	out, err = runAgc(t, "agent", "wake", "reviewer-1", "--json", "--server", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"state": "waking"`) || !strings.Contains(out, `"home": "reviewer-1"`) {
		t.Fatalf("wake = %q", out)
	}

	out, err = runAgc(t, "agent", "delete", "reviewer-1", "--yes", "--server", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Deleted agent reviewer-1") {
		t.Fatalf("delete = %q", out)
	}

	_, err = runAgc(t, "agent", "show", "nope", "--server", server.URL)
	if err == nil {
		t.Fatal("show nope succeeded")
	}

	_, err = runAgc(t, "agent", "keep", "reviewer-1", "--server", server.URL)
	if err == nil || !strings.Contains(err.Error(), "--as") {
		t.Fatalf("keep without --as: %v", err)
	}

	want := []string{
		"GET /api/agents",
		`POST /api/agents {"role":"Reviewer","harness":"codex_cli","model":"","effort":""}`,
		`POST /api/agents/reviewer-1/keep {"name":"ada","role":"main"}`,
		"POST /api/agents/reviewer-1/wake",
		"DELETE /api/agents/reviewer-1",
		"GET /api/agents/nope",
	}
	if strings.Join(fake.requests, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests =\n%s\nwant\n%s", strings.Join(fake.requests, "\n"), strings.Join(want, "\n"))
	}
}

// Inside an agent's home, agc task create and accept go over the home's
// socket, with the agent as manager.
func TestTaskCreateAndAcceptInsideAHome(t *testing.T) {
	var mu sync.Mutex
	requests := []string{}
	created := tasks.CreateRequest{}

	socket := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /tasks":
			_ = json.NewDecoder(r.Body).Decode(&created)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(tasks.TaskView{Name: "review-42", Manager: "reviewer-1", Status: "open"})
		case "GET /tasks/review-42":
			_ = json.NewEncoder(w).Encode(tasks.TaskView{Name: "review-42", Manager: "reviewer-1", Status: "review"})
		case "POST /tasks/review-42/reject":
			_ = json.NewEncoder(w).Encode(tasks.TaskView{Name: "review-42", Manager: "reviewer-1", Status: "open"})
		case "POST /tasks/review-42/accept":
			_ = json.NewEncoder(w).Encode(tasks.TaskView{Name: "review-42", Manager: "reviewer-1", Status: "done", AcceptedBy: "reviewer-1"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer socket.Close()

	t.Setenv("AGC_MSG_URL", socket.URL)
	t.Setenv("AGC_AGENT", "reviewer-1")
	t.Setenv("AGC_TASK", "")
	t.Setenv("AGC_SERVER", "")

	file := filepath.Join(t.TempDir(), "task.yaml")
	err := os.WriteFile(file, []byte("title: Review 42\nbrief: Review it.\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	out, err := runAgc(t, "task", "create", "-f", file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "review-42") || created.Manager != "reviewer-1" || created.Title != "Review 42" {
		t.Fatalf("create = %q, request = %+v", out, created)
	}

	out, err = runAgc(t, "task", "get", "review-42")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "review") {
		t.Fatalf("get = %q", out)
	}

	_, err = runAgc(t, "task", "reject", "review-42", "--why", "Check the tests too.")
	if err != nil {
		t.Fatal(err)
	}

	out, err = runAgc(t, "task", "accept", "review-42")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "done") {
		t.Fatalf("accept = %q", out)
	}

	_, err = runAgc(t, "task", "list")
	if err == nil {
		t.Fatal("task list inside a home succeeded")
	}

	want := "POST /tasks\nGET /tasks/review-42\nPOST /tasks/review-42/reject\nPOST /tasks/review-42/accept"
	if strings.Join(requests, "\n") != want {
		t.Fatalf("requests = %q", requests)
	}
}
