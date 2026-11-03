package sandboxes

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// askAs joins caller as name and asks each body in channel.
func askAs(t *testing.T, env *testEnv, caller messaging.Caller, name, channel string, bodies ...string) []int64 {
	t.Helper()
	ctx := context.Background()

	p, err := env.api.msg.Join(ctx, caller, name)
	if err != nil {
		t.Fatal(err)
	}

	ids := []int64{}
	for _, body := range bodies {
		m, err := env.api.msg.Post(ctx, caller, p.ID, channel, body, nil, msgmodels.KindQuestion)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, m.ID)
	}

	return ids
}

// openQuestions is the ids of the open questions in every channel.
func openQuestions(t *testing.T, env *testEnv) map[int64]bool {
	t.Helper()

	got, err := env.api.msg.Questions(context.Background(), messaging.Caller{Human: true}, messaging.QuestionsFilter{Open: true})
	if err != nil {
		t.Fatal(err)
	}

	ids := map[int64]bool{}
	for _, q := range got {
		ids[q.ID] = true
	}

	return ids
}

func closingReplies(t *testing.T, env *testEnv) []msgmodels.Message {
	t.Helper()

	replies := []msgmodels.Message{}

	err := env.db.NewSelect().Model(&replies).Where("body LIKE ?", messaging.ClosedPrefix+"%").Order("id ASC").Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	return replies
}

// The tick closes the open agent questions of done tasks, in #<task> and
// the manager's accept questions in the owner's inbox, and leaves a
// person's questions and those of tasks not done yet open; once the clock
// passes a task's accept, the next tick closes its questions too.
func TestTickClosesTheQuestionsOfDoneTasks(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	env.api.now = func() time.Time {
		return now
	}
	env.api.keepDone = 0
	env.api.archiveDone = 0

	doneAt := now.Add(-3 * 24 * time.Hour)
	old := &taskmodels.Task{Title: "Old", Name: "old-done", Brief: "b", Manager: "lead", Owner: "franco", DoneAt: &doneAt, AcceptedBy: "franco"}
	open := &taskmodels.Task{Title: "Open", Name: "open-task", Brief: "b", Manager: "lead", Owner: "franco"}
	for _, record := range []*taskmodels.Task{old, open} {
		err := record.Insert(ctx, env.db)
		if err != nil {
			t.Fatal(err)
		}
	}

	lead := messaging.Caller{Agent: "lead-home", Grants: []messaging.Grant{{Channel: "#franco"}}}

	closes := askAs(t, env, messaging.Caller{Task: "old-done"}, "old-worker", "#old-done", "QUESTION: which command runs the msb server?")
	leadIDs := askAs(t, env, lead, "lead", "#franco", "Accept old-done?\nIt adds a thing.", "old-done: adds a thing", "Accept open-task?")
	closes = append(closes, leadIDs[0], leadIDs[1])
	later := []int64{leadIDs[2]}
	later = append(later, askAs(t, env, messaging.Caller{Task: "open-task"}, "open-worker", "#open-task", "Which branch?")...)
	keeps := askAs(t, env, messaging.Caller{Human: true}, "franco", "#old-done", "Why this way?")

	env.api.tick(ctx)
	// Twice: the second tick finds nothing left to close.
	env.api.tick(ctx)

	open1 := openQuestions(t, env)
	for _, id := range closes {
		if open1[id] {
			t.Fatalf("question %d still open after the tick", id)
		}
	}
	for _, id := range append(append([]int64{}, later...), keeps...) {
		if !open1[id] {
			t.Fatalf("question %d closed by the tick", id)
		}
	}

	replies := closingReplies(t, env)
	if len(replies) != len(closes) {
		t.Fatalf("closing replies %+v, want %d", replies, len(closes))
	}
	for i, reply := range replies {
		if reply.Body != "Closed: old-done accepted by franco." || reply.ReplyTo == nil || *reply.ReplyTo != closes[i] || !msgmodels.FromSystem(&reply) {
			t.Fatalf("closing reply %+v", reply)
		}
	}

	// The clock moves on and open-task is accepted without closing its
	// questions: the next tick does.
	now = now.Add(time.Hour)
	_, err := open.MarkAccepted(ctx, env.db, "lead")
	if err != nil {
		t.Fatal(err)
	}

	env.api.tick(ctx)

	open2 := openQuestions(t, env)
	for _, id := range later {
		if open2[id] {
			t.Fatalf("question %d still open after open-task was done", id)
		}
	}
	for _, id := range keeps {
		if !open2[id] {
			t.Fatalf("person's question %d closed", id)
		}
	}

	if len(closingReplies(t, env)) != len(closes)+len(later) {
		t.Fatalf("closing replies %+v", closingReplies(t, env))
	}
}
