package messaging

import "testing"

func TestTurnFor(t *testing.T) {
	manager := &Participant{ID: "p-rev", Name: "reviewer-1", Agent: "reviewer-1"}

	cases := []struct {
		name string
		m    Message
		want bool
	}{
		{"a person's post", Message{From: "franco", ParticipantID: "p-franco", Human: true, Body: "looks good", Kind: KindMessage}, true},
		{"agc's delivery, for the manager", Message{From: SystemName, ParticipantID: "p-agc", Body: "Delivered: added tests", Kind: KindMessage}, true},
		{"another agent's plain post", Message{From: "fix-login", ParticipantID: "p-task", Task: "fix-login", Body: "Nothing new to do.", Kind: KindMessage}, false},
		{"another agent's question", Message{From: "fix-login", ParticipantID: "p-task", Task: "fix-login", Body: "which branch?", Kind: KindQuestion}, true},
		{"another agent's post naming me", Message{From: "watcher-1", ParticipantID: "p-watch", Agent: "watcher-1", Body: "@reviewer-1 please fetch", Kind: KindMessage}, true},
		{"another agent's post naming someone else", Message{From: "watcher-1", ParticipantID: "p-watch", Agent: "watcher-1", Body: "@reviewer-10 please fetch", Kind: KindMessage}, false},
		{"my own post", Message{From: "reviewer-1", ParticipantID: "p-rev", Agent: "reviewer-1", Body: "which?", Kind: KindQuestion}, false},
		{"agc's own accept of mine", Message{From: SystemName, ParticipantID: "p-agc", Body: "Accepted by reviewer-1.", Kind: KindMessage}, false},
		{"agc's other post", Message{From: SystemName, ParticipantID: "p-agc", Body: "Task fix-login created.", Kind: KindMessage}, false},
	}

	for _, c := range cases {
		got := TurnFor(&c.m, manager)
		if got != c.want {
			t.Errorf("%s: TurnFor = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestNames(t *testing.T) {
	cases := map[string]bool{
		"@reviewer-1 fetch":        true,
		"ask @reviewer-1.":         true,
		"@reviewer-10 fetch":       false,
		"@reviewer-1x":             false,
		"@reviewer-10 @reviewer-1": true,
		"reviewer-1":               false,
	}
	for body, want := range cases {
		got := Names(body, "reviewer-1")
		if got != want {
			t.Errorf("Names(%q) = %v, want %v", body, got, want)
		}
	}
}
