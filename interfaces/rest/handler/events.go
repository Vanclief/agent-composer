package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/events"
)

// eventsHeartbeat is how often GET /api/events sends a comment, so proxies
// and the browser keep an idle stream open.
const eventsHeartbeat = 20 * time.Second

// GET /events
//
// Server-sent events: each event is `id: <n>`, `event: <type>` and
// `data: <json>`, plus `: ping` every 20 seconds. The stream ends when the
// client goes away, or when it falls too far behind (the bus drops it and
// EventSource reconnects). Nothing is replayed.
func (h *Handler) Events(c echo.Context) error {
	const op = "Handler.Events"

	// The stream ends with the client or with the server, so shutdown
	// does not wait on open streams.
	ctx, cancel := context.WithCancel(c.Request().Context())
	defer cancel()

	if h.server.RootContext != nil {
		stop := context.AfterFunc(h.server.RootContext, cancel)
		defer stop()
	}

	request := requests.New(c.Request().Header, c.RealIP())
	// The dispatch's request timeout must not end the stream: it gets the
	// connection's own context instead.
	request.Cancel()
	request.SetContext(ctx)
	request.SetBody(&events.SubscribeRequest{})

	response, err := h.server.HandleRequest(request)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	feed, ok := response.(<-chan events.Event)
	if !ok {
		return h.ManageError(c, op, request, ez.New(ez.EINTERNAL, "events subscription is not a channel", nil))
	}

	header := c.Response().Header()
	header.Set(echo.HeaderContentType, "text/event-stream")
	header.Set(echo.HeaderCacheControl, "no-cache")
	header.Set(echo.HeaderConnection, "keep-alive")
	// nginx and similar proxies would otherwise buffer the stream.
	header.Set("X-Accel-Buffering", "no")
	c.Response().WriteHeader(http.StatusOK)

	err = writeSSE(c, ": connected\n\n")
	if err != nil {
		return nil
	}

	heartbeat := time.NewTicker(eventsHeartbeat)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-heartbeat.C:
			err = writeSSE(c, ": ping\n\n")
			if err != nil {
				return nil
			}
		case event, open := <-feed:
			if !open {
				return nil
			}

			data, err := json.Marshal(event.Data)
			if err != nil {
				log.Warn().Err(err).Str("event", event.Type).Msg("cannot encode event")
				continue
			}

			err = writeSSE(c, fmt.Sprintf("id: %d\nevent: %s\ndata: %s\n\n", event.ID, event.Type, data))
			if err != nil {
				return nil
			}
		}
	}
}

// writeSSE writes one frame and flushes it to the client.
func writeSSE(c echo.Context, frame string) error {
	_, err := c.Response().Write([]byte(frame))
	if err != nil {
		return err
	}

	c.Response().Flush()

	return nil
}
