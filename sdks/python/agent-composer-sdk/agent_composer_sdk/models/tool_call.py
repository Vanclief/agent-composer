from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_value_type_0 import JSONValueType0


T = TypeVar("T", bound="ToolCall")


@_attrs_define
class ToolCall:
    """
    Attributes:
        name (str | Unset):
        call_id (str | Unset):
        arguments (str | Unset): Raw JSON arguments returned by the model.
        json_arguments (bool | float | int | JSONValueType0 | list[Any] | None | str | Unset): Represents an arbitrary
            JSON value.
    """

    name: str | Unset = UNSET
    call_id: str | Unset = UNSET
    arguments: str | Unset = UNSET
    json_arguments: bool | float | int | JSONValueType0 | list[Any] | None | str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        from ..models.json_value_type_0 import JSONValueType0

        name = self.name

        call_id = self.call_id

        arguments = self.arguments

        json_arguments: bool | dict[str, Any] | float | int | list[Any] | None | str | Unset
        if isinstance(self.json_arguments, Unset):
            json_arguments = UNSET
        elif isinstance(self.json_arguments, JSONValueType0):
            json_arguments = self.json_arguments.to_dict()
        elif isinstance(self.json_arguments, list):
            json_arguments = self.json_arguments

        else:
            json_arguments = self.json_arguments

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if name is not UNSET:
            field_dict["Name"] = name
        if call_id is not UNSET:
            field_dict["CallID"] = call_id
        if arguments is not UNSET:
            field_dict["Arguments"] = arguments
        if json_arguments is not UNSET:
            field_dict["JSONArguments"] = json_arguments

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.json_value_type_0 import JSONValueType0

        d = dict(src_dict)
        name = d.pop("Name", UNSET)

        call_id = d.pop("CallID", UNSET)

        arguments = d.pop("Arguments", UNSET)

        def _parse_json_arguments(data: object) -> bool | float | int | JSONValueType0 | list[Any] | None | str | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, dict):
                    raise TypeError()
                componentsschemas_json_value_type_0 = JSONValueType0.from_dict(data)

                return componentsschemas_json_value_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            try:
                if not isinstance(data, list):
                    raise TypeError()
                componentsschemas_json_value_type_1 = cast(list[Any], data)

                return componentsschemas_json_value_type_1
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(bool | float | int | JSONValueType0 | list[Any] | None | str | Unset, data)

        json_arguments = _parse_json_arguments(d.pop("JSONArguments", UNSET))

        tool_call = cls(
            name=name,
            call_id=call_id,
            arguments=arguments,
            json_arguments=json_arguments,
        )

        return tool_call
