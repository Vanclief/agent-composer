# Authored DSL Validation

These rules apply to the workflow blueprint before compilation into the internal model.

## Document Rules

- `workflow.id` must be present
- `workflow.version` must be present
- every `nodes.<node_name>` key must be unique
- every `flow.instances.<instance_id>` key must be unique
- every workflow input and output name must be unique within its scope

## Structured Output Rules

- `schemas` is validated according to [structured-outputs.md](structured-outputs.md)
- every `schema_ref` must resolve to a named entry under `schemas`
- cycles through `schema_ref` should be rejected in v1

## Node Rules

- every flow instance must reference an existing node definition
- inference nodes must declare `config.harness`
- inference nodes must declare `config.harness.id`
- inference nodes must declare `config.harness.model`
- connector, conditional, loop, and workflow nodes must not declare `config.harness`
- connector nodes must declare `operation`
- connector `operation` must be one of `collect`, `concat`, `pack`, or `unpack`
- conditional nodes must declare `operation`, `routes_on`, `when_true`, and `when_false`
- conditional `operation` must be `if`
- loop nodes must declare `operation` and `executes`
- loop `operation` must be `foreach` or `while`
- inference nodes must not declare `operation`
- non-conditional nodes must not declare `routes_on`, `when_true`, or `when_false`
- non-loop nodes must not declare `executes`, `over`, `updates`, `breaks_on`, `parallelism`, `preserve_order`, or `max_iterations`
- workflow nodes must declare `workflow_id`
- non-workflow nodes must not declare `workflow_id`
- workflow nodes must not declare `inputs`, `outputs`, `config`, `operation`, `executes`, `over`, `updates`, `breaks_on`, `routes_on`, `when_true`, `when_false`, `parallelism`, `preserve_order`, or `max_iterations`
- every referenced loop `executes` target must resolve to a valid node definition
- every referenced conditional branch target must resolve to a valid node definition

## Connector Rules

- `collect` requires many same-typed inputs and one array output of that element type
- `concat` requires array inputs and one array output with a compatible item schema
- `pack` requires one object output whose property names match the connector input names
- `unpack` requires one object input whose property names match the connector output names

## Conditional Rules

- `routes_on` must name one conditional input
- the conditional input named by `routes_on` must be `boolean`
- `when_true` and `when_false` must resolve to inference nodes or workflow nodes
- every input declared by either branch target must exist on the conditional node with the same name and compatible type
- both branch targets must declare identical outputs by name and compatible types
- the conditional node outputs must match those branch outputs exactly

## Loop Rules

- `foreach` requires `over` to name one loop input
- the `over` input must be array-typed on the loop node
- the executed node must declare an input with the same name as `over`
- the executed node input named by `over` must match the item type of the loop array input
- the executed node must be an inference node or a workflow node
- all other loop inputs must exist on the executed node with the same names and compatible types
- for v1, the executed node must declare one output named `out`
- for v1, the loop node must declare one output named `out`
- the loop node `out` type must be an array whose item type matches the executed node `out` type
- `parallelism` is only valid on loop nodes with `operation: foreach`
- `preserve_order` is only valid on loop nodes with `operation: foreach`
- `while` requires `updates`, `breaks_on`, and `max_iterations`
- the executed node must be an inference node or a workflow node
- `updates` must name one loop input and one loop output
- the executed node must declare one input with the same name as `updates`
- the executed node must declare one output with the same name as `updates`
- the loop input, loop output, executed node input, and executed node output named by `updates` must have compatible types
- the executed node must declare one boolean output named by `breaks_on`
- all other loop inputs must exist on the executed node with the same names and compatible types
- `over`, `parallelism`, and `preserve_order` are invalid on loop nodes with `operation: while`
- for v1, hitting `max_iterations` before `breaks_on` becomes `true` is an execution failure

## Workflow Composition Rules

- `workflow_id` must resolve to exactly one workflow blueprint in the workflow directory
- the resolved child workflow must declare the same `workflow.id` as the referenced `workflow_id`
- workflow node inputs are inferred from the child workflow inputs
- workflow node outputs are inferred from the child workflow outputs
- recursive or cyclic workflow references are forbidden

## Wiring Rules

- every authored input binding must resolve to a valid producer
- every input port may have at most one producer
- every required workflow output must have exactly one producer
- binding compatibility is checked on resolved port schemas, not only on raw authored schema names
- the authored graph must compile into an acyclic graph

## Port Schema Rules

- a port type must be either a built-in primitive keyword or a named entry under `schemas`
- ports must not declare inline `SchemaSpec` values
- `optional` is only valid on entries nested under an object schema's `properties`
- object properties are required by default unless `optional: true` is present
- `additional_properties` is not part of the authored DSL
- authored object schemas are always closed
- array schemas must declare `items`
