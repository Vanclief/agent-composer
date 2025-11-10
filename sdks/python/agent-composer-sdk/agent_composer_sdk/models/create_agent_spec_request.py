from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.llm_provider import LLMProvider
from ..models.reasoning_effort import ReasoningEffort
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.create_agent_spec_request_structured_output_schema import CreateAgentSpecRequestStructuredOutputSchema


T = TypeVar("T", bound="CreateAgentSpecRequest")


@_attrs_define
class CreateAgentSpecRequest:
    """
    Attributes:
        name (str):
        provider (LLMProvider):
        model (str):
        instructions (str):
        reasoning_effort (ReasoningEffort):
        auto_compact (bool | Unset):  Default: False.
        compact_at_percent (int | Unset): Optional override for when automatic compaction triggers (percentage of
            context window).
        compaction_prompt (str | Unset): Custom system prompt injected before compaction runs.
        allowed_tools (list[str] | Unset):
        shell_access (bool | Unset):
        web_search (bool | Unset):
        structured_output (bool | Unset):
        structured_output_schema (CreateAgentSpecRequestStructuredOutputSchema | Unset): Required when
            `structured_output` is true.
    """

    name: str
    provider: LLMProvider
    model: str
    instructions: str
    reasoning_effort: ReasoningEffort
    auto_compact: bool | Unset = False
    compact_at_percent: int | Unset = UNSET
    compaction_prompt: str | Unset = UNSET
    allowed_tools: list[str] | Unset = UNSET
    shell_access: bool | Unset = UNSET
    web_search: bool | Unset = UNSET
    structured_output: bool | Unset = UNSET
    structured_output_schema: CreateAgentSpecRequestStructuredOutputSchema | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        provider = self.provider.value

        model = self.model

        instructions = self.instructions

        reasoning_effort = self.reasoning_effort.value

        auto_compact = self.auto_compact

        compact_at_percent = self.compact_at_percent

        compaction_prompt = self.compaction_prompt

        allowed_tools: list[str] | Unset = UNSET
        if not isinstance(self.allowed_tools, Unset):
            allowed_tools = self.allowed_tools

        shell_access = self.shell_access

        web_search = self.web_search

        structured_output = self.structured_output

        structured_output_schema: dict[str, Any] | Unset = UNSET
        if not isinstance(self.structured_output_schema, Unset):
            structured_output_schema = self.structured_output_schema.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "name": name,
                "provider": provider,
                "model": model,
                "instructions": instructions,
                "reasoning_effort": reasoning_effort,
            }
        )
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
        from ..models.create_agent_spec_request_structured_output_schema import (
            CreateAgentSpecRequestStructuredOutputSchema,
        )

        d = dict(src_dict)
        name = d.pop("name")

        provider = LLMProvider(d.pop("provider"))

        model = d.pop("model")

        instructions = d.pop("instructions")

        reasoning_effort = ReasoningEffort(d.pop("reasoning_effort"))

        auto_compact = d.pop("auto_compact", UNSET)

        compact_at_percent = d.pop("compact_at_percent", UNSET)

        compaction_prompt = d.pop("compaction_prompt", UNSET)

        allowed_tools = cast(list[str], d.pop("allowed_tools", UNSET))

        shell_access = d.pop("shell_access", UNSET)

        web_search = d.pop("web_search", UNSET)

        structured_output = d.pop("structured_output", UNSET)

        _structured_output_schema = d.pop("structured_output_schema", UNSET)
        structured_output_schema: CreateAgentSpecRequestStructuredOutputSchema | Unset
        if isinstance(_structured_output_schema, Unset):
            structured_output_schema = UNSET
        else:
            structured_output_schema = CreateAgentSpecRequestStructuredOutputSchema.from_dict(_structured_output_schema)

        create_agent_spec_request = cls(
            name=name,
            provider=provider,
            model=model,
            instructions=instructions,
            reasoning_effort=reasoning_effort,
            auto_compact=auto_compact,
            compact_at_percent=compact_at_percent,
            compaction_prompt=compaction_prompt,
            allowed_tools=allowed_tools,
            shell_access=shell_access,
            web_search=web_search,
            structured_output=structured_output,
            structured_output_schema=structured_output_schema,
        )

        create_agent_spec_request.additional_properties = d
        return create_agent_spec_request

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
