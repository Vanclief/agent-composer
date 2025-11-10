from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.agent_spec import AgentSpec
from ...models.error_response import ErrorResponse
from ...models.update_agent_spec_request import UpdateAgentSpecRequest
from ...types import Response


def _get_kwargs(
    id: UUID,
    *,
    body: UpdateAgentSpecRequest,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": f"/agents/specs/{id}",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AgentSpec | ErrorResponse | None:
    if response.status_code == 200:
        response_200 = AgentSpec.from_dict(response.json())

        return response_200

    if response.status_code == 400:
        response_400 = ErrorResponse.from_dict(response.json())

        return response_400

    if response.status_code == 404:
        response_404 = ErrorResponse.from_dict(response.json())

        return response_404

    if response.status_code == 500:
        response_500 = ErrorResponse.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[AgentSpec | ErrorResponse]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UpdateAgentSpecRequest,
) -> Response[AgentSpec | ErrorResponse]:
    """Update an agent spec

    Args:
        id (UUID):
        body (UpdateAgentSpecRequest): Supply at least one mutable field; otherwise the service
            returns EINVALID.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentSpec | ErrorResponse]
    """

    kwargs = _get_kwargs(
        id=id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UpdateAgentSpecRequest,
) -> AgentSpec | ErrorResponse | None:
    """Update an agent spec

    Args:
        id (UUID):
        body (UpdateAgentSpecRequest): Supply at least one mutable field; otherwise the service
            returns EINVALID.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AgentSpec | ErrorResponse
    """

    return sync_detailed(
        id=id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UpdateAgentSpecRequest,
) -> Response[AgentSpec | ErrorResponse]:
    """Update an agent spec

    Args:
        id (UUID):
        body (UpdateAgentSpecRequest): Supply at least one mutable field; otherwise the service
            returns EINVALID.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentSpec | ErrorResponse]
    """

    kwargs = _get_kwargs(
        id=id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UpdateAgentSpecRequest,
) -> AgentSpec | ErrorResponse | None:
    """Update an agent spec

    Args:
        id (UUID):
        body (UpdateAgentSpecRequest): Supply at least one mutable field; otherwise the service
            returns EINVALID.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AgentSpec | ErrorResponse
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            body=body,
        )
    ).parsed
