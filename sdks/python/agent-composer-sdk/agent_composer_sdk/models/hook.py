from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, cast
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.hook_event_type import HookEventType

T = TypeVar("T", bound="Hook")


@_attrs_define
class Hook:
    """
    Attributes:
        id (UUID):
        event_type (HookEventType):
        agent_name (str): Empty string represents a wildcard hook.
        command (str):
        args (list[str]):
        enabled (bool):
    """

    id: UUID
    event_type: HookEventType
    agent_name: str
    command: str
    args: list[str]
    enabled: bool
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        event_type = self.event_type.value

        agent_name = self.agent_name

        command = self.command

        args = self.args

        enabled = self.enabled

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "event_type": event_type,
                "agent_name": agent_name,
                "command": command,
                "args": args,
                "enabled": enabled,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        event_type = HookEventType(d.pop("event_type"))

        agent_name = d.pop("agent_name")

        command = d.pop("command")

        args = cast(list[str], d.pop("args"))

        enabled = d.pop("enabled")

        hook = cls(
            id=id,
            event_type=event_type,
            agent_name=agent_name,
            command=command,
            args=args,
            enabled=enabled,
        )

        hook.additional_properties = d
        return hook

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
