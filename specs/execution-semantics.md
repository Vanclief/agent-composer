# Execution Semantics

## 4.1 Scheduling Model

Execution is cascade-based.

A scheduler may compute a topological ordering or equivalent dependency plan over the compiled DAG.

Composite nodes such as loops and conditionals still appear as single nodes in that parent DAG, even when they own nested runtime scopes internally.

A node becomes eligible to run when:

- all required input ports have exactly one resolved value
- all upstream dependencies that could affect it have reached terminal states required by the workflow's failure policy

The scheduler may track readiness internally, but persisted execution status does not need a separate `ready` state.

A queued node may transition to running, then to a terminal status.

## 4.2 Terminal Statuses

At minimum, terminal status values should include:

- succeeded
- failed
- canceled
- blocked

`blocked` is useful when a node or execution can no longer proceed because a required upstream path failed or never produced a required value.

## 4.3 Resolved Workflow Data

At execution start, the workflow engine must reference a specific workflow blueprint identity and version, and embed the workflow snapshot into the `WorkflowExecution`.

That embedded workflow snapshot should include at least:

- workflow blueprint identity and version
- node snapshots
- resolved port definitions
- compiled edges
- resolved static configuration

This makes the execution auditable even if blueprint drafts later change.

## 4.4 Traceability Requirements

For every workflow execution, the system must record:

- workflow input snapshot
- workflow output snapshot
- workflow shell root
- workflow status and timings
- node execution records
- the exact workflow blueprint identity, version, and workflow snapshot that were executed

For every node execution, the system must record:

- node identity
- node kind
- resolved runtime config
- input snapshot
- output snapshot
- status and timings
- generic trace
- optional related conversation, linked by `node_execution_id`

For composite node executions such as loops and conditionals, `trace` should be able to capture nested execution details such as iterations, selected branches, and nested workflow targets.

This satisfies the current traceability goal without introducing a separate `WorkflowSnapshot` resource.
