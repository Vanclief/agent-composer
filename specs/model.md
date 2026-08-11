# Entity Model

The workflow spec is specified under [dsl/workflow-document.md](dsl/workflow-document.md). The shapes below describe the compiled and persisted execution model derived from that spec.

Shared supporting types used below:

```go
type SnapshotValueMap map[string]any

type ExecutionMetadata map[string]any

type NodeTrace map[string]any

type WorkflowExecutionStatus string

const (
    WorkflowExecutionStatusQueued WorkflowExecutionStatus = "queued"
    WorkflowExecutionStatusRunning WorkflowExecutionStatus = "running"
    WorkflowExecutionStatusSucceeded WorkflowExecutionStatus = "succeeded"
    WorkflowExecutionStatusFailed WorkflowExecutionStatus = "failed"
    WorkflowExecutionStatusCanceled WorkflowExecutionStatus = "canceled"
    WorkflowExecutionStatusBlocked WorkflowExecutionStatus = "blocked"
)

type NodeExecutionStatus string

const (
    NodeExecutionStatusQueued NodeExecutionStatus = "queued"
    NodeExecutionStatusRunning NodeExecutionStatus = "running"
    NodeExecutionStatusSucceeded NodeExecutionStatus = "succeeded"
    NodeExecutionStatusFailed NodeExecutionStatus = "failed"
    NodeExecutionStatusCanceled NodeExecutionStatus = "canceled"
    NodeExecutionStatusBlocked NodeExecutionStatus = "blocked"
)
```

`ConversationStatus` and `TokenUsage` are intentionally reused from the existing codebase. This spec does not redefine them.

## 2.2 Workflow and WorkflowSpec

A workflow is a first-class registry row in the database. Its YAML definition — the spec — is stored on that row and versioned there.

Fields:

```go
type Workflow struct {
    ID uuid.UUID      // permanent identity
    Slug string       // human-facing handle, renameable
    Version int       // current head version
    Spec string       // current spec YAML
    Draft string      // proposed spec awaiting save, "" when none
}

type WorkflowVersion struct {
    ID uuid.UUID
    WorkflowID uuid.UUID
    Version int
    Spec string
}

type WorkflowSpec struct {
    Workflow WorkflowHeader
    Schemas map[string]SchemaSpec
    Nodes map[string]NodeSpec
    Flow FlowSpec
}
```

Notes:

- `WorkflowHeader`, `SchemaSpec`, `NodeSpec`, and `FlowSpec` are authored DSL types reused from the DSL spec
- this document defines the compiled and persisted model and does not redefine the full authored DSL surface
- the registry is the database: every install, edit, or rename of a workflow bumps its integer version and records the full spec in `WorkflowVersion` — history is append-only and survives deletes
- a spec can also be run directly from a YAML file on disk without installing it; such runs record no registry row, only the execution
- a workflow execution records the workflow identity (slug and permanent id), version, and the workflow snapshot that actually ran

## 2.3 WorkflowSnapshot

A WorkflowSnapshot is the compiled execution snapshot derived from one workflow spec.

It is the normalized graph and execution metadata that the runtime actually uses. It may exist only in memory during compilation, but each `WorkflowExecution` must embed a frozen copy of it.

Fields:

```go
type WorkflowSnapshot struct {
    WorkflowSlug string
    WorkflowID string
    WorkflowVersion string
    Description string
    Inputs []WorkflowInputPort
    Outputs []WorkflowOutputPort
    Nodes []NodeSnapshot
    Edges []Edge
    Metadata ExecutionMetadata
}
```

Notes:

- `workflow_slug` is the `workflow.slug`, `workflow_id` is the workflow's permanent identity
- `workflow_version` is the `workflow.version`
- a workflow snapshot is embedded into `WorkflowExecution`
- it is not a standalone DB-backed spec resource
- ordinary workflow composition is flattened into the parent graph before the snapshot is finalized
- loop and conditional nodes remain in the snapshot because they own runtime-dependent nested execution

## 2.4 NodeSnapshot

A NodeSnapshot is a concrete compiled placement of a node inside one workflow snapshot.

Fields:

```go
type NodeSnapshot struct {
    NodeID string
    Kind string
    RuntimeConfig NodeRuntimeConfig
    InputPorts []PortSpec
    OutputPorts []PortSpec
    Metadata ExecutionMetadata
}

type NodeRuntimeConfig struct {
    Inference *InferenceRuntimeConfig
    Connector *ConnectorRuntimeConfig
    Conditional *ConditionalRuntimeConfig
    Loop *LoopRuntimeConfig
}

type InferenceRuntimeConfig struct {
    HarnessID string
    Model string
    ReasoningEffort string
    Instruction string
    HarnessSettings map[string]any
}

type ConnectorRuntimeConfig struct {
    Operation string
}

type ExecutableTargetSnapshot struct {
    TargetKind string
    Node *NodeSnapshot
    Workflow *WorkflowSnapshot
}

type ConditionalRuntimeConfig struct {
    Operation string
    RoutesOn string
    WhenTrueRef string
    WhenFalseRef string
    WhenTrueTarget ExecutableTargetSnapshot
    WhenFalseTarget ExecutableTargetSnapshot
}

type LoopRuntimeConfig struct {
    Operation string
    ExecutesRef string
    ExecutesTarget ExecutableTargetSnapshot
    Over string
    Updates string
    BreaksOn string
    Parallelism string
    PreserveOrder bool
    MaxIterations int
}
```

Notes:

- `node_id` must be unique within the workflow snapshot
- exactly one kind-specific field on `NodeRuntimeConfig` should be populated
- `ExecutableTargetSnapshot` stores the resolved nested runtime target owned by a loop or conditional node
- exactly one of `ExecutableTargetSnapshot.Node` or `ExecutableTargetSnapshot.Workflow` should be populated
- `WhenTrueRef`, `WhenFalseRef`, and `ExecutesRef` preserve the authored target references for traceability
- simple nodes may default to one input port named `in` and one output port named `out`
- nodes that require more complex routing or aggregation may use named ports
- node snapshots originating from a composed workflow should use namespaced node ids derived from the parent instance id
- `WorkflowSnapshot.Nodes` contains only the node snapshots that belong to that workflow scope
- nested workflow or inference targets retained for loop and conditional execution live under the owning composite node runtime config, not as extra top-level entries in `WorkflowSnapshot.Nodes`
- connectors are intended for graph/data plumbing such as collect, concat, pack, and unpack. They are not branching nodes or generic external-operation nodes

## 2.5 Port Specifications

Ports define the typed boundary through which data moves.

Shared shape:

```go
type SchemaSnapshot struct {
    Name string
    CanonicalJSONSchema map[string]any
}

type PortSpec struct {
    Name string
    Required bool
    Schema SchemaSnapshot
    Metadata ExecutionMetadata
}
```

For workflow output ports, the port also owns connection declarations:

```go
type OutputPortSpec struct {
    Name string
    Schema SchemaSnapshot
    Connections []ConnectionTarget
    Metadata ExecutionMetadata
}
```

A connection target is:

```go
type ConnectionTarget struct {
    TargetType string
    TargetNodeID string
    TargetInputPort string
    TargetWorkflowOutput string
}
```

Workflow input ports behave as output endpoints at the graph boundary.

Workflow output ports behave as input endpoints at the graph boundary.

Workflow boundary shapes:

```go
type WorkflowInputPort struct {
    Name string
    Schema SchemaSnapshot
    Connections []ConnectionTarget
    Metadata ExecutionMetadata
}

type WorkflowOutputPort struct {
    Name string
    Required bool
    Schema SchemaSnapshot
    Metadata ExecutionMetadata
}
```

Notes:

- A workflow input port is a source endpoint.
- A workflow output port is a sink endpoint.
- This keeps boundary behavior aligned with the internal graph model.

## 2.6 Compiled Edge

Edges are normalized connection records derived from the authoring model.

They are required in the compiled graph and should be stored in workflow snapshots and embedded execution data.

Shape:

```go
type Edge struct {
    EdgeID string
    From EdgeEndpoint
    To EdgeEndpoint
}

type EdgeEndpoint struct {
    EndpointType string
    WorkflowInput string
    WorkflowOutput string
    NodeID string
    Port string
}
```

Rules:

- Edge is structural only.
- No business logic, routing logic, or transformation logic belongs on an edge.
- Every explicit connection in the authoring model must compile into exactly one edge.
- Every input port may have at most one incoming edge.

## 2.7 WorkflowExecution

A WorkflowExecution is one execution of one workflow spec.

Fields:

```go
type WorkflowExecution struct {
    ID string
    WorkflowSlug string
    WorkflowID uuid.UUID
    WorkflowVersion string
    WorkflowSnapshot WorkflowSnapshot
    InputSnapshot SnapshotValueMap
    OutputSnapshot SnapshotValueMap
    Status WorkflowExecutionStatus
    ProjectDir string
    StartedAt *time.Time
    FinishedAt *time.Time
    Metadata ExecutionMetadata
    CreatedAt time.Time
}
```

Required behavior:

- The execution must reference the source workflow spec identity and version.
- The execution must embed a frozen copy of the workflow snapshot that actually ran.
- The execution stores workflow-level input and output snapshots.
- The execution owns the workflow-level project dir used by inference nodes in that run.
- The execution stores aggregate status and timings.
- The execution is linked to per-node execution records by `workflow_execution_id`.

`blocked` is useful when the execution cannot continue because upstream failures prevent remaining required nodes from becoming executable.

## 2.8 NodeExecution

A NodeExecution is the runtime result of executing one node snapshot during one workflow execution.

Fields:

```go
type NodeExecution struct {
    ID string
    WorkflowExecutionID string
    ParentNodeExecutionID string
    NodeID string
    Kind string
    Status NodeExecutionStatus
    NodeSnapshot NodeSnapshot
    InputSnapshot SnapshotValueMap
    OutputSnapshot SnapshotValueMap
    Trace NodeTrace
    IterationIndex *int
    BranchName string
    StartedAt *time.Time
    FinishedAt *time.Time
    Metadata ExecutionMetadata
    CreatedAt time.Time
}
```

Rules:

- Top-level compiled nodes execute at most once per `WorkflowExecution`, excluding possible future retry attempts.
- Child `NodeExecution` records owned by composite nodes may repeat the same `NodeID` within one `WorkflowExecution`.
- `node_snapshot` is the fully resolved node snapshot that actually executed.
- `trace` is a generic deterministic trace container for execution details.
- `parent_node_execution_id` supports nested runtime scopes for loops and conditionals.
- `iteration_index` may be set for loop-owned child executions.
- `branch_name` may be set for conditional-owned child executions.

Connector nodes, inference nodes, conditional nodes, and loop nodes all produce `NodeExecution` records.

The only difference at this layer is whether a Conversation exists with a matching `node_execution_id`.

Ordinary workflow composition nodes are expanded before execution and therefore do not require their own `NodeExecution` records in this baseline model.

If a loop or conditional node references a workflow node, that workflow node remains a resolved nested runtime target owned by the composite node rather than producing its own top-level `NodeExecution` record.

Loop nodes may execute their referenced target node multiple times. Those repeated child executions may share the same `NodeID` and should be distinguished by `ParentNodeExecutionID` and, when relevant, `IterationIndex`.

Conditional nodes may execute exactly one selected branch target. Child executions selected by branching should be distinguished by `ParentNodeExecutionID` and, when relevant, `BranchName`.

## 2.9 Conversation

Conversation is the currently agreed common trace format for harnesses.

This spec does not constrain the internal message-level schema beyond requiring it to be attachable to a node execution and sufficiently detailed for traceability.

Shape:

```go
type Conversation struct {
    ID string
    NodeExecutionID string
    Harness string
    Model string
    InputSnapshot SnapshotValueMap
    OutputSnapshot SnapshotValueMap
    Messages []ConversationMessage
    HarnessSessionRef string
    HarnessState []byte
    RawHarnessOutput string
    HarnessExitCode int
    HarnessError string
    TokenUsage TokenUsage
    Status ConversationStatus
    Metadata ExecutionMetadata
    CreatedAt time.Time
}

type ConversationMessage struct {
    Role string
    Content string
    Metadata ExecutionMetadata
}
```

Minimum expectation:

- the conversation captures the effective harness and model used at execution time
- it captures the input and output seen by the harness
- it captures the message history or equivalent trace payload used by the harness
- it may also capture provider session references, raw harness output, and token usage for debugging and cost analysis
- it inherits execution context such as project dir from the owning workflow execution rather than storing that context itself

This keeps the current harness logging model intact without forcing DAG-level decisions to depend on inference specifics.
