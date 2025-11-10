from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.llm_provider import LLMProvider
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.update_agent_spec_request_structured_output_schema_type_0 import (
        UpdateAgentSpecRequestStructuredOutputSchemaType0,
    )


T = TypeVar("T", bound="UpdateAgentSpecRequest")


@_attrs_define
class UpdateAgentSpecRequest:
    """Supply at least one mutable field; otherwise the service returns EINVALID.

    Attributes:
        agent_spec_id (UUID | Unset): Filled from the path parameter; omit in the payload.
        provider (LLMProvider | Unset):
        name (str | Unset):
        model (str | Unset):
        instructions (str | Unset):
        auto_compact (bool | Unset):
        compact_at_percent (int | Unset):
        compaction_prompt (str | Unset):
        allowed_tools (list[str] | Unset):
        shell_access (bool | Unset):
        web_search (bool | Unset):
        structured_output (bool | Unset):
        structured_output_schema (None | Unset | UpdateAgentSpecRequestStructuredOutputSchemaType0): Send null to clear
            the structured output schema.
    """

    agent_spec_id: UUID | Unset = UNSET
    provider: LLMProvider | Unset = UNSET
    name: str | Unset = UNSET
    model: str | Unset = UNSET
    instructions: str | Unset = UNSET
    auto_compact: bool | Unset = UNSET
    compact_at_percent: int | Unset = UNSET
    compaction_prompt: str | Unset = UNSET
    allowed_tools: list[str] | Unset = UNSET
    shell_access: bool | Unset = UNSET
    web_search: bool | Unset = UNSET
    structured_output: bool | Unset = UNSET
    structured_output_schema: None | Unset | UpdateAgentSpecRequestStructuredOutputSchemaType0 = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        from ..models.update_agent_spec_request_structured_output_schema_type_0 import (
            UpdateAgentSpecRequestStructuredOutputSchemaType0,
        )

        agent_spec_id: str | Unset = UNSET
        if not isinstance(self.agent_spec_id, Unset):
            agent_spec_id = str(self.agent_spec_id)

        provider: str | Unset = UNSET
        if not isinstance(self.provider, Unset):
            provider = self.provider.value

        name = self.name

        model = self.model

        instructions = self.instructions

        auto_compact = self.auto_compact

        compact_at_percent = self.compact_at_percent

        compaction_prompt = self.compaction_prompt

        allowed_tools: list[str] | Unset = UNSET
        if not isinstance(self.allowed_tools, Unset):
            allowed_tools = self.allowed_tools

        shell_access = self.shell_access

        web_search = self.web_search

        structured_output = self.structured_output

        structured_output_schema: dict[str, Any] | None | Unset
        if isinstance(self.structured_output_schema, Unset):
            structured_output_schema = UNSET
        elif isinstance(self.structured_output_schema, UpdateAgentSpecRequestStructuredOutputSchemaType0):
            structured_output_schema = self.structured_output_schema.to_dict()
        else:
            structured_output_schema = self.structured_output_schema

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if agent_spec_id is not UNSET:
            field_dict["agent_spec_id"] = agent_spec_id
        if provider is not UNSET:
            field_dict["provider"] = provider
        if name is not UNSET:
            field_dict["name"] = name
        if model is not UNSET:
            field_dict["model"] = model
        if instructions is not UNSET:
            field_dict["instructions"] = instructions
        if auto_compact is not UNSET:
            field_dict["auto_compact"] = auto_compact
        if compact_at_percent is not UNSET:
            field_dict["compact_at_percent"] = compact_at_percent
        if compaction_prompt is not UNSET:
            field_dict["compaction_prompt"] = compaction_prompt
        if allowed_tools is not UNSET:
            field_dict["allowed_tools"] = allowed_tools
        if shell_access is not UNSET:
            field_dict["shell_access"] = shell_access
        if web_search is not UNSET:
            field_dict["web_search"] = web_search
        if structured_output is not UNSET:
            field_dict["structured_output"] = structured_output
        if structured_output_schema is not UNSET:
            field_dict["structured_output_schema"] = structured_output_schema

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.update_agent_spec_request_structured_output_schema_type_0 import (
            UpdateAgentSpecRequestStructuredOutputSchemaType0,
        )

        d = dict(src_dict)
        _agent_spec_id = d.pop("agent_spec_id", UNSET)
        agent_spec_id: UUID | Unset
        if isinstance(_agent_spec_id, Unset):
            agent_spec_id = UNSET
        else:
            agent_spec_id = UUID(_agent_spec_id)

        _provider = d.pop("provider", UNSET)
        provider: LLMProvider | Unset
        if isinstance(_provider, Unset):
            provider = UNSET
        else:
            provider = LLMProvider(_provider)

        name = d.pop("name", UNSET)

        model = d.pop("model", UNSET)

        instructions = d.pop("instructions", UNSET)

        auto_compact = d.pop("auto_compact", UNSET)

        compact_at_percent = d.pop("compact_at_percent", UNSET)

        compaction_prompt = d.pop("compaction_prompt", UNSET)

        allowed_tools = cast(list[str], d.pop("allowed_tools", UNSET))

        shell_access = d.pop("shell_access", UNSET)

        web_search = d.pop("web_search", UNSET)

        structured_output = d.pop("structured_output", UNSET)

        def _parse_structured_output_schema(
            data: object,
        ) -> None | Unset | UpdateAgentSpecRequestStructuredOutputSchemaType0:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, dict):
                    raise TypeError()
                structured_output_schema_type_0 = UpdateAgentSpecRequestStructuredOutputSchemaType0.from_dict(data)

                return structured_output_schema_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(None | Unset | UpdateAgentSpecRequestStructuredOutputSchemaType0, data)

        structured_output_schema = _parse_structured_output_schema(d.pop("structured_output_schema", UNSET))

        update_agent_spec_request = cls(
            agent_spec_id=agent_spec_id,
            provider=provider,
            name=name,
            model=model,
            instructions=instructions,
            auto_compact=auto_compact,
            compact_at_percent=compact_at_percent,
            compaction_prompt=compaction_prompt,
            allowed_tools=allowed_tools,
            shell_access=shell_access,
            web_search=web_search,
            structured_output=structured_output,
            structured_output_schema=structured_output_schema,
        )

        update_agent_spec_request.additional_properties = d
        return update_agent_spec_request

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
