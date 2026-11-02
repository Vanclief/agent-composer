package tasks

import (
	"context"
	"testing"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

func TestOpeningMessage(t *testing.T) {
	harness := &taskmodels.Task{Title: "Fix login", Name: "fix-login", Assignee: "claude_code", Model: "claude-opus-5-5", Manager: "orchestrator", Owner: "franco"}
	sandbox := &sandboxes.SandboxView{Name: "fix-login"}

	cases := []struct {
		name      string
		record    taskmodels.Task
		allocated *sandboxes.SandboxView
		want      string
	}{
		{
			name:      "harness on a sandbox",
			record:    *harness,
			allocated: sandbox,
			want: "@orchestrator (manager), @franco (owner): task `fix-login` is open: Fix login.\n" +
				"Agent: claude_code (claude-opus-5-5) on sandbox `fix-login`, starting.\n" +
				"The agent's instructions are the next message. Reply here to talk to it;\n" +
				"it escalates to #franco when only the owner can decide. Post /done to\n" +
				"close the task.",
		},
		{
			name:   "person without a sandbox",
			record: taskmodels.Task{Title: "Call the bank", Name: "call-the-bank", Assignee: "ana", Manager: "franco", Owner: "franco"},
			want: "@franco (manager), @franco (owner): task `call-the-bank` is open: Call the bank.\n" +
				"Assigned to ana, no sandbox.\n" +
				"Post /done to close the task.",
		},
		{
			name:      "no owner",
			record:    taskmodels.Task{Title: "Fix login", Name: "fix-login", Assignee: "codex_cli", Model: "gpt-5.5", Manager: "orchestrator"},
			allocated: sandbox,
			want: "@orchestrator (manager): task `fix-login` is open: Fix login.\n" +
				"Agent: codex_cli (gpt-5.5) on sandbox `fix-login`, starting.\n" +
				"The agent's instructions are the next message. Reply here to talk to it.\n" +
				"Post /done to close the task.",
		},
	}

	for _, tc := range cases {
		got := OpeningMessage(&tc.record, tc.allocated)
		if got != tc.want {
			t.Fatalf("%s:\n--- got\n%s\n--- want\n%s", tc.name, got, tc.want)
		}
	}
}

func TestCreateAppliesDefaultsAndOpensTheChannel(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	view, err := env.api.Create(ctx, nil, &CreateRequest{Title: "Fix login", Brief: "Make 2FA work.", Manager: "orchestrator", Sandbox: &task.Sandbox{Spec: task.Spec{Repo: "github.com/o/n"}}})
	if err != nil {
		t.Fatal(err)
	}

	if view.Assignee != "claude_code" || view.Model != "claude-opus-5-5" || view.Manager != "orchestrator" || view.Owner != "franco" {
		t.Fatalf("defaults not applied: %+v", view)
	}

	created := view.Sandboxes[0]
	if created.Command != "/usr/local/bin/agc agent run --harness claude_code --model claude-opus-5-5 --task fix-login --manager orchestrator --owner franco --brief /home/agent/TASK.md" {
		t.Fatalf("command = %q", created.Command)
	}

	messages, err := env.msg.Read(ctx, messaging.Caller{Human: true}, "#fix-login", 0, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 2 {
		t.Fatalf("messages = %+v", messages)
	}

	opening := "@orchestrator (manager), @franco (owner): task `fix-login` is open: Fix login.\n" +
		"Agent: claude_code (claude-opus-5-5) on sandbox `fix-login`, starting.\n" +
		"The agent's instructions are the next message. Reply here to talk to it;\n" +
		"it escalates to #franco when only the owner can decide. Post /done to\n" +
		"close the task."
	t.Logf("opening message:\n%s", messages[0].Body)
	t.Logf("instructions:\n%s", messages[1].Body)

	if messages[0].Body != opening || messages[1].Body != created.Instructions || created.Instructions == "" {
		t.Fatalf("messages = %+v", messages)
	}

	for _, m := range messages {
		if m.From != "agc" || m.Task != "" || m.Human {
			t.Fatalf("not from agc: %+v", m)
		}
	}

	// A person's task: one message, no instructions.
	_, err = env.api.Create(ctx, nil, &CreateRequest{Title: "Call the bank", Brief: "Ask.", Assignee: "ana", Manager: "orchestrator", Owner: "ana"})
	if err != nil {
		t.Fatal(err)
	}

	messages, err = env.msg.Read(ctx, messaging.Caller{Human: true}, "#call-the-bank", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].Body != "@orchestrator (manager), @ana (owner): task `call-the-bank` is open: Call the bank.\nAssigned to ana, no sandbox.\nPost /done to close the task." {
		t.Fatalf("messages = %+v", messages)
	}
}

func TestDoneFromManagerOrOwner(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	human := messaging.Caller{Human: true}

	for _, name := range []string{"by-manager", "by-owner", "by-other"} {
		_, err := env.api.Create(ctx, nil, &CreateRequest{Name: name, Brief: "b", Assignee: "ana", Manager: "orchestrator", Owner: "franco"})
		if err != nil {
			t.Fatal(err)
		}
	}

	post := func(caller messaging.Caller, who string, channel string, body string) {
		t.Helper()

		p, err := env.msg.Join(ctx, caller, who)
		if err != nil {
			t.Fatal(err)
		}

		_, err = env.msg.Post(ctx, caller, p.ID, channel, body, nil)
		if err != nil {
			t.Fatal(err)
		}

		err = env.msg.Leave(ctx, caller, p.ID)
		if err != nil {
			t.Fatal(err)
		}
	}

	post(human, "orchestrator", "#by-manager", " /done\n")
	post(human, "franco", "#by-owner", "/done")
	post(human, "someone", "#by-other", "/done")
	post(human, "franco", "#by-other", "/done please")
	// The task's own agent cannot close it, even under the manager's name.
	post(messaging.Caller{Task: "by-other"}, "orchestrator", "#by-other", "/done")

	for name, done := range map[string]bool{"by-manager": true, "by-owner": true, "by-other": false} {
		view, err := env.api.Get(ctx, nil, &GetRequest{Name: name})
		if err != nil {
			t.Fatal(err)
		}

		if (view.DoneAt != nil) != done || (view.Status == StatusDone) != done {
			t.Fatalf("%s: done_at %v, status %s", name, view.DoneAt, view.Status)
		}
	}
}
