package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/task"
)

func writeJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(value)
	if err != nil {
		t.Error(err)
	}
}

func TestCreateTaskSendsTheRequestAndReturnsTheView(t *testing.T) {
	var got tasks.CreateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/tasks" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type %q", r.Header.Get("Content-Type"))
		}

		err := json.NewDecoder(r.Body).Decode(&got)
		if err != nil {
			t.Error(err)
		}

		writeJSON(t, w, http.StatusOK, tasks.TaskView{
			Name:      "fix-login",
			Title:     got.Title,
			Status:    tasks.StatusOpen,
			Sandboxes: []sandboxes.SandboxView{{Name: "fix-login", Task: "fix-login", Status: sandboxes.StatusStarting}},
		})
	}))
	defer server.Close()

	c := New(server.URL + "/")
	sandbox := &task.Sandbox{Spec: task.Spec{Repo: "github.com/o/n", CPUs: 2}, Command: "claude"}
	view, err := c.CreateTask(context.Background(), &tasks.CreateRequest{Title: "Fix login", Brief: "Make 2FA work.", Sandbox: sandbox})
	if err != nil {
		t.Fatal(err)
	}

	if got.Title != "Fix login" || got.Brief != "Make 2FA work." || got.Sandbox == nil || got.Sandbox.Repo != "github.com/o/n" || got.Sandbox.CPUs != 2 || got.Sandbox.Command != "claude" {
		t.Fatalf("request = %+v, sandbox %+v", got, got.Sandbox)
	}
	if view.Name != "fix-login" || view.Status != "open" || len(view.Sandboxes) != 1 || view.Sandboxes[0].Status != "starting" {
		t.Fatalf("view = %+v", view)
	}
	if c.TaskURL(view.Name) != server.URL+"/panel/tasks/fix-login" {
		t.Fatalf("task url %s", c.TaskURL(view.Name))
	}
}

func TestListTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/tasks" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}

		writeJSON(t, w, http.StatusOK, tasks.ListResponse{Tasks: []tasks.TaskView{
			{Name: "a", Status: tasks.StatusRunning, Sandboxes: []sandboxes.SandboxView{{Name: "a", Status: sandboxes.StatusRunning}}},
			{Name: "b", Status: tasks.StatusDone, Sandboxes: []sandboxes.SandboxView{}},
		}})
	}))
	defer server.Close()

	list, err := New(server.URL).ListTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(list.Tasks) != 2 || list.Tasks[0].Name != "a" || list.Tasks[0].Sandboxes[0].Status != "running" || list.Tasks[1].Status != "done" {
		t.Fatalf("list = %+v", list)
	}
}

func TestErrorBodyKeepsTheServerMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tasks/unknown-route/stop" {
			writeJSON(t, w, http.StatusNotFound, map[string]string{"message": "Not Found"})
			return
		}

		writeJSON(t, w, http.StatusNotFound, map[string]any{"error": map[string]string{
			"code":       "not_found",
			"message":    "task review-42 not found",
			"request_id": "r1",
		}})
	}))
	defer server.Close()

	c := New(server.URL)
	_, err := c.GetTask(context.Background(), "review-42")
	if err == nil {
		t.Fatal("expected an error")
	}
	if ez.ErrorMessage(err) != "task review-42 not found" || ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("error %q code %q", ez.ErrorMessage(err), ez.ErrorCode(err))
	}

	err = c.do(context.Background(), http.MethodPost, "/api/tasks/unknown-route/stop", nil, nil)
	if ez.ErrorMessage(err) != "Not Found" {
		t.Fatalf("echo error %q", ez.ErrorMessage(err))
	}
}

func TestUnreachableServerSaysWhere(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	_, err := New(url).ListTasks(context.Background())
	if ez.ErrorCode(err) != ez.EUNAVAILABLE {
		t.Fatalf("err = %v", err)
	}
}

func TestWaitSandboxReachesRunning(t *testing.T) {
	statuses := []string{sandboxes.StatusStarting, sandboxes.StatusStarting, sandboxes.StatusStarting, sandboxes.StatusRunning}
	mu := sync.Mutex{}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/sandboxes/fix-login" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}

		mu.Lock()
		status := statuses[calls]
		calls++
		mu.Unlock()

		writeJSON(t, w, http.StatusOK, sandboxes.SandboxView{Name: "fix-login", Task: "fix-login", Status: status})
	}))
	defer server.Close()

	c := New(server.URL)
	c.PollInterval = time.Millisecond

	seen := []string{}
	view, err := c.WaitSandbox(context.Background(), "fix-login", func(view *sandboxes.SandboxView) {
		seen = append(seen, view.Status)
	})
	if err != nil {
		t.Fatal(err)
	}

	if view.Status != "running" || calls != 4 {
		t.Fatalf("view %+v after %d calls", view, calls)
	}
	if len(seen) != 2 || seen[0] != "starting" || seen[1] != "running" {
		t.Fatalf("reported %v", seen)
	}
}

func TestWaitSandboxStopsAtFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, sandboxes.SandboxView{Name: "x", Status: sandboxes.StatusFailed, StatusError: "clone failed"})
	}))
	defer server.Close()

	view, err := New(server.URL).WaitSandbox(context.Background(), "x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "failed" || view.StatusError != "clone failed" {
		t.Fatalf("view %+v", view)
	}
}

func TestAllocateSendsTheSandboxInline(t *testing.T) {
	body := map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/tasks/fix-login/sandboxes" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}

		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			t.Error(err)
		}

		writeJSON(t, w, http.StatusCreated, sandboxes.SandboxView{Name: "fix-login-2", Status: sandboxes.StatusStarting})
	}))
	defer server.Close()

	view, err := New(server.URL).Allocate(context.Background(), "fix-login", task.Sandbox{Spec: task.Spec{Image: "debian"}, Command: "bash"})
	if err != nil {
		t.Fatal(err)
	}

	if body["image"] != "debian" || body["command"] != "bash" {
		t.Fatalf("body %v", body)
	}
	if view.Name != "fix-login-2" || view.Status != "starting" {
		t.Fatalf("view %+v", view)
	}
}
