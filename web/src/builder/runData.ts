import type {
  NodeExecution,
  WorkflowExecution,
  WorkflowExecutionStatus,
} from "../types/api";
import {
  WORKFLOW_INPUTS_NODE_ID,
  WORKFLOW_OUTPUTS_NODE_ID,
} from "../api/specs";
import type { RunDisplayStatus } from "../types/workflow";

export interface RunNodeSnapshot {
  /** The node execution row's id — the key to its conversations. */
  nodeExecutionId: string;
  /** Set when this node's result was reused from a prior execution. */
  reusedFrom?: string;
  status: RunDisplayStatus;
  ms: number;
  tokens: number;
  inputSnapshot?: Record<string, unknown>;
  outputSnapshot?: Record<string, unknown>;
  error: string | null;
}

export interface RunEntry {
  id: string;
  fullId: string;
  when: string;
  whenAbsolute: string;
  status: RunDisplayStatus;
  executionStatus: WorkflowExecutionStatus;
  duration: number;
  tokens: number;
  cost: number;
  outputs?: Record<string, unknown>;
  nodes: Record<string, RunNodeSnapshot>;
}

export function mapStatus(status?: string): RunDisplayStatus {
  if (status === "succeeded") {
    return "ok";
  }
  if (
    status === "failed" ||
    status === "canceled" ||
    status === "blocked"
  ) {
    return "err";
  }
  if (
    status === "running" ||
    status === "queued" ||
    status === "pending"
  ) {
    return "run";
  }
  return "idle";
}

export function timeAgo(dateString?: string) {
  if (!dateString) {
    return "—";
  }
  const milliseconds = new Date(dateString).getTime();
  if (Number.isNaN(milliseconds)) {
    return "—";
  }

  const seconds = Math.max(
    0,
    Math.floor((Date.now() - milliseconds) / 1000),
  );
  if (seconds < 60) {
    return `${seconds}s ago`;
  }
  if (seconds < 3600) {
    return `${Math.floor(seconds / 60)} min ago`;
  }
  if (seconds < 86400) {
    return `${Math.floor(seconds / 3600)} hr ago`;
  }
  return `${Math.floor(seconds / 86400)}d ago`;
}

export function formatClock(dateString?: string) {
  if (!dateString) {
    return "—";
  }
  const date = new Date(dateString);
  if (Number.isNaN(date.getTime())) {
    return "—";
  }
  return date.toLocaleTimeString("en-US", { hour12: false });
}

export function computeMilliseconds(
  startedAt?: string,
  finishedAt?: string,
) {
  if (!startedAt) {
    return 0;
  }
  const started = new Date(startedAt).getTime();
  const finished = finishedAt
    ? new Date(finishedAt).getTime()
    : Date.now();
  if (Number.isNaN(started) || Number.isNaN(finished)) {
    return 0;
  }
  return Math.max(0, finished - started);
}

function traceError(nodeExecution: NodeExecution) {
  const value = nodeExecution.trace?.error;
  if (typeof value === "string") {
    return value;
  }
  return value == null ? null : JSON.stringify(value);
}

export function nodeSnapshotFrom(
  nodeExecution: NodeExecution,
): RunNodeSnapshot {
  return {
    nodeExecutionId: nodeExecution.id,
    status: mapStatus(nodeExecution.status),
    ms: computeMilliseconds(
      nodeExecution.started_at,
      nodeExecution.finished_at,
    ),
    tokens: 0,
    inputSnapshot: nodeExecution.input_snapshot,
    outputSnapshot: nodeExecution.output_snapshot,
    error: traceError(nodeExecution),
  };
}

export function buildRunEntry(
  execution: WorkflowExecution,
  nodeExecutions: NodeExecution[],
): RunEntry {
  // Loop iterations share one node_id (differing by iteration_index),
  // so a node's entry aggregates: any running iteration keeps the node
  // running, failures beat successes, durations sum, and the latest
  // iteration provides the snapshots.
  const statusRank: Record<RunDisplayStatus, number> = {
    run: 4,
    err: 3,
    warn: 2,
    ok: 1,
    idle: 0,
  };
  const nodeMap: Record<string, RunNodeSnapshot> = {};
  for (const nodeExecution of nodeExecutions) {
    const next = nodeSnapshotFrom(nodeExecution);
    const prior = nodeMap[nodeExecution.node_id];
    if (!prior) {
      nodeMap[nodeExecution.node_id] = next;
      continue;
    }
    nodeMap[nodeExecution.node_id] = {
      ...next,
      status:
        statusRank[next.status] >= statusRank[prior.status]
          ? next.status
          : prior.status,
      ms: prior.ms + next.ms,
      tokens: prior.tokens + next.tokens,
      error: next.error ?? prior.error,
      inputSnapshot: next.inputSnapshot ?? prior.inputSnapshot,
      outputSnapshot: next.outputSnapshot ?? prior.outputSnapshot,
    };
  }

  if (
    execution.input_snapshot &&
    Object.keys(execution.input_snapshot).length > 0
  ) {
    // "ok", not "idle" — the inspector hides values of idle nodes,
    // and inputs are known from the moment the run starts.
    nodeMap[WORKFLOW_INPUTS_NODE_ID] = {
      nodeExecutionId: "",
      status: "ok",
      ms: 0,
      tokens: 0,
      outputSnapshot: execution.input_snapshot,
      error: null,
    };
  }

  if (
    execution.output_snapshot &&
    Object.keys(execution.output_snapshot).length > 0
  ) {
    // The Outputs node's ports are inputs — the final values arrive
    // into it.
    nodeMap[WORKFLOW_OUTPUTS_NODE_ID] = {
      nodeExecutionId: "",
      status: mapStatus(execution.status),
      ms: 0,
      tokens: 0,
      inputSnapshot: execution.output_snapshot,
      outputSnapshot: execution.output_snapshot,
      error: null,
    };
  }

  return {
    id: execution.id?.substring(0, 8) || "—",
    fullId: execution.id,
    when: timeAgo(execution.created_at),
    whenAbsolute: formatClock(execution.created_at),
    status: mapStatus(execution.status),
    executionStatus: execution.status,
    duration: computeMilliseconds(
      execution.started_at,
      execution.finished_at,
    ),
    tokens: 0,
    cost: 0,
    outputs: execution.output_snapshot,
    nodes: nodeMap,
  };
}

export function isTerminalStatus(status?: string) {
  return (
    status === "succeeded" ||
    status === "failed" ||
    status === "canceled" ||
    status === "blocked"
  );
}
