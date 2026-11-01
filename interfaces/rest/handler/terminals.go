package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/executor"
)

// replayBytes is how much recent output a new attachment replays.
const replayBytes = 64 * 1024

// ConnectTerminal connects to the VM of the task named by ref, a task UUID
// or name. release frees the handle and leaves the VM and its jobs running.
type ConnectTerminal func(ctx context.Context, ref string) (exe executor.TerminalExecutor, release func(), err error)

// GET /tasks/:id/terminals
func (h *Handler) ListTerminals(c echo.Context) error {
	const op = "Handler.ListTerminals"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &tasks.ListTerminalsRequest{ID: id, Name: name})
}

// POST /tasks/:id/terminals
func (h *Handler) StartTerminal(c echo.Context) error {
	const op = "Handler.StartTerminal"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.BindedJSONResponse(c, op, request, &tasks.StartTerminalRequest{ID: id, Name: name})
}

// resizeMessage is the one text frame the client sends.
type resizeMessage struct {
	Type string `json:"type"`
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

// GET /tasks/:id/terminals/:job/ws
//
// Binary frames carry terminal bytes both ways. A text frame
// {"type":"resize","rows":N,"cols":N} resizes the terminal. The socket closes
// normally with reason "job completed" when the program exits. Closing the
// socket detaches, and the job keeps running.
//
// This is a plain echo handler, not the request dispatch, because it hijacks
// the connection and must not inherit the dispatch's request timeout.
func (h *Handler) TerminalSocket(c echo.Context) error {
	const op = "Handler.TerminalSocket"

	ctx, cancel := context.WithCancel(c.Request().Context())
	defer cancel()

	exe, release, err := h.ConnectTerminal(ctx, c.Param("id"))
	if err != nil {
		return h.ManageError(c, op, requests.New(c.Request().Header, c.RealIP()), ez.Wrap(err))
	}
	defer release()

	stream, err := exe.AttachTerminal(ctx, c.Param("job"), replayBytes)
	if err != nil {
		return h.ManageError(c, op, requests.New(c.Request().Header, c.RealIP()), ez.Wrap(err))
	}
	defer stream.Close()

	// Default options authorize only the request's own host as Origin, which
	// is the control panel. Other sites cannot open a shell in a Task.
	conn, err := websocket.Accept(c.Response(), c.Request(), nil)
	if err != nil {
		// Accept has already written the HTTP error.
		return nil
	}
	// Pasted text arrives as one frame, so allow more than the 32 KiB default.
	conn.SetReadLimit(1 << 20)

	inputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		defer cancel()
		forwardInput(ctx, conn, stream)
	}()
	defer func() {
		conn.CloseNow()
		<-inputDone
	}()

	for {
		data, err := stream.Recv(ctx)
		if errors.Is(err, io.EOF) {
			conn.Close(websocket.StatusNormalClosure, "job completed")
			return nil
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			conn.Close(websocket.StatusInternalError, ez.ErrorMessage(err))
			return nil
		}

		err = conn.Write(ctx, websocket.MessageBinary, data)
		if err != nil {
			return nil
		}
	}
}

// forwardInput copies the socket's frames into the terminal until the socket
// closes or a write fails.
func forwardInput(ctx context.Context, conn *websocket.Conn, stream executor.TerminalStream) {
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		if typ == websocket.MessageBinary {
			err = stream.Write(ctx, data)
			if err != nil {
				return
			}
			continue
		}

		var msg resizeMessage
		err = json.Unmarshal(data, &msg)
		if err != nil || msg.Type != "resize" {
			continue
		}

		err = stream.Resize(ctx, msg.Rows, msg.Cols)
		if err != nil {
			return
		}
	}
}
