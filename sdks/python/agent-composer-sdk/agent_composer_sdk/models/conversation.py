from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from dateutil.parser import isoparse

from ..models.conversation_status import ConversationStatus
from ..models.llm_provider import LLMProvider
from ..models.reasoning_effort import ReasoningEffort
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.conversation_structured_output_schema_type_0 import ConversationStructuredOutputSchemaType0
    from ..models.message import Message


T = TypeVar("T", bound="Conversation")


@_attrs_define
class Conversation:
    """
    Attributes:
        id (UUID):
        agent_spec_id (UUID):
        agent_name (str):
        provider (LLMProvider):
        model (str):
        reasoning_effort (ReasoningEffort):
        instructions (str):
        messages (list[Message]):
        status (ConversationStatus):
        input_tokens (int):
        output_tokens (int):
        cached_tokens (int):
        cost (int): Tracked cost in the smallest billing unit for the provider.
        created_at (datetime.datetime):
        auto_compact (bool):
        compact_at_percent (int):
        compact_count (int):
        shell_access (bool):
        web_search (bool):
        structured_output (bool):
        session_id (None | str | Unset):
        compaction_prompt (str | Unset):
        structured_output_schema (ConversationStructuredOutputSchemaType0 | None | Unset):
    """

    id: UUID
    agent_spec_id: UUID
    agent_name: str
    provider: LLMProvider
    model: str
    reasoning_effort: ReasoningEffort
    instructions: str
    messages: list[Message]
    status: ConversationStatus
    input_tokens: int
    output_tokens: int
    cached_tokens: int
    cost: int
    created_at: datetime.datetime
    auto_compact: bool
    compact_at_percent: int
    compact_count: int
    shell_access: bool
    web_search: bool
    structured_output: bool
    session_id: None | str | Unset = UNSET
    compaction_prompt: str | Unset = UNSET
    structured_output_schema: ConversationStructuredOutputSchemaType0 | None | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        from ..models.conversation_structured_output_schema_type_0 import ConversationStructuredOutputSchemaType0

        id = str(self.id)

        agent_spec_id = str(self.agent_spec_id)

        agent_name = self.agent_name

        provider = self.provider.value

        model = self.model

        reasoning_effort = self.reasoning_effort.value

        instructions = self.instructions

        messages = []
        for messages_item_data in self.messages:
            messages_item = messages_item_data.to_dict()
            messages.append(messages_item)

        status = self.status.value

        input_tokens = self.input_tokens

        output_tokens = self.output_tokens

        cached_tokens = self.cached_tokens

        cost = self.cost

        created_at = self.created_at.isoformat()

        auto_compact = self.auto_compact

        compact_at_percent = self.compact_at_percent

        compact_count = self.compact_count

        shell_access = self.shell_access

        web_search = self.web_search

        structured_output = self.structured_output

        session_id: None | str | Unset
        if isinstance(self.session_id, Unset):
            session_id = UNSET
        else:
            session_id = self.session_id

        compaction_prompt = self.compaction_prompt

        structured_output_schema: dict[str, Any] | None | Unset
        if isinstance(self.structured_output_schema, Unset):
            structured_output_schema = UNSET
        elif isinstance(self.structured_output_schema, ConversationStructuredOutputSchemaType0):
            structured_output_schema = self.structured_output_schema.to_dict()
        else:
            structured_output_schema = self.structured_output_schema

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "agent_spec_id": agent_spec_id,
                "agent_name": agent_name,
                "provider": provider,
                "model": model,
                "reasoning_effort": reasoning_effort,
                "instructions": instructions,
                "messages": messages,
                "status": status,
                "input_tokens": input_tokens,
                "output_tokens": output_tokens,
                "cached_tokens": cached_tokens,
                "cost": cost,
                "created_at": created_at,
                "auto_compact": auto_compact,
                "compact_at_percent": compact_at_percent,
                "compact_count": compact_count,
                "shell_access": shell_access,
                "web_search": web_search,
                "structured_output": structured_output,
            }
        )
        if session_id is not UNSET:
            field_dict["session_id"] = session_id
        if compaction_prompt is not UNSET:
            field_dict["compaction_prompt"] = compaction_prompt
        if structured_output_schema is not UNSET:
            field_dict["structured_output_schema"] = structured_output_schema

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.conversation_structured_output_schema_type_0 import ConversationStructuredOutputSchemaType0
        from ..models.message import Message

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        agent_spec_id = UUID(d.pop("agent_spec_id"))

        agent_name = d.pop("agent_name")

        provider = LLMProvider(d.pop("provider"))

        model = d.pop("model")

        reasoning_effort = ReasoningEffort(d.pop("reasoning_effort"))

        instructions = d.pop("instructions")

        messages = []
        _messages = d.pop("messages")
        for messages_item_data in _messages:
            messages_item = Message.from_dict(messages_item_data)

            messages.append(messages_item)

        status = ConversationStatus(d.pop("status"))

        input_tokens = d.pop("input_tokens")

        output_tokens = d.pop("output_tokens")

        cached_tokens = d.pop("cached_tokens")

        cost = d.pop("cost")

        created_at = isoparse(d.pop("created_at"))

        auto_compact = d.pop("auto_compact")

        compact_at_percent = d.pop("compact_at_percent")

        compact_count = d.pop("compact_count")

        shell_access = d.pop("shell_access")

        web_search = d.pop("web_search")

        structured_output = d.pop("structured_output")

        def _parse_session_id(data: object) -> None | str | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(None | str | Unset, data)

        session_id = _parse_session_id(d.pop("session_id", UNSET))

        compaction_prompt = d.pop("compaction_prompt", UNSET)

        def _parse_structured_output_schema(data: object) -> ConversationStructuredOutputSchemaType0 | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, dict):
                    raise TypeError()
                structured_output_schema_type_0 = ConversationStructuredOutputSchemaType0.from_dict(data)

                return structured_output_schema_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(ConversationStructuredOutputSchemaType0 | None | Unset, data)

        structured_output_schema = _parse_structured_output_schema(d.pop("structured_output_schema", UNSET))

        conversation = cls(
            id=id,
            agent_spec_id=agent_spec_id,
            agent_name=agent_name,
            provider=provider,
            model=model,
            reasoning_effort=reasoning_effort,
            instructions=instructions,
            messages=messages,
            status=status,
            input_tokens=input_tokens,
            output_tokens=output_tokens,
            cached_tokens=cached_tokens,
            cost=cost,
            created_at=created_at,
            auto_compact=auto_compact,
            compact_at_percent=compact_at_percent,
            compact_count=compact_count,
            shell_access=shell_access,
            web_search=web_search,
            structured_output=structured_output,
            session_id=session_id,
            compaction_prompt=compaction_prompt,
            structured_output_schema=structured_output_schema,
        )

        conversation.additional_properties = d
        return conversation

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
