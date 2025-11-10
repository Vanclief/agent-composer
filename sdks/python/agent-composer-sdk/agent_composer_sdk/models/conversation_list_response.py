from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.conversation import Conversation


T = TypeVar("T", bound="ConversationListResponse")


@_attrs_define
class ConversationListResponse:
    """
    Attributes:
        has_next_page (bool): Indicates whether another page exists.
        conversations (list[Conversation]):
        next_cursor (str | Unset): Cursor to fetch the next page, omitted if there is no next page.
        hash_ (str | Unset): SHA-256 hash of the returned items for cache validation.
    """

    has_next_page: bool
    conversations: list[Conversation]
    next_cursor: str | Unset = UNSET
    hash_: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        has_next_page = self.has_next_page

        conversations = []
        for conversations_item_data in self.conversations:
            conversations_item = conversations_item_data.to_dict()
            conversations.append(conversations_item)

        next_cursor = self.next_cursor

        hash_ = self.hash_

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "has_next_page": has_next_page,
                "conversations": conversations,
            }
        )
        if next_cursor is not UNSET:
            field_dict["next_cursor"] = next_cursor
        if hash_ is not UNSET:
            field_dict["hash"] = hash_

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.conversation import Conversation

        d = dict(src_dict)
        has_next_page = d.pop("has_next_page")

        conversations = []
        _conversations = d.pop("conversations")
        for conversations_item_data in _conversations:
            conversations_item = Conversation.from_dict(conversations_item_data)

            conversations.append(conversations_item)

        next_cursor = d.pop("next_cursor", UNSET)

        hash_ = d.pop("hash", UNSET)

        conversation_list_response = cls(
            has_next_page=has_next_page,
            conversations=conversations,
            next_cursor=next_cursor,
            hash_=hash_,
        )

        conversation_list_response.additional_properties = d
        return conversation_list_response

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
