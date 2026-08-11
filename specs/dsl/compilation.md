# DSL Compilation

The workflow DSL compiles into two different targets:

- the internal normalized workflow model
- canonical JSON Schema for structured outputs

Provider-specific request wrappers are applied only after those compilation steps.

## Compilation Pipeline

1. Parse the workflow spec.
2. Resolve any `workflow_slug` references used by workflow nodes.
3. Validate authored `schemas`, node ports, flow instances, and bindings.
4. Resolve port type references from built-in primitives, named entries under `schemas`, and referenced workflow boundaries.
5. Resolve `schema_ref` within `schemas`.
6. Lower authored `SchemaSpec` values into canonical JSON Schema, including converting property presence rules into parent object `required` arrays.
7. Expand ordinary workflow nodes into namespaced child nodes and child edges.
8. Preserve loop and conditional nodes as runtime composite nodes and resolve their referenced targets.
9. Resolve node definitions and flow instances into normalized `NodeSnapshot` records.
10. Normalize authored bindings into explicit `Edge` records.
11. Validate single-producer, acyclicity, and compatibility rules.
12. Emit the internal model used by execution.
13. Apply provider-specific translation only if a runtime needs a provider-specific structured output request shape.

## Internal Targets

The authored DSL should compile into internal structures such as:

- `WorkflowSnapshot`
- explicit workflow input and output ports
- `NodeSnapshot`
- normalized `Edge` records

See [../model.md](../model.md).

Port definitions are not mini schema-definition sites. A port uses either:

- a built-in primitive keyword such as `string` or `boolean`
- a named schema from `schemas`

If a port references a named schema, the compiler resolves that name before emitting the internal model or canonical JSON Schema.

## Workflow Composition

Ordinary workflow composition is compile-time only.

A workflow node references another workflow by `workflow_slug`. The compiler should:

1. load the child workflow spec
   Child workflows resolve from the registry database first, then from YAML files next to the parent spec's source file when the parent was loaded from disk.
2. validate the child workflow as a normal workflow
3. infer the workflow node input and output ports from the child workflow boundary
4. namespace the child node ids under the parent instance id
5. rewrite child workflow inputs and outputs to the parent workflow bindings
6. emit one flat normalized graph for execution

Port compatibility across workflow boundaries is checked on resolved schemas, not on raw authored schema names. That allows a composed workflow output and a parent-local named schema to connect as long as they resolve to compatible structures.

If a workflow node is referenced by a loop or conditional composite node, the compiler should still resolve the child workflow and validate its boundary, but it should retain it as a nested runtime target owned by that composite node instead of flattening it into the parent graph.

That nested target should be embedded into the owning composite node runtime config in the compiled model, not left to be re-resolved from disk during execution. Top-level `WorkflowSnapshot.Nodes` should contain only the nodes that belong to that workflow scope.

## Loop Compilation And Execution

Loop nodes are not expanded into the parent graph at compile time.

Unlike workflow composition, loop iteration counts are runtime-dependent, so the normalized graph should retain the loop node as a runtime composite node.

Compilation must resolve the target named by `executes` and store the resulting nested target snapshot under the owning loop node runtime config. Runtime execution should use that embedded target snapshot directly.

`foreach` execution model:

1. resolve the executed node definition
2. read the array input named by `over`
3. for each item, execute the target node once using that item plus any pass-through inputs
4. collect each iteration's `out`
5. emit the collected array as the loop node `out`

If the executed target is a workflow node, each iteration executes the resolved child workflow in a nested runtime scope.

`while` execution model:

1. resolve the executed node definition
2. initialize the evolving value from the loop input named by `updates`
3. execute the target node with that evolving value plus any pass-through inputs
4. read the next evolving value from the executed node output named by `updates`
5. read the stop flag from the executed node output named by `breaks_on`
6. if the stop flag is `true`, emit the evolving value as the loop output named by `updates`
7. otherwise repeat until `max_iterations` is reached

If the executed target is a workflow node, each iteration executes the resolved child workflow in a nested runtime scope.

Loop compatibility is checked on resolved port schemas, not on raw authored schema names.

## Conditional Compilation And Execution

Conditional nodes are not expanded into the parent graph at compile time.

Like loop nodes, branch selection is runtime-dependent, so the normalized graph should retain the conditional node as a runtime composite node.

Compilation must resolve the targets named by `when_true` and `when_false` and store those nested target snapshots under the owning conditional node runtime config. Runtime execution should use those embedded target snapshots directly.

`if` execution model:

1. resolve the `when_true` and `when_false` target node definitions
2. read the boolean input named by `routes_on`
3. select either `when_true` or `when_false`
4. construct the selected target input set from the conditional node inputs whose names are declared by the selected target
5. execute the selected target once
6. emit the selected target outputs as the conditional node outputs

If the selected target is a workflow node, the conditional node executes the resolved child workflow in a nested runtime scope.

Conditional compatibility is checked on resolved port schemas, not on raw authored schema names.

## Structured Output Lowering

`schema_ref` is an authored convenience, not a provider payload format.

The compiler should resolve it before provider-specific translation. That means downstream runtimes receive:

- canonical JSON Schema with `$defs` and `$ref`
- or canonical JSON Schema with references inlined

The provider should not receive the raw DSL form containing `schema_ref`.

## Canonical JSON Schema IR

Canonical JSON Schema is the intermediate representation between:

- the authored workflow DSL
- any provider-specific structured output request shape

This IR should:

- resolve `schema_ref`
- convert authored property presence rules into parent object `required` arrays
- lower `nullable: true` into a union with `null`
- preserve `required`
- preserve `enum`
- preserve `description`
- emit `additionalProperties: false` for authored object schemas

The authored DSL is optimized for ergonomics. The canonical JSON Schema IR is optimized for compatibility with JSON Schema-based provider APIs.

The compiler may emit canonical JSON Schema using:

- `$defs` and `$ref`
- or inline expansion

That choice belongs to the compilation and runtime boundary, not to the authored DSL.

## Provider Boundary

The DSL must stay provider-neutral.

Provider-specific request wrappers do not belong in the DSL itself.

The only contract the DSL compiler should expose here is canonical JSON Schema.

Any runtime-specific translation should happen after compilation and should be documented only when concrete provider differences appear, for example:

- whether the provider accepts `$defs` and `$ref` directly or prefers inlined schemas
- whether closed-object handling via `additionalProperties: false` needs provider-specific adjustments
- whether unions such as `one_of` or `any_of` need narrowing
- whether the provider expects the schema to be wrapped in a tool definition, a response-format object, or another transport-specific envelope
