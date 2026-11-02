package events

import (
	"context"
	"testing"
	"time"
)

func receive(t *testing.T, ch <-chan Event) Event {
	t.Helper()

	select {
	case event, ok := <-ch:
		if !ok {
			t.Fatal("channel closed")
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("no event")
	}

	return Event{}
}

func TestPublishSubscribe(t *testing.T) {
	bus := New()
	ctx, cancel := context.WithCancel(context.Background())

	a := bus.Subscribe(ctx)
	b := bus.Subscribe(ctx)

	bus.Publish(Event{Type: TaskCreated, Data: "one"})
	bus.Publish(Event{Type: TaskDone, Data: "two"})

	for _, ch := range []<-chan Event{a, b} {
		first := receive(t, ch)
		if first.Type != TaskCreated || first.Data != "one" || first.ID != 1 {
			t.Fatalf("first event %+v", first)
		}

		second := receive(t, ch)
		if second.Type != TaskDone || second.ID != 2 {
			t.Fatalf("second event %+v", second)
		}
	}

	cancel()

	deadline := time.Now().Add(time.Second)
	for bus.Subscribers() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d subscribers left after cancel", bus.Subscribers())
		}
		time.Sleep(time.Millisecond)
	}

	_, ok := <-a
	if ok {
		t.Fatal("channel open after cancel")
	}
}

func TestSlowSubscriberDropped(t *testing.T) {
	bus := New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slow := bus.Subscribe(ctx)
	fast := bus.Subscribe(ctx)

	for i := 0; i < Buffer+1; i++ {
		bus.Publish(Event{Type: MessagePosted, Data: i})
		receive(t, fast)
	}

	if bus.Subscribers() != 1 {
		t.Fatalf("%d subscribers, want the fast one only", bus.Subscribers())
	}

	count := 0
	for range slow {
		count++
	}
	if count != Buffer {
		t.Fatalf("slow subscriber got %d events before it was closed, want %d", count, Buffer)
	}

	bus.Publish(Event{Type: MessagePosted, Data: "after"})
	receive(t, fast)
}

func TestNilBus(t *testing.T) {
	var bus *Bus

	bus.Publish(Event{Type: TaskCreated})

	_, ok := <-bus.Subscribe(context.Background())
	if ok {
		t.Fatal("nil bus channel is open")
	}
}
