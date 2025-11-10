from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.agent_spec_list_response import AgentSpecListResponse
from ...models.error_response import ErrorResponse
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    search: str | Unset = UNSET,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    params["limit"] = limit

    params["cursor"] = cursor

    params["search"] = search

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/agents/specs",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AgentSpecListResponse | ErrorResponse | None:
    if response.status_code == 200:
        response_200 = AgentSpecListResponse.from_dict(response.json())

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
) -> Response[AgentSpecListResponse | ErrorResponse]:
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
) -> Response[AgentSpecListResponse | ErrorResponse]:
    """List agent specs

     Returns the newest agent specs first. Results are cursor paginated and filtered only by an optional
    case-insensitive search over the spec name.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        search (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentSpecListResponse | ErrorResponse]
    """

    kwargs = _get_kwargs(
        limit=limit,
        cursor=cursor,
        search=search,
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
) -> AgentSpecListResponse | ErrorResponse | None:
    """List agent specs

     Returns the newest agent specs first. Results are cursor paginated and filtered only by an optional
    case-insensitive search over the spec name.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        search (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AgentSpecListResponse | ErrorResponse
    """

    return sync_detailed(
        client=client,
        limit=limit,
        cursor=cursor,
        search=search,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    search: str | Unset = UNSET,
) -> Response[AgentSpecListResponse | ErrorResponse]:
    """List agent specs

     Returns the newest agent specs first. Results are cursor paginated and filtered only by an optional
    case-insensitive search over the spec name.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        search (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentSpecListResponse | ErrorResponse]
    """

    kwargs = _get_kwargs(
        limit=limit,
        cursor=cursor,
        search=search,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    search: str | Unset = UNSET,
) -> AgentSpecListResponse | ErrorResponse | None:
    """List agent specs

     Returns the newest agent specs first. Results are cursor paginated and filtered only by an optional
    case-insensitive search over the spec name.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        search (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AgentSpecListResponse | ErrorResponse
    """

    return (
        await asyncio_detailed(
            client=client,
            limit=limit,
            cursor=cursor,
            search=search,
        )
    ).parsed
