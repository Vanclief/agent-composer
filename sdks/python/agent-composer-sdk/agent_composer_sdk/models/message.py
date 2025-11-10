from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.message_role import MessageRole
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.tool_call import ToolCall


T = TypeVar("T", bound="Message")


@_attrs_define
class Message:
    """
    Attributes:
        role (MessageRole | Unset):
        content (str | Unset):
        name (str | Unset):
        tool_call_id (str | Unset):
        tool_call (ToolCall | Unset):
    """

    role: MessageRole | Unset = UNSET
    content: str | Unset = UNSET
    name: str | Unset = UNSET
    tool_call_id: str | Unset = UNSET
    tool_call: ToolCall | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        role: str | Unset = UNSET
        if not isinstance(self.role, Unset):
            role = self.role.value

        content = self.content

        name = self.name

        tool_call_id = self.tool_call_id

        tool_call: dict[str, Any] | Unset = UNSET
        if not isinstance(self.tool_call, Unset):
            tool_call = self.tool_call.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if role is not UNSET:
            field_dict["Role"] = role
        if content is not UNSET:
            field_dict["Content"] = content
        if name is not UNSET:
            field_dict["Name"] = name
        if tool_call_id is not UNSET:
            field_dict["ToolCallID"] = tool_call_id
        if tool_call is not UNSET:
            field_dict["ToolCall"] = tool_call

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.tool_call import ToolCall

        d = dict(src_dict)
        _role = d.pop("Role", UNSET)
        role: MessageRole | Unset
        if isinstance(_role, Unset):
            role = UNSET
        else:
            role = MessageRole(_role)

        content = d.pop("Content", UNSET)

        name = d.pop("Name", UNSET)

        tool_call_id = d.pop("ToolCallID", UNSET)

        _tool_call = d.pop("ToolCall", UNSET)
        tool_call: ToolCall | Unset
        if isinstance(_tool_call, Unset):
            tool_call = UNSET
        else:
            tool_call = ToolCall.from_dict(_tool_call)

        message = cls(
            role=role,
            content=content,
            name=name,
            tool_call_id=tool_call_id,
            tool_call=tool_call,
        )

        message.additional_properties = d
        return message

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
