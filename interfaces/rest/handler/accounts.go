package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/accounts"
	"github.com/vanclief/compose/components/rest/requests"
)

// Accounts (ADR 0018). :name is the account's name. No route returns an
// account's value.

// GET /accounts
func (h *Handler) ListAccounts(c echo.Context) error {
	const op = "Handler.ListAccounts"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &accounts.ListRequest{})
}

// POST /accounts answers 201 with the account. The body carries the
// value; it is written to secrets.env and never echoed or logged
// (accounts.CreateRequest marshals without it).
func (h *Handler) CreateAccount(c echo.Context) error {
	const op = "Handler.CreateAccount"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &accounts.CreateRequest{}
	err := c.Bind(body)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	request.SetBody(body)

	response, err := h.App.HandleRequest(request)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return c.JSON(http.StatusCreated, response)
}

// POST /accounts/:name/default
func (h *Handler) SetDefaultAccount(c echo.Context) error {
	const op = "Handler.SetDefaultAccount"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &accounts.SetDefaultRequest{Name: c.Param("name")})
}

// DELETE /accounts/:name/default
func (h *Handler) ClearDefaultAccount(c echo.Context) error {
	const op = "Handler.ClearDefaultAccount"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &accounts.ClearDefaultRequest{Name: c.Param("name")})
}

// DELETE /accounts/:name answers 204.
func (h *Handler) DeleteAccount(c echo.Context) error {
	const op = "Handler.DeleteAccount"

	request := requests.New(c.Request().Header, c.RealIP())
	request.SetBody(&accounts.DeleteRequest{Name: c.Param("name")})

	_, err := h.App.HandleRequest(request)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// POST /accounts/:name/signin begins a sign-in and answers the page to
// open (ADR 0018 "Sign-in").
func (h *Handler) SigninAccount(c echo.Context) error {
	const op = "Handler.SigninAccount"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &accounts.SigninRequest{Name: c.Param("name")})
}

// POST /accounts/:name/signin/complete finishes it with the pasted code,
// which is never echoed or logged (accounts.SigninCompleteRequest
// marshals without it).
func (h *Handler) CompleteSigninAccount(c echo.Context) error {
	const op = "Handler.CompleteSigninAccount"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &accounts.SigninCompleteRequest{}
	err := c.Bind(body)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}
	body.Name = c.Param("name")

	return h.JSONResponse(c, op, request, body)
}

// DELETE /accounts/:name/signin signs the account out.
func (h *Handler) SignoutAccount(c echo.Context) error {
	const op = "Handler.SignoutAccount"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &accounts.SignoutRequest{Name: c.Param("name")})
}
