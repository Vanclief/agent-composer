package tasks

import (
	"context"
	"fmt"
	"testing"

	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// Questions for a person are the ones in their inbox and in #<task> of
// the tasks they manage; a task they only own is its manager's to answer,
// who raises to the owner in their inbox (ADR 0009 item 5).
func TestQuestionsForManagerNotOwner(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	human := messaging.Caller{Human: true}

	tasks := []CreateRequest{
		{Name: "managed", Brief: "b", Manager: "lead", Owner: "franco"},
		{Name: "owned", Brief: "b", Manager: "other", Owner: "lead"},
		{Name: "unrelated", Brief: "b", Manager: "other", Owner: "franco"},
	}
	for i := range tasks {
		_, err := env.api.Create(ctx, nil, &tasks[i])
		if err != nil {
			t.Fatal(err)
		}
	}

	p, err := env.msg.Join(ctx, human, "asker")
	if err != nil {
		t.Fatal(err)
	}

	asked := map[string]int64{}
	for _, channel := range []string{"#managed", "#owned", "#unrelated", "#lead", "#franco"} {
		m, err := env.msg.Post(ctx, human, p.ID, channel, "which?", nil, msgmodels.KindQuestion)
		if err != nil {
			t.Fatal(err)
		}
		asked[channel] = m.ID
	}

	got, err := env.msg.Questions(ctx, human, messaging.QuestionsFilter{For: "lead"})
	if err != nil {
		t.Fatal(err)
	}

	ids := []int64{}
	for _, q := range got {
		ids = append(ids, q.ID)
	}

	want := []int64{asked["#lead"], asked["#managed"]}
	if fmt.Sprint(ids) != fmt.Sprint(want) {
		t.Fatalf("questions for lead = %v, want %v", ids, want)
	}
}
