import type {
  NodeExecution,
  WorkflowExecution,
  WorkflowExecutionStatus,
} from "../types/api";
import type { RunDisplayStatus } from "../types/workflow";

export interface RunNodeSnapshot {
  /** The node execution row's id — the key to its conversations. */
  nodeExecutionId: string;
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

export function buildRunEntry(
  execution: WorkflowExecution,
  nodeExecutions: NodeExecution[],
): RunEntry {
  const nodeMap: Record<string, RunNodeSnapshot> = {};
  for (const nodeExecution of nodeExecutions) {
    nodeMap[nodeExecution.node_id] = {
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
