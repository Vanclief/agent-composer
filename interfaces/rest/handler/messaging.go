package handler

import (
	"net/http"

	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/messaging/httpapi"
)

// Messaging serves the messaging API for humans, mounted at /api/msg. A
// human is bound by no channel rules (ADR 0005).
func (h *Handler) Messaging() http.Handler {
	humanCaller := func(*http.Request) (messaging.Caller, error) {
		return messaging.Caller{Human: true}, nil
	}

	return http.StripPrefix("/api/msg", httpapi.New(h.server.Messaging, humanCaller))
}
