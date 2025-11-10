from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="CursorPage")


@_attrs_define
class CursorPage:
    """
    Attributes:
        has_next_page (bool): Indicates whether another page exists.
        next_cursor (str | Unset): Cursor to fetch the next page, omitted if there is no next page.
        hash_ (str | Unset): SHA-256 hash of the returned items for cache validation.
    """

    has_next_page: bool
    next_cursor: str | Unset = UNSET
    hash_: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        has_next_page = self.has_next_page

        next_cursor = self.next_cursor

        hash_ = self.hash_

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "has_next_page": has_next_page,
            }
        )
        if next_cursor is not UNSET:
            field_dict["next_cursor"] = next_cursor
        if hash_ is not UNSET:
            field_dict["hash"] = hash_

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        has_next_page = d.pop("has_next_page")

        next_cursor = d.pop("next_cursor", UNSET)

        hash_ = d.pop("hash", UNSET)

        cursor_page = cls(
            has_next_page=has_next_page,
            next_cursor=next_cursor,
            hash_=hash_,
        )

        cursor_page.additional_properties = d
        return cursor_page

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
