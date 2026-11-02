package handler

import (
	"context"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/vanclief/compose/components/rest/requests"

	"github.com/vanclief/agent-composer/core/resources/peers"
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/messaging/httpapi"
)

// GET /servers
func (h *Handler) ListServers(c echo.Context) error {
	const op = "Handler.ListServers"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &peers.ListServersRequest{})
}

// RequirePeerToken guards the peer routes, /api/peer/*: the request must
// carry this server's AGC_PEER_TOKEN (ADR 0018).
func (h *Handler) RequirePeerToken(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := peers.CheckToken(h.server.PeersAPI.SecretsPath(), c.Request().Header.Get("Authorization"))
		if err != nil {
			return h.ManageError(c, "Handler.RequirePeerToken", requests.New(c.Request().Header, c.RealIP()), err)
		}

		return next(c)
	}
}

// GET /peer/host
func (h *Handler) PeerHost(c echo.Context) error {
	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, "Handler.PeerHost", request, &peers.HostRequest{})
}

// POST /peer/secrets/check
func (h *Handler) PeerCheckSecrets(c echo.Context) error {
	request := requests.New(c.Request().Header, c.RealIP())

	return h.BindedJSONResponse(c, "Handler.PeerCheckSecrets", request, &peers.CheckSecretsRequest{})
}

// POST /peer/vms
func (h *Handler) PeerCreateVM(c echo.Context) error {
	request := requests.New(c.Request().Header, c.RealIP())

	return h.BindedJSONResponse(c, "Handler.PeerCreateVM", request, &peers.CreateVMRequest{})
}

// GET /peer/vms/:name
func (h *Handler) PeerVMStatus(c echo.Context) error {
	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, "Handler.PeerVMStatus", request, &peers.VMStatusRequest{Name: c.Param("name")})
}

// POST /peer/vms/:name/start
func (h *Handler) PeerStartVM(c echo.Context) error {
	const op = "Handler.PeerStartVM"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &peers.StartVMRequest{}
	err := c.Bind(body)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}
	body.Name = c.Param("name")

	return h.JSONResponse(c, op, request, body)
}

// POST /peer/vms/:name/stop
func (h *Handler) PeerStopVM(c echo.Context) error {
	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, "Handler.PeerStopVM", request, &peers.StopVMRequest{Name: c.Param("name")})
}

// DELETE /peer/vms/:name
func (h *Handler) PeerRemoveVM(c echo.Context) error {
	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, "Handler.PeerRemoveVM", request, &peers.RemoveVMRequest{Name: c.Param("name")})
}

// POST /peer/vms/:name/exec
func (h *Handler) PeerExec(c echo.Context) error {
	const op = "Handler.PeerExec"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &peers.ExecRequest{}
	err := c.Bind(body)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}
	body.Name = c.Param("name")

	return h.JSONResponse(c, op, request, body)
}

// GET /peer/vms/:name/terminals/:job/ws?replay=N
//
// The panel's terminal socket for a VM on this server, for the primary,
// which relays it to the panel (ADR 0018).
func (h *Handler) PeerTerminalSocket(c echo.Context) error {
	replay := replayBytes

	parsed, err := strconv.Atoi(c.QueryParam("replay"))
	if err == nil && parsed >= 0 {
		replay = parsed
	}

	return h.serveTerminal(c, "Handler.PeerTerminalSocket", replay, func(ctx context.Context) (executor.TerminalExecutor, func(), error) {
		return h.server.PeersAPI.ConnectTerminal(ctx, c.Param("name"))
	})
}

// ANY /peer/sockets/:name/*
//
// A request a peer forwarded from the socket of the sandbox :name, which
// it runs for this server, answered as the sandbox's own socket would be
// (ADR 0018). Errors use the socket's shape, which the guest's agc reads.
func (h *Handler) PeerSocket(c echo.Context) error {
	w := c.Response()
	r := c.Request()

	err := peers.CheckToken(h.server.PeersAPI.SecretsPath(), r.Header.Get("Authorization"))
	if err != nil {
		httpapi.WriteError(w, err)
		return nil
	}

	forwarded := r.Clone(r.Context())
	forwarded.URL.Path = "/" + c.Param("*")
	forwarded.URL.RawPath = ""
	forwarded.RequestURI = forwarded.URL.RequestURI()

	h.server.SandboxesAPI.ServeForwarded(w, forwarded, c.Param("name"))

	return nil
}
