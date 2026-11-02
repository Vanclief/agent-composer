package messaging

import (
	"context"
	"testing"

	"github.com/vanclief/ez"
)

// ADR 0016 item 9: the agent whose role is main may use every channel;
// any other agent only its grants, and tasks are unchanged.
func TestAuthorizeLetsOnlyMainEverywhere(t *testing.T) {
	svc := newTestService(t)
	svc.SetMainAgent(func(home string) bool {
		return home == "main-1"
	})

	main := Caller{Agent: "main-1"}
	reviewer := Caller{Agent: "reviewer-1", Grants: []Grant{{Channel: "#review-42"}}}

	for _, channel := range []string{"#review-42", "#franco", "#anything"} {
		for _, post := range []bool{false, true} {
			err := svc.authorize(main, channel, post)
			if err != nil {
				t.Fatalf("main in %s (post %v): %v", channel, post, err)
			}
		}
	}

	err := svc.authorize(reviewer, "#review-42", true)
	if err != nil {
		t.Fatalf("reviewer in its granted channel: %v", err)
	}

	for _, caller := range []Caller{reviewer, review, other} {
		err = svc.authorize(caller, "#franco", false)
		if ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
			t.Fatalf("%+v reading #franco: %v, want ENOTAUTHORIZED", caller, err)
		}
	}

	// An agent has no channel of its own: #reviewer-1 is nobody's task.
	err = svc.authorize(reviewer, "#reviewer-1", false)
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
		t.Fatalf("reviewer in #reviewer-1: %v, want ENOTAUTHORIZED", err)
	}

	// Without the check wired, nobody is main.
	plain := newTestService(t)
	err = plain.authorize(main, "#franco", false)
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
		t.Fatalf("main without SetMainAgent: %v, want ENOTAUTHORIZED", err)
	}
}

func TestAgentCallerOwnsItsParticipantsOnly(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	svc.SetMainAgent(func(home string) bool {
		return home == "main-1"
	})

	main := Caller{Agent: "main-1"}

	p, err := svc.Join(ctx, main, "main-1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Agent != "main-1" || p.Task != "" || p.Human {
		t.Fatalf("participant = %+v", p)
	}

	m, err := svc.Post(ctx, main, p.ID, "#review-42", "on it", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Agent != "main-1" || m.Task != "" {
		t.Fatalf("message = %+v", m)
	}

	// Neither a task nor another agent may post as it.
	for _, caller := range []Caller{review, {Agent: "reviewer-1", Grants: []Grant{{Channel: "#review-42"}}}} {
		_, err = svc.Post(ctx, caller, p.ID, "#review-42", "spoofed", nil, "")
		if ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
			t.Fatalf("%+v posting as main-1: %v, want ENOTAUTHORIZED", caller, err)
		}
	}

	// main sees every channel, as a human does.
	channels, err := svc.Channels(ctx, main, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].Channel != "#review-42" {
		t.Fatalf("channels = %+v", channels)
	}

	// An agent's posts never pass for agc's.
	_, err = svc.Join(ctx, Caller{Agent: "x", Task: "y"}, "both")
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
		t.Fatalf("caller with a task and an agent: %v, want ENOTAUTHORIZED", err)
	}
}
