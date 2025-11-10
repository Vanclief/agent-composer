from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.hook_event_type import HookEventType
from ..types import UNSET, Unset

T = TypeVar("T", bound="CreateHookRequest")


@_attrs_define
class CreateHookRequest:
    """
    Attributes:
        event_type (HookEventType):
        command (str):
        enabled (bool):
        agent_name (str | Unset): Empty string (or omission) registers a wildcard hook.
        args (list[str] | Unset):
    """

    event_type: HookEventType
    command: str
    enabled: bool
    agent_name: str | Unset = UNSET
    args: list[str] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        event_type = self.event_type.value

        command = self.command

        enabled = self.enabled

        agent_name = self.agent_name

        args: list[str] | Unset = UNSET
        if not isinstance(self.args, Unset):
            args = self.args

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "event_type": event_type,
                "command": command,
                "enabled": enabled,
            }
        )
        if agent_name is not UNSET:
            field_dict["agent_name"] = agent_name
        if args is not UNSET:
            field_dict["args"] = args

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        event_type = HookEventType(d.pop("event_type"))

        command = d.pop("command")

        enabled = d.pop("enabled")

        agent_name = d.pop("agent_name", UNSET)

        args = cast(list[str], d.pop("args", UNSET))

        create_hook_request = cls(
            event_type=event_type,
            command=command,
            enabled=enabled,
            agent_name=agent_name,
            args=args,
        )

        create_hook_request.additional_properties = d
        return create_hook_request

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
