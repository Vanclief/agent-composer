package messaging

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/core/events"
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
