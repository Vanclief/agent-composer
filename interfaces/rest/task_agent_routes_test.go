package rest

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// ADR 0009: defaults, manager and owner, the opening messages and /accept.
func TestTaskAgentRoutes(t *testing.T) {
	routes := newTaskRoutes(t, &fakeRuntime{sb: &fakeSandbox{}}, "")

	routes.check(t, []routeCase{
		{http.MethodPost, "/api/tasks", `{"title":"Fix login","brief":"b"}`, http.StatusBadRequest, "manager is required"},
		{http.MethodPost, "/api/tasks", `{"title":"Fix login","brief":"b","manager":" "}`, http.StatusBadRequest, "manager is required"},
		{http.MethodPost, "/api/tasks", `{"title":"Fix login","brief":"b","manager":"Franco V"}`, http.StatusBadRequest, "manager name"},
		{http.MethodPost, "/api/tasks", `{"title":"Fix login","brief":"b","manager":"franco","assignee":"ana","model":"gpt-5.5"}`, http.StatusBadRequest, "assignee ana is a person"},
		{http.MethodPost, "/api/msg/join", `{"name":"agc"}`, http.StatusConflict, "reserved"},
	})

	status, body := routes.do(http.MethodPost, "/api/tasks", `{"title":"Fix login","brief":"Make 2FA work.","manager":"orchestrator","sandbox":{"repo":"github.com/o/n"}}`)
	if status != http.StatusOK {
		t.Fatalf("status %d, body %s", status, body)
	}

	var view tasks.TaskView
	err := json.Unmarshal([]byte(body), &view)
	if err != nil {
		t.Fatal(err)
	}

	if view.Assignee != "claude_code" || view.Model != "claude-opus-5-5" || view.Manager != "orchestrator" || view.Owner != "franco" {
		t.Fatalf("view = %+v", view)
	}

	created := view.Sandboxes[0]
	if created.Command != "/usr/local/bin/agc agent run --harness claude_code --model claude-opus-5-5 --task fix-login --manager orchestrator --owner franco --brief /home/agent/TASK.md" {
		t.Fatalf("command = %q", created.Command)
	}

	routes.server.SandboxesAPI.Wait()

	status, body = routes.do(http.MethodGet, "/api/msg/read?channel=%23fix-login&after=0", "")
	if status != http.StatusOK {
		t.Fatalf("read: status %d, body %s", status, body)
	}

	var read httpapi.MessagesResponse
	err = json.Unmarshal([]byte(body), &read)
	if err != nil {
		t.Fatal(err)
	}

	if len(read.Messages) != 2 {
		t.Fatalf("messages = %s", body)
	}

	opening := "@orchestrator (manager), @franco (owner): task `fix-login` is open: Fix login.\n" +
		"Agent: claude_code (claude-opus-5-5) on sandbox `fix-login`, starting.\n" +
		"The agent's instructions are the next message. Reply here to talk to it;\n" +
		"its manager raises to franco what only the owner can decide.\n" +
		"Post /accept to accept the task, /reject <why> to send it back."
	if read.Messages[0].Body != opening || read.Messages[1].Body != created.Instructions {
		t.Fatalf("messages = %s", body)
	}

	for _, m := range read.Messages {
		if m.From != "agc" || m.Task != "" || m.Human {
			t.Fatalf("not from agc: %+v", m)
		}
	}

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/sandboxes/fix-login", "", http.StatusOK, `"instructions":"# Fix login\n\nMake 2FA work.\n\n## How this task works\n`},
		{http.MethodGet, "/api/tasks/fix-login", "", http.StatusOK, `"model":"claude-opus-5-5","manager":"orchestrator","owner":"franco"`},
	})

	// The owner posts /accept from the panel.
	status, body = routes.do(http.MethodPost, "/api/msg/join", `{"name":"franco"}`)
	if status != http.StatusOK {
		t.Fatalf("join: status %d, body %s", status, body)
	}

	var franco msgmodels.Participant
	err = json.Unmarshal([]byte(body), &franco)
	if err != nil {
		t.Fatal(err)
	}

	routes.check(t, []routeCase{
		{http.MethodPost, "/api/msg/post", `{"participant_id":"` + franco.ID + `","channel":"#fix-login","body":"/accept"}`, http.StatusOK, `"human":true`},
		{http.MethodGet, "/api/tasks/fix-login", "", http.StatusOK, `"done_at":"20`},
		{http.MethodGet, "/api/tasks/fix-login", "", http.StatusOK, `"accepted_by":"franco"`},
		{http.MethodGet, "/api/tasks/fix-login", "", http.StatusOK, `"status":"done"`},
	})
}
