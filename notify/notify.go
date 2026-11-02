// Package notify tells a human, outside agc, that something landed in
// their direct messages (ADR 0011). It knows nothing of channels or the
// database: the messaging service hands it a Notice.
package notify

import "context"

// Notice is one direct message for a human.
type Notice struct {
	// Human is who the message is for, the name in @<human>/<task>.
	Human string
	// Sender is who posted it, as agc recorded it.
	Sender string
	// Task is the task the direct message belongs to.
	Task string
	// Text is the message body.
	Text string
}

// Notifier delivers a Notice. An error means it was not delivered; the
// caller logs it and carries on, a failed notice never fails a post.
type Notifier interface {
	Notify(ctx context.Context, notice Notice) error
}
