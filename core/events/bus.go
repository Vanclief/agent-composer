// Package events is agc's in-process event bus, the source of the live
// stream GET /api/events. Publish never blocks: a subscriber whose buffer
// is full is dropped and its channel closed.
package events

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
)

// The event types GET /api/events sends, as panel/src/api/events.ts reads
// them.
const (
	TaskCreated   = "task.created"
	TaskDone      = "task.done"
	SandboxStatus = "sandbox.status"
	MessagePosted = "message.posted"
	// ScheduleRan is published after every tick of a schedule (ADR 0012).
	ScheduleRan = "schedule.ran"
	// AgentState is published on every state change of a persistent
	// agent's home, and on create, keep and delete (ADR 0016).
	AgentState = "agent.state"
	// ChannelArchived is published when agc rest archives a done task's
	// channel (ADR 0005 "Archived channels").
	ChannelArchived = "channel.archived"
)

// Buffer is how many events a subscriber may fall behind before it is
// dropped.
const Buffer = 256

// Event is one thing that happened. Data is marshalled as the SSE data.
// ID is the event's number, set by Publish.
type Event struct {
	ID   uint64 `json:"id"`
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Bus fans events out to its subscribers. A nil Bus discards every event,
// so publishers need not check.
type Bus struct {
	mu     sync.Mutex
	subs   map[*subscriber]struct{}
	nextID atomic.Uint64
}

type subscriber struct {
	ch chan Event
}

func New() *Bus {
	return &Bus{subs: map[*subscriber]struct{}{}}
}

// Publish numbers the event and hands it to every subscriber. A
// subscriber with a full buffer is dropped, logged once, and its channel
// closed.
func (b *Bus) Publish(event Event) {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	event.ID = b.nextID.Add(1)

	for sub := range b.subs {
		select {
		case sub.ch <- event:
		default:
			delete(b.subs, sub)
			close(sub.ch)
			log.Warn().Str("event", event.Type).Int("buffer", Buffer).Msg("dropping a slow event subscriber")
		}
	}
}

// Subscribe returns a channel of every event published from now until ctx
// ends or the subscriber falls Buffer events behind, and the channel is then
// closed.
func (b *Bus) Subscribe(ctx context.Context) <-chan Event {
	sub := &subscriber{ch: make(chan Event, Buffer)}

	if b == nil {
		close(sub.ch)
		return sub.ch
	}

	b.mu.Lock()
	b.subs[sub] = struct{}{}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.remove(sub)
	}()

	return sub.ch
}

// remove closes sub's channel unless Publish has already dropped it.
func (b *Bus) remove(sub *subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, ok := b.subs[sub]
	if !ok {
		return
	}

	delete(b.subs, sub)
	close(sub.ch)
}

// Subscribers is how many subscribers are attached, for tests.
func (b *Bus) Subscribers() int {
	if b == nil {
		return 0
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	return len(b.subs)
}

// SubscribeRequest is GET /api/events. It has no fields.
type SubscribeRequest struct{}

func (r SubscribeRequest) Validate() error {
	return nil
}
