# Schemas DSL

Reusable schemas are authored under `schemas`.

This is an authoring DSL, not the final provider-facing payload. The compiler resolves `schema_ref`, validates the authored graph, and emits canonical JSON Schema before any provider-specific translation runs.

## Shape

Authored shape:

```yaml
schemas:
  <type_name>: SchemaSpec
```

`SchemaSpec` is recursive:

```yaml
SchemaSpec:
  # Exactly one of `schema_ref`, `type`, `any_of`, `one_of` must be set.
  schema_ref: string?
  type: object | array | string | integer | number | boolean | null

  description: string?
  nullable: boolean?
  enum: any[]?

  properties:
    <field_name>: PropertySpec

  items: SchemaSpec?

  any_of: SchemaSpec[]?
  one_of: SchemaSpec[]?

PropertySpec:
  optional: boolean?
  schema_ref: string?
  type: object | array | string | integer | number | boolean | null

  description: string?
  nullable: boolean?
  enum: any[]?

  properties:
    <field_name>: PropertySpec

  items: SchemaSpec?

  any_of: SchemaSpec[]?
  one_of: SchemaSpec[]?
```

Recursion is explicit:

- `properties.<field_name>` is a `PropertySpec`
- `items` is another `SchemaSpec`
- `any_of` contains `SchemaSpec[]`
- `one_of` contains `SchemaSpec[]`
- `schema_ref` points to another named entry under `schemas`

## Rules

- Exactly one of `schema_ref`, `type`, `any_of`, `one_of` must be present.
- If `type: object`, `properties` is allowed.
- Each object property is required by default.
- A property may declare `optional: true` to allow omission.
- `optional` is authoring sugar on `PropertySpec`, not part of the canonical JSON Schema IR.
- Authored object schemas are always closed. The authored DSL does not expose an `additional_properties` escape hatch.
- If `type: array`, `items` is required.
- `schema_ref` must point to another named entry under `schemas`.
- `nullable: true` is authoring sugar only. The compiler lowers it to a union with `null`.
- `enum` should be limited to scalar values in v1 for portability.
- Cycles through `schema_ref` should be rejected in v1.
- Arbitrary map-like object schemas are out of scope in v1.

## Why `schema_ref` Exists

`schema_ref` is an authoring convenience.

Authored DSL:

```yaml
schemas:
  Issue:
    type: object
    properties:
      path:
        type: string
      line:
        type: integer

  IssueList:
    type: array
    items:
      schema_ref: Issue
```

The compiler resolves `schema_ref` before talking to any provider. It also converts authored property presence rules into the parent object's JSON Schema `required` array. The provider receives canonical JSON Schema, not the DSL form above.

That canonical schema may be emitted with `$defs` and `$ref`, or with references inlined, depending on runtime translation requirements.

## Example

```yaml
schemas:
  Issue:
    type: object
    description: A single review issue.
    properties:
      path:
        type: string
      line:
        type: integer
      title:
        type: string
      description:
        type: string

  ValidatedIssue:
    type: object
    properties:
      issue:
        schema_ref: Issue
      is_valid:
        type: boolean
      reason:
        type: string
        nullable: true

  ValidatedIssueList:
    type: array
    items:
      schema_ref: ValidatedIssue
```

## Provider Boundary

Provider wrappers stay out of this DSL.

The pipeline should be:

1. parse the authored DSL
2. validate `SchemaSpec`
3. resolve `schema_ref`
4. emit canonical JSON Schema
5. let the runtime translate that canonical schema into an OpenAI, Codex, Claude, or other provider-specific request shape if needed

See:

- [compilation.md](compilation.md)
