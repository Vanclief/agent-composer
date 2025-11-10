from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.conversation_list_response import ConversationListResponse
from ...models.conversation_status import ConversationStatus
from ...models.error_response import ErrorResponse
from ...models.llm_provider import LLMProvider
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    search: str | Unset = UNSET,
    provider: LLMProvider | Unset = UNSET,
    agent_spec_id: UUID | Unset = UNSET,
    status: ConversationStatus | Unset = UNSET,
    session_id: str | Unset = UNSET,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    params["limit"] = limit

    params["cursor"] = cursor

    params["search"] = search

    json_provider: str | Unset = UNSET
    if not isinstance(provider, Unset):
        json_provider = provider.value

    params["provider"] = json_provider

    json_agent_spec_id: str | Unset = UNSET
    if not isinstance(agent_spec_id, Unset):
        json_agent_spec_id = str(agent_spec_id)
    params["agent_spec_id"] = json_agent_spec_id

    json_status: str | Unset = UNSET
    if not isinstance(status, Unset):
        json_status = status.value

    params["status"] = json_status

    params["session_id"] = session_id

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/agents/conversations",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ConversationListResponse | ErrorResponse | None:
    if response.status_code == 200:
        response_200 = ConversationListResponse.from_dict(response.json())

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
) -> Response[ConversationListResponse | ErrorResponse]:
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
    provider: LLMProvider | Unset = UNSET,
    agent_spec_id: UUID | Unset = UNSET,
    status: ConversationStatus | Unset = UNSET,
    session_id: str | Unset = UNSET,
) -> Response[ConversationListResponse | ErrorResponse]:
    """List conversations

     Returns the newest conversations first. `search` matches agent names, `agent_spec_id` filters to a
    single spec, `provider` filters by LLM provider, `status` filters the conversation lifecycle state,
    and `session_id` restricts results to a client session.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        search (str | Unset):
        provider (LLMProvider | Unset):
        agent_spec_id (UUID | Unset):
        status (ConversationStatus | Unset):
        session_id (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ConversationListResponse | ErrorResponse]
    """

    kwargs = _get_kwargs(
        limit=limit,
        cursor=cursor,
        search=search,
        provider=provider,
        agent_spec_id=agent_spec_id,
        status=status,
        session_id=session_id,
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
    provider: LLMProvider | Unset = UNSET,
    agent_spec_id: UUID | Unset = UNSET,
    status: ConversationStatus | Unset = UNSET,
    session_id: str | Unset = UNSET,
) -> ConversationListResponse | ErrorResponse | None:
    """List conversations

     Returns the newest conversations first. `search` matches agent names, `agent_spec_id` filters to a
    single spec, `provider` filters by LLM provider, `status` filters the conversation lifecycle state,
    and `session_id` restricts results to a client session.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        search (str | Unset):
        provider (LLMProvider | Unset):
        agent_spec_id (UUID | Unset):
        status (ConversationStatus | Unset):
        session_id (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ConversationListResponse | ErrorResponse
    """

    return sync_detailed(
        client=client,
        limit=limit,
        cursor=cursor,
        search=search,
        provider=provider,
        agent_spec_id=agent_spec_id,
        status=status,
        session_id=session_id,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    search: str | Unset = UNSET,
    provider: LLMProvider | Unset = UNSET,
    agent_spec_id: UUID | Unset = UNSET,
    status: ConversationStatus | Unset = UNSET,
    session_id: str | Unset = UNSET,
) -> Response[ConversationListResponse | ErrorResponse]:
    """List conversations

     Returns the newest conversations first. `search` matches agent names, `agent_spec_id` filters to a
    single spec, `provider` filters by LLM provider, `status` filters the conversation lifecycle state,
    and `session_id` restricts results to a client session.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        search (str | Unset):
        provider (LLMProvider | Unset):
        agent_spec_id (UUID | Unset):
        status (ConversationStatus | Unset):
        session_id (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ConversationListResponse | ErrorResponse]
    """

    kwargs = _get_kwargs(
        limit=limit,
        cursor=cursor,
        search=search,
        provider=provider,
        agent_spec_id=agent_spec_id,
        status=status,
        session_id=session_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = 50,
    cursor: str | Unset = UNSET,
    search: str | Unset = UNSET,
    provider: LLMProvider | Unset = UNSET,
    agent_spec_id: UUID | Unset = UNSET,
    status: ConversationStatus | Unset = UNSET,
    session_id: str | Unset = UNSET,
) -> ConversationListResponse | ErrorResponse | None:
    """List conversations

     Returns the newest conversations first. `search` matches agent names, `agent_spec_id` filters to a
    single spec, `provider` filters by LLM provider, `status` filters the conversation lifecycle state,
    and `session_id` restricts results to a client session.

    Args:
        limit (int | Unset):  Default: 50.
        cursor (str | Unset):
        search (str | Unset):
        provider (LLMProvider | Unset):
        agent_spec_id (UUID | Unset):
        status (ConversationStatus | Unset):
        session_id (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ConversationListResponse | ErrorResponse
    """

    return (
        await asyncio_detailed(
            client=client,
            limit=limit,
            cursor=cursor,
            search=search,
            provider=provider,
            agent_spec_id=agent_spec_id,
            status=status,
            session_id=session_id,
        )
    ).parsed
