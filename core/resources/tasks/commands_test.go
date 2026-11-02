package tasks

import (
	"context"
	"testing"

	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/task"
)

func TestParseCommand(t *testing.T) {
	cases := []struct {
		body string
		ok   bool
		want Command
	}{
		{" /accept\n", true, Command{Verb: AcceptCommand}},
		{"/accept fix-login", true, Command{Verb: AcceptCommand, Arg: "fix-login"}},
		{"/reject the icon is still hollow", true, Command{Verb: RejectCommand, Arg: "the icon is still hollow"}},
		{"/reject\nno tests", true, Command{Verb: RejectCommand, Arg: "no tests"}},
		{"/reject", false, Command{}},
		{"/accepted", false, Command{}},
		{"/done", false, Command{}},
		{"please /accept", false, Command{}},
	}

	for _, tc := range cases {
		got, ok := ParseCommand(tc.body)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("ParseCommand(%q) = %+v, %v", tc.body, got, ok)
		}
	}
}

func TestChannelCommands(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	human := messaging.Caller{Human: true}

	names := []string{"by-manager", "by-owner", "by-other", "by-inbox", "rejected", "not-inbox"}
	for _, name := range names {
		_, err := env.api.Create(ctx, nil, &CreateRequest{Name: name, Brief: "b", Manager: "orchestrator", Owner: "franco", Sandbox: &task.Sandbox{}})
		if err != nil {
			t.Fatal(err)
		}

		_, err = env.api.Deliver(ctx, nil, &DeliverRequest{Name: name, Summary: "s"})
		if err != nil {
			t.Fatal(err)
		}
	}
	env.api.Sandboxes.Wait()

	post := func(caller messaging.Caller, who string, channel string, body string) {
		t.Helper()

		p, err := env.msg.Join(ctx, caller, who)
		if err != nil {
			t.Fatal(err)
		}

		_, err = env.msg.Post(ctx, caller, p.ID, channel, body, nil, "")
		if err != nil {
			t.Fatal(err)
		}

		err = env.msg.Leave(ctx, caller, p.ID)
		if err != nil {
			t.Fatal(err)
		}
	}

	post(human, "orchestrator", "#by-manager", " /accept\n")
	post(human, "franco", "#by-owner", "/accept")
	post(human, "someone", "#by-other", "/accept")
	post(human, "franco", "#by-other", "/accept please")
	post(human, "franco", "#by-other", "/done")
	// The task's own agent cannot accept it, even under the manager's name.
	post(messaging.Caller{Task: "by-other"}, "orchestrator", "#by-other", "/accept")
	post(human, "franco", "#franco", "/accept by-inbox")
	// Only the owner, only in the owner's own inbox.
	post(human, "orchestrator", "#orchestrator", "/accept not-inbox")
	post(human, "orchestrator", "#franco", "/accept not-inbox")
	post(human, "orchestrator", "#rejected", "/reject the icon is still hollow")
	post(human, "someone", "#not-inbox", "/reject no")

	want := map[string]string{
		"by-manager": StatusDone,
		"by-owner":   StatusDone,
		"by-other":   StatusReview,
		"by-inbox":   StatusDone,
		"rejected":   StatusRunning,
		"not-inbox":  StatusReview,
	}
	for name, status := range want {
		view, err := env.api.Get(ctx, nil, &GetRequest{Name: name})
		if err != nil {
			t.Fatal(err)
		}

		if view.Status != status {
			t.Fatalf("%s: status %s, want %s", name, view.Status, status)
		}

		accepted := status == StatusDone
		if accepted != (view.Sandboxes[0].Status == "stopped") {
			t.Fatalf("%s: sandbox %s", name, view.Sandboxes[0].Status)
		}
	}

	view, err := env.api.Get(ctx, nil, &GetRequest{Name: "by-manager"})
	if err != nil || view.AcceptedBy != "orchestrator" {
		t.Fatalf("by-manager: %+v, err %v", view, err)
	}

	view, err = env.api.Get(ctx, nil, &GetRequest{Name: "by-inbox"})
	if err != nil || view.AcceptedBy != "franco" {
		t.Fatalf("by-inbox: %+v, err %v", view, err)
	}

	messages, err := env.msg.Read(ctx, human, "#rejected", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	last := messages[len(messages)-1]
	if last.From != "agc" || last.Body != "Rejected by orchestrator: the icon is still hollow" {
		t.Fatalf("last message = %+v", last)
	}

	messages, err = env.msg.Read(ctx, human, "#by-inbox", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	last = messages[len(messages)-1]
	if last.From != "agc" || last.Body != "Accepted by franco." {
		t.Fatalf("last message = %+v", last)
	}
}
