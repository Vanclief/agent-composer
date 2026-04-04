# Core Design Principles

The system is based on the following agreed rules.

## 1.1 Authored Workflows Are DAGs

A workflow blueprint defines an acyclic directed graph.

Cycles are not allowed in the authored graph.

Any repeated behavior must be represented without introducing cycles into the authored workflow topology.

The system supports composite control flow only when it is modeled as hierarchical execution over acyclic scopes:

- the parent workflow scope is a DAG
- the loop target is either an inference-node target or a workflow target whose internal graph is a DAG
- a conditional branch target is either an inference-node target or a workflow target whose internal graph is a DAG
- repetition and branch selection exist only at runtime, where the engine executes nested composite targets

This means the system does not support "DAGs with cycles." It supports hierarchical DAG execution through composite nodes.

### Iteration Model

A loop-capable composite node must satisfy all of the following:

- it appears as a single node in the parent DAG
- it references one executed target that is either an inference node or a workflow
- it creates one or more child `NodeExecution` records at runtime
- in `foreach` mode, child `NodeExecution` records may run in parallel subject to node configuration
- it waits for each child `NodeExecution` to reach a terminal state before deciding whether to continue
- it emits a final output back into the parent DAG when iteration is complete

If the executed target is a composed workflow, cycle detection applies independently within each workflow scope. The parent workflow must contain no back-edge, and the child workflow must contain no back-edge. Repetition is represented by repeated invocation of the loop target, not by an authored edge that returns to an earlier node.

### Scope Boundaries

The following rules apply to nested workflow scopes:

- if the executed target is a workflow, the child workflow may communicate with the parent workflow only through the composite node boundary
- cross-scope edges are not allowed
- each iteration must have explicit inputs and explicit outputs
- for `while` mode, iteration `n+1` receives the value named by `updates` from iteration `n`
- for `foreach` mode, each iteration receives one item plus any explicitly bound shared context
- the composite node owns termination; the executed target never jumps back to an earlier point in the parent workflow

### Conditional Model

A conditional-capable composite node must satisfy all of the following:

- it appears as a single node in the parent DAG
- it references exactly two branch targets, each of which is either an inference node or a workflow
- it reads one boolean routing input from its boundary
- it executes only the selected branch target as one or more child `NodeExecution` records in a nested execution scope
- it emits the selected branch outputs back into the parent DAG

If a branch target is a composed workflow, cycle detection applies independently within each workflow scope. The parent workflow must contain no back-edge, and the child workflow must contain no back-edge. Branching is represented by runtime branch selection, not by authored alternative edge paths that rejoin through hidden control flow.

### Composite Runtime Shape

The recommended runtime model is semantic containment under composite node executions:

- `WorkflowExecution` is the versioned executable workflow record
- a loop node produces one parent `NodeExecution`
- each loop iteration produces one or more child `NodeExecution` records whose `ParentNodeExecutionID` points to that parent loop execution
- if the loop target is an inference node, repeated iterations may produce multiple child `NodeExecution` records with the same `NodeID`
- `IterationIndex` identifies loop-owned child executions
- a conditional node produces one parent `NodeExecution`
- the selected branch produces one or more child `NodeExecution` records whose `ParentNodeExecutionID` points to that parent conditional execution
- `BranchName` identifies conditional-owned child executions

Nested child executions should not be treated as independent top-level workflow executions unless that independence is an explicit operational requirement.

An illustrative runtime tree is:

```text
WorkflowExecution(parent)
  NodeExecution(A)
  NodeExecution(B)
  NodeExecution(LoopNode)
    NodeExecution(Target, IterationIndex=0)
    NodeExecution(Target, IterationIndex=1)
    NodeExecution(Target, IterationIndex=2)
  NodeExecution(ConditionalNode)
    NodeExecution(Target, BranchName=when_true)
  NodeExecution(C)
```

### Required Loop Controls

Any loop-capable composite node should define and enforce, at minimum:

- `operation` (`while` or `foreach`)
- `executes`
- `over` for `foreach`
- `updates` for `while`
- `breaks_on` for `while`
- `max_iterations`
- `parallelism` for `foreach`
- `preserve_order` for `foreach`

The governing rule is:

> Every workflow scope is a DAG. Looping is implemented by a composite node that repeatedly executes a node target through child `NodeExecution` records in nested execution scopes.

### Required Conditional Controls

Any conditional-capable composite node should define and enforce, at minimum:

- `operation` (`if`)
- `routes_on`
- `when_true`
- `when_false`

The governing rule is:

> Every workflow scope is a DAG. Branching is implemented by a composite node that chooses one target through child `NodeExecution` records in a nested execution scope.

## 1.2 Execution Is Cascade-Based

Execution is data-driven and one-shot.

There is no separate control-flow layer. A node becomes eligible to execute when its required inputs are available and its upstream dependencies have completed according to the workflow's execution rules.

Each top-level node snapshot in the compiled parent workflow executes at most once per workflow execution, excluding retries if retries are later introduced.

Child node executions owned by composite nodes may repeat the same `NodeID` within one workflow execution.

Each edge carries at most one value per workflow execution.

Each output port emits at most one value per workflow execution.

## 1.3 One Producer Per Input Port

An input port may have at most one incoming connection.

Fan-in must be made explicit through connector nodes or other future composite nodes. A downstream node must never receive two producers directly into the same input port.

## 1.4 Wiring Is Authored Declaratively, Normalized to Edges

In the authoring document, data flow is declared declaratively and compiled into explicit edges.

In the compiled/runtime model, all connections are normalized into explicit edge records.

Edges are structural only. They do not contain behavior.

## 1.5 Traceability Is First-Class

Every workflow execution records the exact workflow blueprint identity, version, and an embedded frozen copy of the workflow snapshot that actually executed.

Every node execution records input, output, status, timings, trace, and the node snapshot that actually executed.

Inference nodes may additionally attach a Conversation, which is the current common trace format used by harnesses.

## 1.6 No Separate WorkflowSnapshot Resource

There is no standalone WorkflowSnapshot resource.

A WorkflowExecution must reference a specific `workflow_id` and `workflow_version`, and must embed the workflow snapshot that actually executed.
