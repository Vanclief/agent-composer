# Agent Composer 0.2 Plan

This document is a planning artifact for the `0.2` workflow-based redesign.

It captures the agreed direction and the intended migration path. It does not
imply that any of these changes have already been implemented.

## Goal

Re-center Agent Composer around workflow execution.

The source of truth becomes workflow blueprint YAML files on disk. The database
stores only execution history and inference debugging artifacts.

`0.2` is allowed to introduce breaking changes.

## Core Decisions

1. Workflows and node definitions are file-backed, not DB-backed.

- The source of truth for workflow blueprints is YAML in the workflow directory on disk.
- Workflow composition resolves by `workflow.id`.
- Node definitions live inside workflow files and are not stored independently.

2. Agent specs are removed.

- `agent_specs` is not retained as a compatibility layer.
- The primary product entrypoint becomes "execute workflow", not "create conversation from spec".

3. The DB stores executions, not blueprint definitions.

- Keep workflow execution records.
- Keep node execution records.
- Keep conversations as inference debugging artifacts.
- Do not store workflow blueprints, node blueprints, or workflow directory state in the DB.

4. Every workflow execution stores a frozen compiled snapshot.

- This is required for auditability and reproducibility.
- Historical executions must not depend on the current YAML contents on disk.

5. Conversations belong to inference node executions.

- Conversations are attached to `node_execution_id`, not `agent_spec_id`.
- A conversation is a debug/runtime artifact of a specific inference execution.

## Filesystem Model

Recommended layout:

- workflow directory: `$AGENT_COMPOSER_HOME/workflows/`
- starter workflows: shipped with the installation and copied into the workflow directory during setup or first-run bootstrap

Recommended loader behavior:

- scan the workflow directory for `*.yaml` and `*.yml`
- bootstrap starter workflows by copying them into the workflow directory only when the destination file does not already exist
- index by `workflow.id`
- reject duplicate installed `workflow.id`
- for `0.2`, support exactly one installed version per `workflow.id`
- require `workflow.version` to be increased manually

Notes:

- Do not use the binary directory itself as the writable workflow directory.
- A user home directory is safer for packaged installs and containers.
- After bootstrap, runtime loading reads only from the workflow directory.
- Starter workflows are just initial files. Once copied, they are fully user-owned.

## In-Memory Architecture

The implementation should distinguish three layers:

1. `WorkflowBlueprint`

- loaded from DSL YAML
- close to the user-written shape

2. `WorkflowSnapshot`

- validated
- normalized
- schema refs resolved
- workflow composition resolved
- loops and conditionals preserved as composite runtime nodes
- top-level node lists contain only the nodes in that workflow scope

3. `NodeSnapshot`

- execution-ready configuration for one node
- especially important for inference nodes
- loop and conditional node snapshots own their resolved nested runtime targets inline
- those nested runtime targets are either inference targets or workflow targets
- runtime executes composite targets from the embedded snapshot data, not by re-resolving from disk

Recommended naming:

- use `WorkflowBlueprint` for the file-backed definition
- use `WorkflowSnapshot` for the compiled execution snapshot
- use `NodeSnapshot` for the compiled node snapshot used at execution time
- avoid DB-backed blueprint models

## Database Model

Suggested supporting types:

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

### `workflow_executions`

Suggested Go shape:

```go
type WorkflowExecution struct {
    ID string
    WorkflowID string
    WorkflowVersion string
    WorkflowSnapshot WorkflowSnapshot
    InputSnapshot SnapshotValueMap
    OutputSnapshot SnapshotValueMap
    Status WorkflowExecutionStatus
    ShellRoot string
    Metadata ExecutionMetadata
    StartedAt *time.Time
    FinishedAt *time.Time
    CreatedAt time.Time
}
```

Purpose:

- record what workflow ran
- record the exact compiled workflow that ran
- record workflow-level input/output/state
- keep the application schema typed in Go even if the backing DB persists it as a serialized payload

### `node_executions`

Suggested Go shape:

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
    CreatedAt time.Time
}
```

Purpose:

- record every node that actually ran
- support nested runtime scopes for loops and conditionals
- support traceability without storing flow-instance placements as first-class rows

Notes:

- keep `trace` loosely typed JSON in `0.2`

### `conversations`

Keep the table, but change its ownership model.

Main changes:

- remove `agent_spec_id`
- add `node_execution_id`

Suggested Go shape:

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
```

Purpose:

- debug a specific inference execution
- preserve harness transcripts and runtime details

Rules:

- do not allow starting a conversation directly
- every new conversation must come from a workflow-owned inference node execution
- `ConversationStatus` and `TokenUsage` intentionally reuse the existing codebase types

## What Is Removed

Remove:

- `agent_specs` table and related model
- agent spec CRUD API
- agent-spec-driven conversation creation
- TUI sections that assume specs are the primary user-facing object

Do not add:

- `workflow_blueprints` table
- `node_blueprints` table
- `node_instances` table
- workflow registry tables

## Runtime Direction

Current runtime flow:

- fetch DB `agent.Spec`
- create `Conversation`
- run harness

Target runtime flow:

- load workflow blueprint from disk
- compile it into a workflow snapshot
- create `workflow_execution`
- create `node_execution` records as nodes start
- for inference nodes, build `Conversation` directly from the node snapshot and the owning workflow execution context
- run harness
- attach conversation to the owning `node_execution`

Recommended runtime change:

- remove `NewConversationInstanceFromSpec`
- replace it with a node-execution-based entrypoint, for example:
  - `NewConversationInstanceFromNodeExecution`
  - or `StartInferenceNodeExecution`

## API Direction

Remove:

- `/api/agents/specs`
- create/update/delete/get/list agent spec endpoints

Add a workflow-first surface:

- `GET /api/workflows`
- `GET /api/workflows/{id}`
- `POST /api/workflows/validate`
- `POST /api/workflow/executions`
- `GET /api/workflow/executions`
- `GET /api/workflow/executions/{id}`
- `GET /api/workflow/executions/{id}/node-executions`

Conversations remain as a debugging surface:

- `GET /api/conversations`
- `GET /api/conversations/{id}`

Recommended new workflow execution request shape:

- `workflow_id`
- `input`
- `shell_root`
- `metadata`

Hooks are out of scope for `0.2`.

## Migration Strategy

### Phase 1: Workflow Registry

- implement disk-backed workflow loader
- resolve by `workflow.id`
- validate duplicates and parse errors cleanly

### Phase 2: Compiler

- compile a workflow blueprint into `WorkflowSnapshot`
- produce a frozen resolved snapshot suitable for persistence
- preserve loop and conditional composite nodes

### Phase 3: Execution Records

- add `workflow_executions`
- add `node_executions`
- define status model and trace payload shape

### Phase 4: Inference Runtime Refactor

- create conversations from `NodeSnapshot` plus owning `WorkflowExecution`
- attach conversations to `node_execution_id`
- remove runtime dependence on DB agent specs

### Phase 5: API Migration

- add workflow and workflow execution APIs
- remove agent spec APIs
- rework conversation creation semantics

### Phase 6: UI Migration

- remove the existing UI for `0.2`

### Phase 7: Cleanup

- remove `models/agent/spec.go`
- remove spec migrations and resources
- rename packages that still assume "agents/specs" is the center of the product

## Expected Breaking Changes

- agent spec endpoints disappear
- conversation creation API changes
- the current UI is removed
- DB schema changes
- execution becomes workflow-first

This is acceptable for `0.2`.

## Immediate Next Step

Before implementation starts, agree on:

- the exact workflow directory path rules
- the final DB persistence format for `workflow_executions`, `node_executions`, and `conversations`
- the replacement REST surface for executing workflows and inspecting executions
