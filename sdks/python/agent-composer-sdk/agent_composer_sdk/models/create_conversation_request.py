from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="CreateConversationRequest")


@_attrs_define
class CreateConversationRequest:
    """
    Attributes:
        agent_spec_id (UUID):
        prompt (str):
        parallel_conversations (int | Unset): Number of conversation instances to launch in parallel. Values <1 default
            to 1. Default: 1.
        session_id (str | Unset): Optional client-provided identifier for logical sessions.
    """

    agent_spec_id: UUID
    prompt: str
    parallel_conversations: int | Unset = 1
    session_id: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        agent_spec_id = str(self.agent_spec_id)

        prompt = self.prompt

        parallel_conversations = self.parallel_conversations

        session_id = self.session_id

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "agent_spec_id": agent_spec_id,
                "prompt": prompt,
            }
        )
        if parallel_conversations is not UNSET:
            field_dict["parallel_conversations"] = parallel_conversations
        if session_id is not UNSET:
            field_dict["session_id"] = session_id

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        agent_spec_id = UUID(d.pop("agent_spec_id"))

        prompt = d.pop("prompt")

        parallel_conversations = d.pop("parallel_conversations", UNSET)

        session_id = d.pop("session_id", UNSET)

        create_conversation_request = cls(
            agent_spec_id=agent_spec_id,
            prompt=prompt,
            parallel_conversations=parallel_conversations,
            session_id=session_id,
        )

        create_conversation_request.additional_properties = d
        return create_conversation_request

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
