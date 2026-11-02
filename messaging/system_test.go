package messaging

import (
	"context"
	"strings"
	"testing"

	"github.com/vanclief/ez"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

func TestSystemParticipantIsReserved(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	for _, caller := range []Caller{human, review} {
		_, err := svc.Join(ctx, caller, "agc")
		if ez.ErrorCode(err) != ez.ECONFLICT || !strings.Contains(ez.ErrorMessage(err), "reserved") {
			t.Fatalf("join agc as %+v: %v", caller, err)
		}
	}

	var hooked []string
	svc.OnPost(func(ctx context.Context, m *msgmodels.Message) {
		hooked = append(hooked, m.From+": "+m.Body)
	})

	first, err := svc.PostAsSystem(ctx, "#review-42", "task is open")
	if err != nil {
		t.Fatal(err)
	}

	second, err := svc.PostAsSystem(ctx, "#review-42", "instructions")
	if err != nil {
		t.Fatal(err)
	}

	if first.From != "agc" || first.Task != "" || first.Human || first.ParticipantID != second.ParticipantID {
		t.Fatalf("system messages = %+v, %+v", first, second)
	}

	franco := join(t, svc, human, "franco")
	reply := post(t, svc, human, franco, "#review-42", "thanks")
	if !reply.Human {
		t.Fatalf("a human's message has human false: %+v", reply)
	}

	reviewer := join(t, svc, review, "reviewer")
	answer := post(t, svc, review, reviewer, "#review-42", "on it")
	if answer.Human || answer.Task != "review-42" {
		t.Fatalf("an agent's message = %+v", answer)
	}

	want := []string{"agc: task is open", "agc: instructions", "franco: thanks", "reviewer: on it"}
	if strings.Join(hooked, "|") != strings.Join(want, "|") {
		t.Fatalf("hooks saw %q", hooked)
	}

	err = svc.Leave(ctx, human, first.ParticipantID)
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
		t.Fatalf("a human released agc: %v", err)
	}
}
