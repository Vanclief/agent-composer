# Workflow DSL

This directory defines the workflow spec DSL.

The DSL is the user-facing configuration format for reusable workflows. It is intentionally compact and declarative. It is not a 1:1 serialization of the internal execution model.

Provider-specific payloads do not belong in this DSL. The DSL compiles into internal workflow entities and into canonical JSON Schema, and provider-specific translation happens only after that boundary.

## Documents

- [workflow-document.md](workflow-document.md): top-level authored document shape
- [structured-outputs.md](structured-outputs.md): reusable output schema language
- [nodes.md](nodes.md): reusable local node definitions
- [flow.md](flow.md): flow instances and authored bindings
- [validation.md](validation.md): authoring-time validation rules
- [compilation.md](compilation.md): how authored DSL compiles into the internal model and canonical JSON Schema
- [examples/README.md](../../examples/README.md): concrete workflow spec examples
