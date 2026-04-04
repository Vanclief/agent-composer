# Workflow Blueprint DSL

The workflow blueprint should behave more like a Docker Compose file than like a dump of the internal database model.

It is the primary authoring surface a user edits.

It should:

- declare workflow identity, inputs, and outputs in a compact form
- define reusable local node definitions
- allow composing workflows by id
- express structured control flow through loops and conditionals
- define concrete flow instances that compile into `NodeSnapshot` definitions
- express wiring declaratively
- omit normalized internal structures such as compiled edges

## Top-Level Shape

Top-level shape:

```yaml
workflow:
  id: string
  version: string
  description: string?
  inputs: object
  outputs: object

schemas: object?
nodes: object
flow:
  instances: object
```

Where:

- `workflow` declares identity and boundary ports
- `schemas` defines reusable schema fragments
- `nodes` defines reusable local node definitions, including workflow composition nodes
- `flow.instances` places concrete node instances into the workflow

## Workflow Block

Shape:

```yaml
workflow:
  id: string
  version: string
  description: string?
  inputs:
    <input_name>: PortTypeRef
  outputs:
    <output_name>:
      schema: PortTypeRef
      from: string
```

Rules:

- each workflow input declares a boundary port type
- each workflow output declares a boundary port type
- `workflow.id` is the stable identifier for the workflow
- `from` binds the workflow output to a producer such as `instance.some_node.out`
- `PortTypeRef` is either a built-in primitive keyword or a named reusable schema under `schemas`
- complex object, array, and union shapes must be authored under `schemas`

`SchemaSpec` is defined in [structured-outputs.md](structured-outputs.md). `PortTypeRef` consumes those named schemas but does not define them inline.

## Reusable Local Definitions

- `schemas` is defined in [structured-outputs.md](structured-outputs.md)
- `nodes` is defined in [nodes.md](nodes.md)
- `flow.instances` is defined in [flow.md](flow.md)

## Composition

A node definition may compose another workflow by declaring `kind: workflow` and `workflow_id`.

The compiler resolves `workflow_id` from the workflow directory and uses the child workflow blueprint boundary as the composed node boundary. When the workflow node is used as an ordinary flow instance, the compiler expands the child workflow into the parent graph during compilation. When a loop or conditional node references that workflow node, the child workflow is retained as a nested runtime target owned by the composite node.

## Example

Workflow blueprint examples live under [examples/](examples/README.md).

- [examples/composition-article-summary-with-brief.yaml](examples/composition-article-summary-with-brief.yaml): compose one workflow and continue from its named outputs
- [examples/composition-loop-iterative-code-repair.yaml](examples/composition-loop-iterative-code-repair.yaml): seed an implementation and then run a composed `while` review and repair loop
- [examples/conditional-boolean-routing-review.yaml](examples/conditional-boolean-routing-review.yaml): route to one of two branch nodes based on a boolean output from an upstream inference node
- [examples/connector-collect-binary-votes.yaml](examples/connector-collect-binary-votes.yaml): connector `collect` example with raw vote collection
- [examples/loop-and-connector-parallel-code-review.yaml](examples/loop-and-connector-parallel-code-review.yaml): larger workflow with `foreach` loops and `concat` aggregation
- [examples/loop-foreach-section-summary.yaml](examples/loop-foreach-section-summary.yaml): simple `foreach` loop with one iterated input and one pass-through input
- [examples/loop-while-binary-consensus.yaml](examples/loop-while-binary-consensus.yaml): `while` loop with one evolving state value and one boolean break output
- [examples/pipeline-parallel-review-fix-cycle.yaml](examples/pipeline-parallel-review-fix-cycle.yaml): one repair cycle with parallel reviewers, issue validation, and question triage
- [examples/pipeline-summary-critique-revise.yaml](examples/pipeline-summary-critique-revise.yaml): summarize, critique, and revise workflow
