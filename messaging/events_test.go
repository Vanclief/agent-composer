package messaging

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/core/events"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

func TestPostPublishesMessagePosted(t *testing.T) {
	svc := newTestService(t)
	bus := events.New()
	svc.SetEvents(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed := bus.Subscribe(ctx)

	franco := join(t, svc, human, "franco")
	stored := post(t, svc, human, franco, "#releases", "ship it")

	system, err := svc.PostAsSystem(context.Background(), "#releases", "from agc")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []int64{stored.ID, system.ID} {
		select {
		case event := <-feed:
			data, ok := event.Data.(PostedEvent)
			if event.Type != events.MessagePosted || !ok {
				t.Fatalf("event %+v", event)
			}
			if data.Message.ID != want || data.Message.Channel != "#releases" {
				t.Fatalf("message %+v, want id %d", data.Message, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("no message.posted for %d", want)
		}
	}

	if system.From != "agc" {
		t.Fatalf("system message from %s", system.From)
	}
}

// TestPostedEventCarriesTurnFor: turn_for is stored before message.posted,
// so the event, a read and the post hooks see the same names, and a hook's
// own post is published after the message that set it off.
func TestPostedEventCarriesTurnFor(t *testing.T) {
	svc := newTestService(t)
	bus := events.New()
	svc.SetEvents(bus)

	svc.SetTurns(func(ctx context.Context, m *msgmodels.Message) []string {
		if m.Channel == "#designer-1" && m.Human {
			return []string{"designer-1"}
		}

		return nil
	})

	hooked := [][]string{}
	svc.OnPost(func(ctx context.Context, m *msgmodels.Message) {
		hooked = append(hooked, m.TurnFor)
		if m.Human {
			_, err := svc.PostAsSystem(ctx, m.Channel, "noted")
			if err != nil {
				t.Error(err)
			}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed := bus.Subscribe(ctx)

	franco := join(t, svc, human, "franco")
	stored := post(t, svc, human, franco, "#designer-1", "make it blue")

	read, err := svc.Read(context.Background(), human, "#designer-1", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(read) != 2 || read[0].ID != stored.ID || !reflect.DeepEqual(read[0].TurnFor, []string{"designer-1"}) || len(read[1].TurnFor) != 0 {
		t.Fatalf("read = %+v", read)
	}

	if !reflect.DeepEqual(hooked, [][]string{{"designer-1"}, nil}) {
		t.Fatalf("hooks saw turn_for %q", hooked)
	}

	for i, want := range read {
		select {
		case event := <-feed:
			data, ok := event.Data.(PostedEvent)
			if event.Type != events.MessagePosted || !ok {
				t.Fatalf("event %+v", event)
			}

			// As the panel gets it: the JSON of the event's message.
			published, err := json.Marshal(data.Message)
			if err != nil {
				t.Fatal(err)
			}
			decoded := struct {
				ID      int64    `json:"id"`
				TurnFor []string `json:"turn_for"`
			}{}
			err = json.Unmarshal(published, &decoded)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.ID != want.ID || !reflect.DeepEqual(decoded.TurnFor, want.TurnFor) {
				t.Fatalf("event %s, read turn_for %q of %d", published, want.TurnFor, want.ID)
			}
			if i == 0 && len(decoded.TurnFor) != 1 {
				t.Fatalf("event %s, want turn_for designer-1", published)
			}
		case <-time.After(time.Second):
			t.Fatalf("no message.posted for %d", want.ID)
		}
	}
}
