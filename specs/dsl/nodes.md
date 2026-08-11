# Node Definitions DSL

Reusable node definitions are authored under `nodes`.

Common shape:

```yaml
nodes:
  <node_name>:
    kind: inference | connector | conditional | loop | workflow
    inputs: object?
    outputs: object?
```

Common rules:

- `<node_name>` defines a reusable local node definition
- `kind` identifies the node behavior category
- `inputs` and `outputs` define the authored port shape for that node
- each node kind adds its own required fields and config shape

## Port Shape

Authoring shape:

```yaml
PortTypeRef:
  string | integer | number | boolean | null | <schema_name>
```

Rules:

- port definitions do not author schemas inline
- a port type is either a built-in primitive keyword or a named reusable schema under `schemas`
- object, array, and union shapes must be declared under [structured-outputs.md](structured-outputs.md) and then referenced by name

Examples:

```yaml
nodes:
  text_summarizer:
    kind: inference
    inputs:
      article: string
    outputs:
      out: SummaryDraft
    config:
      harness:
        id: codex_cli
        model: gpt-5.4-mini
        reasoning_effort: medium
```

## Node Kinds

### Inference

Inference nodes produce outputs through an LLM or AI harness.

Authoring shape:

```yaml
nodes:
  <node_name>:
    kind: inference
    inputs: object?
    outputs: object?
    config:
      harness:
        id: string
        model: string
        reasoning_effort: string?
        # additional harness-specific settings are allowed here
      # task-specific settings such as instruction live here too
```

Rules:

- inference nodes must declare `config.harness`
- `config.harness.id` selects the harness, for example `codex_cli` or `claude_code`
- `config.harness.model` selects the model used by that harness
- `config.harness.reasoning_effort` is optional
- `config.harness.permissions` sets the access tier and is the same for every harness: `read_only` (default), `exec`, or `dangerously-exec`. `read_only` may only read the workspace; `exec` may modify it and run shell commands scoped to it; `dangerously-exec` removes all guardrails (network, writes anywhere). Each harness translates the tier into its own backend flags.
- additional keys under `config.harness` are harness-specific settings (for example `profile`/`config_overrides` for `codex_cli`, or `allowed_tools`/`mcp_config` for `claude_code`)
- task-specific settings such as `instruction` remain as sibling fields under `config`

### Connector

Connector nodes perform deterministic graph or data plumbing such as collecting, concatenating, packing, and unpacking values.

Authoring shape:

```yaml
nodes:
  <node_name>:
    kind: connector
    operation: collect | concat | pack | unpack
    inputs: object?
    outputs: object?
```

Connector nodes must select a built-in `operation`.

Supported connector operations:

- `collect`: many same-typed inputs become one array output. Output order is input port declaration order.
- `concat`: many array inputs become one array output. Output order is input port declaration order, then original item order within each input array.
- `pack`: many named inputs become one object output. Input port names must match properties on the output object schema.
- `unpack`: one object input becomes many named outputs. Output port names must match properties on the input object schema.

Notes:

- connector operations are a closed built-in set, not arbitrary custom functions
- plain fan-out does not need a connector
- array item expansion belongs to loop nodes with `operation: foreach`, not to connectors

Example:

```yaml
vote_collector:
  kind: connector
  operation: collect
  inputs:
    vote_a: Vote
    vote_b: Vote
  outputs:
    out: VoteList
```

### Conditional

Conditional nodes perform runtime branch selection over one boolean input and one of two referenced branch targets.

Authoring shape:

```yaml
nodes:
  <node_name>:
    kind: conditional
    operation: if
    routes_on: string
    when_true: string
    when_false: string
    inputs: object?
    outputs: object?
```

Conditional-specific fields:

- `operation`
- `routes_on`
- `when_true`
- `when_false`

Rules:

- conditional nodes must declare `operation`, `routes_on`, `when_true`, and `when_false`
- `operation` is `if` in v1
- `routes_on` selects the boolean input that drives branch selection
- `when_true` and `when_false` may reference an inference node or a workflow node
- both branch targets must expose the same outputs by name and compatible type
- the conditional node outputs must match those branch outputs exactly
- each branch target may declare only inputs that also exist on the conditional node, matched by name and compatible type
- at runtime, only the selected branch executes
- conditional nodes are runtime composite nodes. They do not compile away into the parent graph

Example:

```yaml
review_router:
  kind: conditional
  operation: if
  routes_on: agrees
  when_true: text_summarizer
  when_false: disagreement_explainer
  inputs:
    text: string
    agrees: boolean
  outputs:
    out: ReviewOutcome
```

### Loop

Loop nodes are composite nodes that repeatedly execute another node in a nested execution scope.

Supported loop operations are `foreach` and `while`.

Authoring shape:

```yaml
# foreach
nodes:
  <node_name>:
    kind: loop
    operation: foreach
    executes: string
    over: string
    inputs: object?
    outputs: object?
    parallelism: max | serial?
    preserve_order: boolean?

# while
nodes:
  <node_name>:
    kind: loop
    operation: while
    executes: string
    updates: string
    breaks_on: string
    inputs: object?
    outputs: object?
    max_iterations: integer
```

Loop-specific fields:

- `operation`
- `executes`
- `over`
- `updates`
- `breaks_on`
- `parallelism`
- `preserve_order`
- `max_iterations`

Rules:

- loop nodes must declare `operation` and `executes`
- `executes` may reference an inference node or a workflow node
- loop node inputs must match the executed node inputs by name unless operation-specific rules say otherwise

`foreach` rules:

- `operation: foreach` iterates over one array input on the loop node
- `over` is required for `foreach`
- `over` selects which loop input is the iterated array
- the `over` input must be array-typed on the loop node and item-typed on the executed node
- all non-iterated inputs are passed through unchanged to every iteration
- for v1, the executed node must declare one output named `out`
- for v1, the loop node must declare one output named `out`, typed as an array of the executed node `out` type
- `parallelism` is optional and applies only to `foreach`
- `preserve_order` is optional and defaults to `true`

`while` rules:

- `operation: while` repeatedly executes the target node until a named boolean output says the loop should stop
- `updates`, `breaks_on`, and `max_iterations` are required for `while`
- `updates` selects the evolving input and output carried across iterations
- the loop node must declare one input named by `updates`
- the loop node must declare one output with the same name as `updates`
- the executed node must declare one input named by `updates`
- the executed node must declare one output with the same name as `updates`
- the executed node must declare one boolean output named by `breaks_on`
- on the first iteration, the executed node receives the loop input named by `updates`
- on later iterations, the executed node receives the previous iteration's output named by `updates`
- all other loop inputs are passed through unchanged to every iteration
- for v1, `while` runs sequentially
- reaching `max_iterations` before `breaks_on` becomes `true` stops the loop gracefully and returns the most recent `updates` state, rather than failing the execution

Example:

```yaml
issue_validation_worker:
  kind: loop
  operation: foreach
  executes: issue_validator
  over: issue
  inputs:
    issue: IssueList
  outputs:
    out: ValidatedIssueList
  parallelism: max
  preserve_order: true

consensus_loop:
  kind: loop
  operation: while
  executes: vote_consensus_step
  updates: vote_state
  breaks_on: should_stop
  inputs:
    question: string
    vote_state: VoteState
  outputs:
    vote_state: VoteState
  max_iterations: 10
```

### Workflow

Workflow nodes compose one workflow inside another.

Authoring shape:

```yaml
nodes:
  <node_name>:
    kind: workflow
    workflow_slug: string
```

Rules:

- workflow nodes must declare `workflow_slug`
- `workflow_slug` references another workflow by its `workflow.slug`
- workflow nodes infer their input ports from the referenced workflow inputs
- workflow nodes infer their output ports from the referenced workflow outputs
- workflow nodes must not declare `inputs`, `outputs`, `config`, `operation`, `executes`, `over`, `updates`, `breaks_on`, `routes_on`, `when_true`, `when_false`, `parallelism`, `preserve_order`, or `max_iterations`
- bindings to workflow node outputs use the child workflow output names, for example `instance.summarize.final_summary`
- recursive workflow references are forbidden
- workflow nodes are an authored composition convenience
- when used as ordinary flow instances, the compiler expands workflow nodes into the parent graph before execution
- when referenced by a loop or conditional composite node, the compiler resolves the child workflow as a nested runtime target instead of flattening it into the parent graph

Example:

```yaml
article_summary_pipeline:
  kind: workflow
  workflow_slug: article_summary
```
