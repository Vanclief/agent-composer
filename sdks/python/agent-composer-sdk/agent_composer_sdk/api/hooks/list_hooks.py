from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error_response import ErrorResponse
from ...models.hook_event_type import HookEventType
from ...models.hook_list_response import HookListResponse
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    search: str | Unset = UNSET,
    event_type: HookEventType | Unset = UNSET,
    agent_name: str | Unset = UNSET,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    params["limit"] = limit

    params["cursor"] = cursor

    params["search"] = search

    json_event_type: str | Unset = UNSET
    if not isinstance(event_type, Unset):
        json_event_type = event_type.value

    params["event_type"] = json_event_type

    params["agent_name"] = agent_name

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/hooks",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ErrorResponse | HookListResponse | None:
    if response.status_code == 200:
        response_200 = HookListResponse.from_dict(response.json())

        return response_200

    if response.status_code == 400:
        response_400 = ErrorResponse.from_dict(response.json())

        return response_400

    if response.status_code == 500:
        response_500 = ErrorResponse.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ErrorResponse | HookListResponse]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    search: str | Unset = UNSET,
    event_type: HookEventType | Unset = UNSET,
    agent_name: str | Unset = UNSET,
) -> Response[ErrorResponse | HookListResponse]:
    """List hooks

     Returns hooks ordered by cursor. `search` matches the agent name or command, `event_type` narrows
    down to a hook category, and `agent_name` filters to hooks bound to one agent.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        search (str | Unset):
        event_type (HookEventType | Unset):
        agent_name (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ErrorResponse | HookListResponse]
    """

    kwargs = _get_kwargs(
        limit=limit,
        cursor=cursor,
        search=search,
        event_type=event_type,
        agent_name=agent_name,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    search: str | Unset = UNSET,
    event_type: HookEventType | Unset = UNSET,
    agent_name: str | Unset = UNSET,
) -> ErrorResponse | HookListResponse | None:
    """List hooks

     Returns hooks ordered by cursor. `search` matches the agent name or command, `event_type` narrows
    down to a hook category, and `agent_name` filters to hooks bound to one agent.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        search (str | Unset):
        event_type (HookEventType | Unset):
        agent_name (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ErrorResponse | HookListResponse
    """

    return sync_detailed(
        client=client,
        limit=limit,
        cursor=cursor,
        search=search,
        event_type=event_type,
        agent_name=agent_name,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    search: str | Unset = UNSET,
    event_type: HookEventType | Unset = UNSET,
    agent_name: str | Unset = UNSET,
) -> Response[ErrorResponse | HookListResponse]:
    """List hooks

     Returns hooks ordered by cursor. `search` matches the agent name or command, `event_type` narrows
    down to a hook category, and `agent_name` filters to hooks bound to one agent.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        search (str | Unset):
        event_type (HookEventType | Unset):
        agent_name (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ErrorResponse | HookListResponse]
    """

    kwargs = _get_kwargs(
        limit=limit,
        cursor=cursor,
        search=search,
        event_type=event_type,
        agent_name=agent_name,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    search: str | Unset = UNSET,
    event_type: HookEventType | Unset = UNSET,
    agent_name: str | Unset = UNSET,
) -> ErrorResponse | HookListResponse | None:
    """List hooks

     Returns hooks ordered by cursor. `search` matches the agent name or command, `event_type` narrows
    down to a hook category, and `agent_name` filters to hooks bound to one agent.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        search (str | Unset):
        event_type (HookEventType | Unset):
        agent_name (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ErrorResponse | HookListResponse
    """

    return (
        await asyncio_detailed(
            client=client,
            limit=limit,
            cursor=cursor,
            search=search,
            event_type=event_type,
            agent_name=agent_name,
        )
    ).parsed
