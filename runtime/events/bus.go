package events

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/vanclief/compose/types"
	"github.com/vanclief/ez"
)

type FilterFunc func(event Event) bool

type subscription struct {
	id     int
	name   string
	ch     chan Event
	filter FilterFunc
}

type Bus struct {
	mu          sync.RWMutex
	ctx         context.Context
	closed      bool
	subscribers map[int]*subscription
	nextSubID   int
	nextEventID uint64
}

// NewBus creates a bus with no external deps.
func NewBus(ctx context.Context) (*Bus, error) {
	const op = "events.NewBus"

	if ctx == nil {
		return nil, ez.Root(op, ez.EINVALID, "Context is nil")
	}

	return &Bus{
		ctx:         ctx,
		subscribers: make(map[int]*subscription),
	}, nil
}

func (b *Bus) Publish(event Event) error {
	const op = "Bus.Publish"

	// Get a copy of current subscribers
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ez.Root(op, ez.EINVALID, "Bus is closed")
	}

	subscribersCopy := make([]*subscription, 0, len(b.subscribers))
	for _, subscriber := range b.subscribers {
		subscribersCopy = append(subscribersCopy, subscriber)
	}
	b.mu.RUnlock()

	// Send the events to the subscribers
	event.ID = atomic.AddUint64(&b.nextEventID, 1)
	event.Timestamp = types.UnixSecondsNow()

	for _, subscriber := range subscribersCopy {
		matches := true
		if subscriber.filter != nil {
			matches = subscriber.filter(event)
		}
		if !matches {
			continue
		}
		select {
		case subscriber.ch <- event:
			// delivered
		case <-b.ctx.Done():
			return ez.New(op, ez.EUNAVAILABLE, "Publish interrupted", b.ctx.Err())
		}
	}

	return nil
}

// Subscribe registers a consumer with an optional filter and buffer. Returns a read-only channel and an unsubscribe func.
func (b *Bus) Subscribe(name string, filter FilterFunc, buffer int) (<-chan Event, func() error, error) {
	const op = "Bus.Subscribe"

	if buffer < 1 {
		return nil, nil, ez.New(op, ez.EINVALID, "Buffer size cannot be less than 1", nil)
	}

	// Lock the bus until we finish adding the subscriber
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, nil, ez.New(op, ez.EUNAVAILABLE, "Bus is closed", nil)
	}

	subscriptionID := b.nextSubID
	b.nextSubID++

	eventChan := make(chan Event, buffer)

	newSubscription := &subscription{
		id:     subscriptionID,
		name:   name,
		ch:     eventChan,
		filter: filter,
	}
	b.subscribers[subscriptionID] = newSubscription

	unsubscribe := func() error {
		b.mu.Lock()
		defer b.mu.Unlock()

		_, exists := b.subscribers[subscriptionID]
		if !exists {
			return ez.New(op, ez.ENOTFOUND, "Subscription not found", nil)
		}

		delete(b.subscribers, subscriptionID)
		close(eventChan)
		return nil
	}

	return eventChan, unsubscribe, nil
}

// Close shuts the bus down and closes all subscriber channels.
func (b *Bus) Close() error {
	const op = "Bus.Close"

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ez.New("Bus.Close", ez.EINVALID, "Bus is already closed", nil)
	}

	b.closed = true
	for id, subscriber := range b.subscribers {
		delete(b.subscribers, id)
		close(subscriber.ch)
	}

	return nil
}
