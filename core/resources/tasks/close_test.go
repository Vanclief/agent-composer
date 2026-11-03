package tasks

import (
	"context"
	"testing"

	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// asker is a joined participant that asks questions.
type asker struct {
	caller messaging.Caller
	id     string
}

func join(t *testing.T, env *testEnv, caller messaging.Caller, name string) asker {
	t.Helper()

	p, err := env.msg.Join(context.Background(), caller, name)
	if err != nil {
		t.Fatal(err)
	}

	return asker{caller: caller, id: p.ID}
}

// ask asks body in channel as a.
func ask(t *testing.T, env *testEnv, a asker, channel, body string) int64 {
	t.Helper()

	m, err := env.msg.Post(context.Background(), a.caller, a.id, channel, body, nil, msgmodels.KindQuestion)
	if err != nil {
		t.Fatal(err)
	}

	return m.ID
}

// openIDs is the ids of the open questions for name.
func openIDs(t *testing.T, env *testEnv, name string) map[int64]bool {
	t.Helper()

	got, err := env.msg.Questions(context.Background(), messaging.Caller{Human: true}, messaging.QuestionsFilter{For: name, Open: true})
	if err != nil {
		t.Fatal(err)
	}

	ids := map[int64]bool{}
	for _, q := range got {
		ids[q.ID] = true
	}

	return ids
}

func TestAcceptClosesTheTasksAgentQuestions(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	for _, name := range []string{"managed", "other"} {
		_, err := env.api.Create(ctx, nil, &CreateRequest{Name: name, Brief: "b", Manager: "lead", Owner: "franco"})
		if err != nil {
			t.Fatal(err)
		}
	}

	worker := join(t, env, messaging.Caller{Task: "managed"}, "worker")
	lead := join(t, env, messaging.Caller{Agent: "lead-home", Grants: []messaging.Grant{{Channel: "#franco"}, {Channel: "#managed"}, {Channel: "#other"}}}, "lead")
	stranger := join(t, env, messaging.Caller{Agent: "stranger-home", Grants: []messaging.Grant{{Channel: "#franco"}}}, "stranger")
	human := join(t, env, messaging.Caller{Human: true}, "franco")

	closes := []int64{
		ask(t, env, worker, "#managed", "QUESTION: which command runs the server?"),
		ask(t, env, lead, "#managed", "Is the branch pushed?"),
		ask(t, env, lead, "#franco", "Accept managed?\nIt adds a thing."),
		ask(t, env, lead, "#franco", "managed: adds the thing, accept?"),
	}
	keeps := []int64{
		ask(t, env, human, "#managed", "Why this way?"),
		ask(t, env, lead, "#franco", "Accept other?"),
		ask(t, env, lead, "#franco", "managed-two: accept?"),
		ask(t, env, stranger, "#franco", "Accept managed?"),
		ask(t, env, lead, "#other", "Done yet?"),
	}

	before := openIDs(t, env, "franco")
	for _, id := range append(append([]int64{}, closes...), keeps...) {
		if !before[id] {
			t.Fatalf("question %d not open before the accept", id)
		}
	}

	_, err := env.api.Accept(ctx, nil, &AcceptRequest{Name: "managed", By: "franco"})
	if err != nil {
		t.Fatal(err)
	}

	after := openIDs(t, env, "franco")
	for _, id := range closes {
		if after[id] {
			t.Fatalf("question %d still open after the accept", id)
		}
	}
	for _, id := range keeps {
		if !after[id] {
			t.Fatalf("question %d closed by the accept", id)
		}
	}

	for _, id := range closes {
		replies := []msgmodels.Message{}

		err = env.db.NewSelect().Model(&replies).Where("reply_to = ?", id).Scan(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(replies) != 1 || replies[0].Body != "Closed: managed accepted by franco." || !msgmodels.FromSystem(&replies[0]) {
			t.Fatalf("replies to %d = %+v", id, replies)
		}
	}

	// A second accept is a no-op and posts nothing more.
	_, err = env.api.Accept(ctx, nil, &AcceptRequest{Name: "managed", By: "franco"})
	if err != nil {
		t.Fatal(err)
	}

	count, err := env.db.NewSelect().Model((*msgmodels.Message)(nil)).Where("body LIKE ?", "Closed: %").Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != len(closes) {
		t.Fatalf("closing replies = %d, want %d", count, len(closes))
	}
}

func TestRejectClosesNothing(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	_, err := env.api.Create(ctx, nil, &CreateRequest{Name: "managed", Brief: "b", Manager: "lead", Owner: "franco"})
	if err != nil {
		t.Fatal(err)
	}

	worker := join(t, env, messaging.Caller{Task: "managed"}, "worker")
	id := ask(t, env, worker, "#managed", "Which command?")

	_, err = env.api.Deliver(ctx, nil, &DeliverRequest{Name: "managed", Summary: "done"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.api.Reject(ctx, nil, &RejectRequest{Name: "managed", By: "franco", Why: "not yet"})
	if err != nil {
		t.Fatal(err)
	}

	if !openIDs(t, env, "franco")[id] {
		t.Fatalf("question %d closed by a reject", id)
	}
}
