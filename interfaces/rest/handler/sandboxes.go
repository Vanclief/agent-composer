package handler

import (
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/compose/components/rest/requests"
)

// GET /sandboxes?work=1
func (h *Handler) ListSandboxes(c echo.Context) error {
	const op = "Handler.ListSandboxes"

	request := requests.New(c.Request().Header, c.RealIP())
	work := c.QueryParam("work")

	return h.JSONResponse(c, op, request, &sandboxes.ListRequest{Work: work == "1" || work == "true"})
}

// GET /sandboxes/:id
func (h *Handler) GetSandbox(c echo.Context) error {
	const op = "Handler.GetSandbox"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &sandboxes.GetRequest{ID: id, Name: name})
}

// POST /sandboxes/:id/stop
func (h *Handler) StopSandbox(c echo.Context) error {
	const op = "Handler.StopSandbox"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &sandboxes.StopRequest{ID: id, Name: name})
}

// POST /sandboxes/:id/start
func (h *Handler) StartSandbox(c echo.Context) error {
	const op = "Handler.StartSandbox"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &sandboxes.StartRequest{ID: id, Name: name})
}

// DELETE /sandboxes/:id
func (h *Handler) DeleteSandbox(c echo.Context) error {
	const op = "Handler.DeleteSandbox"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &sandboxes.DeleteRequest{ID: id, Name: name})
}

// GET /sandboxes/:id/transcripts
func (h *Handler) ListTranscripts(c echo.Context) error {
	const op = "Handler.ListTranscripts"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &sandboxes.ListTranscriptsRequest{ID: id, Name: name})
}

// GET /sandboxes/:id/transcripts/*
//
// Streams one transcript as JSON Lines, read from the host side of the
// sandbox's volume. A plain echo handler, not the request dispatch, so a
// large file is streamed instead of buffered and is not cut off by the
// dispatch's request timeout.
func (h *Handler) GetTranscript(c echo.Context) error {
	const op = "Handler.GetTranscript"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	transcriptPath, err := url.PathUnescape(c.Param("*"))
	if err != nil {
		return h.ManageError(c, op, request, ez.New(ez.EINVALID, "path is not a valid URL path", err))
	}

	file, err := h.server.SandboxesAPI.OpenTranscript(c.Request().Context(), nil, &sandboxes.GetTranscriptRequest{ID: id, Name: name, Path: transcriptPath})
	if err != nil {
		return h.ManageError(c, op, request, err)
	}
	defer file.Close()

	c.Response().Header().Set(echo.HeaderContentType, "application/x-ndjson")
	http.ServeContent(c.Response(), c.Request(), file.Name, file.ModifiedAt, file)

	return nil
}
