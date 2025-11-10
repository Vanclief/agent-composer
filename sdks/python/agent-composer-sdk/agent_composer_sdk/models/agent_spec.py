from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.llm_provider import LLMProvider
from ..models.reasoning_effort import ReasoningEffort
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_spec_structured_output_schema_type_0 import AgentSpecStructuredOutputSchemaType0


T = TypeVar("T", bound="AgentSpec")


@_attrs_define
class AgentSpec:
    """
    Attributes:
        id (UUID):
        name (str):
        provider (LLMProvider):
        model (str):
        reasoning_effort (ReasoningEffort):
        instructions (str):
        auto_compact (bool):
        compact_at_percent (int):
        shell_access (bool):
        web_search (bool):
        structured_output (bool):
        version (int):
        compaction_prompt (str | Unset):
        structured_output_schema (AgentSpecStructuredOutputSchemaType0 | None | Unset):
    """

    id: UUID
    name: str
    provider: LLMProvider
    model: str
    reasoning_effort: ReasoningEffort
    instructions: str
    auto_compact: bool
    compact_at_percent: int
    shell_access: bool
    web_search: bool
    structured_output: bool
    version: int
    compaction_prompt: str | Unset = UNSET
    structured_output_schema: AgentSpecStructuredOutputSchemaType0 | None | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        from ..models.agent_spec_structured_output_schema_type_0 import AgentSpecStructuredOutputSchemaType0

        id = str(self.id)

        name = self.name

        provider = self.provider.value

        model = self.model

        reasoning_effort = self.reasoning_effort.value

        instructions = self.instructions

        auto_compact = self.auto_compact

        compact_at_percent = self.compact_at_percent

        shell_access = self.shell_access

        web_search = self.web_search

        structured_output = self.structured_output

        version = self.version

        compaction_prompt = self.compaction_prompt

        structured_output_schema: dict[str, Any] | None | Unset
        if isinstance(self.structured_output_schema, Unset):
            structured_output_schema = UNSET
        elif isinstance(self.structured_output_schema, AgentSpecStructuredOutputSchemaType0):
            structured_output_schema = self.structured_output_schema.to_dict()
        else:
            structured_output_schema = self.structured_output_schema

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "name": name,
                "provider": provider,
                "model": model,
                "reasoning_effort": reasoning_effort,
                "instructions": instructions,
                "auto_compact": auto_compact,
                "compact_at_percent": compact_at_percent,
                "shell_access": shell_access,
                "web_search": web_search,
                "structured_output": structured_output,
                "version": version,
            }
        )
        if compaction_prompt is not UNSET:
            field_dict["compaction_prompt"] = compaction_prompt
        if structured_output_schema is not UNSET:
            field_dict["structured_output_schema"] = structured_output_schema

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.agent_spec_structured_output_schema_type_0 import AgentSpecStructuredOutputSchemaType0

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        name = d.pop("name")

        provider = LLMProvider(d.pop("provider"))

        model = d.pop("model")

        reasoning_effort = ReasoningEffort(d.pop("reasoning_effort"))

        instructions = d.pop("instructions")

        auto_compact = d.pop("auto_compact")

        compact_at_percent = d.pop("compact_at_percent")

        shell_access = d.pop("shell_access")

        web_search = d.pop("web_search")

        structured_output = d.pop("structured_output")

        version = d.pop("version")

        compaction_prompt = d.pop("compaction_prompt", UNSET)

        def _parse_structured_output_schema(data: object) -> AgentSpecStructuredOutputSchemaType0 | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, dict):
                    raise TypeError()
                structured_output_schema_type_0 = AgentSpecStructuredOutputSchemaType0.from_dict(data)

                return structured_output_schema_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(AgentSpecStructuredOutputSchemaType0 | None | Unset, data)

        structured_output_schema = _parse_structured_output_schema(d.pop("structured_output_schema", UNSET))

        agent_spec = cls(
            id=id,
            name=name,
            provider=provider,
            model=model,
            reasoning_effort=reasoning_effort,
            instructions=instructions,
            auto_compact=auto_compact,
            compact_at_percent=compact_at_percent,
            shell_access=shell_access,
            web_search=web_search,
            structured_output=structured_output,
            version=version,
            compaction_prompt=compaction_prompt,
            structured_output_schema=structured_output_schema,
        )

        agent_spec.additional_properties = d
        return agent_spec

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
