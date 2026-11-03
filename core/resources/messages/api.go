// Package messages is the REST face of messaging for what is not the
// messaging HTTP API itself (ADR 0005): today, a message's images, served
// as stored.
package messages

import (
	"github.com/vanclief/agent-composer/messaging"
)

type API struct {
	msg *messaging.Service
}

// NewAPI returns the messages API over the messaging service.
func NewAPI(msg *messaging.Service) *API {
	return &API{msg: msg}
}
