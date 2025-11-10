from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, cast
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.hook_event_type import HookEventType
from ..types import UNSET, Unset

T = TypeVar("T", bound="UpdateHookRequest")


@_attrs_define
class UpdateHookRequest:
    """Supply at least one mutable field; otherwise the service returns EINVALID.

    Attributes:
        hook_id (UUID | Unset): Filled from the path parameter; omit in the payload.
        event_type (HookEventType | Unset):
        agent_name (str | Unset):
        command (str | Unset):
        args (list[str] | None | Unset):
        enabled (bool | Unset):
    """

    hook_id: UUID | Unset = UNSET
    event_type: HookEventType | Unset = UNSET
    agent_name: str | Unset = UNSET
    command: str | Unset = UNSET
    args: list[str] | None | Unset = UNSET
    enabled: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        hook_id: str | Unset = UNSET
        if not isinstance(self.hook_id, Unset):
            hook_id = str(self.hook_id)

        event_type: str | Unset = UNSET
        if not isinstance(self.event_type, Unset):
            event_type = self.event_type.value

        agent_name = self.agent_name

        command = self.command

        args: list[str] | None | Unset
        if isinstance(self.args, Unset):
            args = UNSET
        elif isinstance(self.args, list):
            args = self.args

        else:
            args = self.args

        enabled = self.enabled

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if hook_id is not UNSET:
            field_dict["hook_id"] = hook_id
        if event_type is not UNSET:
            field_dict["event_type"] = event_type
        if agent_name is not UNSET:
            field_dict["agent_name"] = agent_name
        if command is not UNSET:
            field_dict["command"] = command
        if args is not UNSET:
            field_dict["args"] = args
        if enabled is not UNSET:
            field_dict["enabled"] = enabled

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        _hook_id = d.pop("hook_id", UNSET)
        hook_id: UUID | Unset
        if isinstance(_hook_id, Unset):
            hook_id = UNSET
        else:
            hook_id = UUID(_hook_id)

        _event_type = d.pop("event_type", UNSET)
        event_type: HookEventType | Unset
        if isinstance(_event_type, Unset):
            event_type = UNSET
        else:
            event_type = HookEventType(_event_type)

        agent_name = d.pop("agent_name", UNSET)

        command = d.pop("command", UNSET)

        def _parse_args(data: object) -> list[str] | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, list):
                    raise TypeError()
                args_type_0 = cast(list[str], data)

                return args_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(list[str] | None | Unset, data)

        args = _parse_args(d.pop("args", UNSET))

        enabled = d.pop("enabled", UNSET)

        update_hook_request = cls(
            hook_id=hook_id,
            event_type=event_type,
            agent_name=agent_name,
            command=command,
            args=args,
            enabled=enabled,
        )

        update_hook_request.additional_properties = d
        return update_hook_request

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
