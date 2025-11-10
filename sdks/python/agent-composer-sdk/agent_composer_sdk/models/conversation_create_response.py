from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.conversation_create_response_conversations_item import ConversationCreateResponseConversationsItem


T = TypeVar("T", bound="ConversationCreateResponse")


@_attrs_define
class ConversationCreateResponse:
    """
    Attributes:
        conversations (list[ConversationCreateResponseConversationsItem]):
    """

    conversations: list[ConversationCreateResponseConversationsItem]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        conversations = []
        for conversations_item_data in self.conversations:
            conversations_item = conversations_item_data.to_dict()
            conversations.append(conversations_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "conversations": conversations,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.conversation_create_response_conversations_item import ConversationCreateResponseConversationsItem

        d = dict(src_dict)
        conversations = []
        _conversations = d.pop("conversations")
        for conversations_item_data in _conversations:
            conversations_item = ConversationCreateResponseConversationsItem.from_dict(conversations_item_data)

            conversations.append(conversations_item)

        conversation_create_response = cls(
            conversations=conversations,
        )

        conversation_create_response.additional_properties = d
        return conversation_create_response

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
