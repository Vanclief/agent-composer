# Specs

This directory contains the workflow and execution specifications for Agent Composer.

The spec is split into focused documents instead of a single large README. The DSL lives under [`specs/dsl/`](dsl/README.md). Provider-specific translation is documented only at the boundary where the DSL compiles into canonical JSON Schema.

## Index

- [principles.md](principles.md): core design principles
- [model.md](model.md): normalized internal entity model
- [graph-semantics.md](graph-semantics.md): graph and wiring semantics
- [execution-semantics.md](execution-semantics.md): execution behavior and traceability
- [validation.md](validation.md): spec-level validation rules
- [dsl/README.md](dsl/README.md): workflow spec DSL

## DSL Documents

- [dsl/workflow-document.md](dsl/workflow-document.md): top-level authored document shape
- [dsl/structured-outputs.md](dsl/structured-outputs.md): recursive schemas DSL
- [dsl/nodes.md](dsl/nodes.md): reusable node definitions
- [dsl/flow.md](dsl/flow.md): flow instances and bindings
- [dsl/validation.md](dsl/validation.md): authoring-time validation rules
- [dsl/compilation.md](dsl/compilation.md): compilation pipeline, canonical JSON Schema IR, and provider boundary
