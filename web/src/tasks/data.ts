import {
  createWorkflowExecution,
  fetchNodeExecutions,
  fetchWorkflowExecution,
  fetchWorkflowExecutions,
  fetchWorkflows,
} from "../api";
import type {
  JsonObject,
  NodeExecution,
  WorkflowExecution,
  WorkflowExecutionCreateResponse,
  WorkflowExecutionStatus,
  WorkflowSummary,
} from "../types/api";

// The UI always shows the backend's status verbatim — no derived states.
export type TaskStatus = WorkflowExecutionStatus;

export interface Task {
  id: string;
  title: string;
  status: TaskStatus;
  workflowIds: string[];
  input: JsonObject | undefined;
  output: JsonObject | undefined;
  startedAt: string | undefined;
  finishedAt: string | undefined;
  executions: WorkflowExecution[];
}

export interface TaskConsoleData {
  tasks: Task[];
  workflows: WorkflowSummary[];
}

const UUID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function compactValue(value: unknown) {
  if (typeof value === "string") {
    return value.replace(/\s+/g, " ").trim();
  }
  if (value === null || value === undefined) {
    return "";
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  try {
    return JSON.stringify(value).replace(/\s+/g, " ").trim();
  } catch {
    return "";
  }
}

function firstMeaningfulValue(value?: JsonObject) {
  for (const candidate of Object.values(value ?? {})) {
    const text = compactValue(candidate);
    if (text) {
      return text;
    }
  }
  return "";
}

function truncate(value: string, length: number) {
  if (value.length <= length) {
    return value;
  }
  return `${value.slice(0, length - 1).trimEnd()}…`;
}

export function executionToTask(
  execution: WorkflowExecution,
  workflowNames: Map<string, string>,
): Task {
  const workflowName =
    workflowNames.get(execution.workflow_slug) || execution.workflow_slug;
  const startedAt = execution.started_at || execution.created_at;
  return {
    id: execution.id,
    title: workflowName,
    status: execution.status,
    workflowIds: [execution.workflow_slug],
    input: execution.input_snapshot,
    output: execution.output_snapshot,
    startedAt,
    finishedAt: execution.finished_at,
    executions: [execution],
  };
}

function taskTime(task: Task) {
  return Date.parse(task.finishedAt || task.startedAt || "") || 0;
}

function sortTasks(tasks: Task[]) {
  return [...tasks].sort(
    (left, right) => taskTime(right) - taskTime(left),
  );
}

function workflowNameMap(workflows: WorkflowSummary[]) {
  return new Map(
    workflows.map((workflow) => [
      workflow.slug,
      workflow.name || workflow.slug,
    ]),
  );
}

export async function fetchTaskConsoleData(
  limit = 50,
  signal?: AbortSignal,
): Promise<TaskConsoleData> {
  const [workflows, executions] = await Promise.all([
    fetchWorkflows(signal),
    fetchWorkflowExecutions(undefined, limit, signal),
  ]);
  const names = workflowNameMap(workflows);
  const tasks = executions.map((execution) =>
    executionToTask(execution, names),
  );
  return {
    tasks: sortTasks(tasks),
    workflows,
  };
}

export async function fetchTask(
  taskId: string,
  signal?: AbortSignal,
) {
  const [execution, workflows] = await Promise.all([
    fetchWorkflowExecution(taskId, signal),
    fetchWorkflows(signal),
  ]);
  return executionToTask(execution, workflowNameMap(workflows));
}

export async function fetchTaskMonitorData(
  taskId: string,
  signal?: AbortSignal,
): Promise<TaskConsoleData & { task: Task }> {
  const data = await fetchTaskConsoleData(50, signal);
  const listed = data.tasks.find((task) => task.id === taskId);
  if (listed) {
    return { ...data, task: listed };
  }

  const execution = await fetchWorkflowExecution(taskId, signal);
  const task = executionToTask(
    execution,
    workflowNameMap(data.workflows),
  );
  return {
    ...data,
    task,
    tasks: sortTasks([task, ...data.tasks]),
  };
}

export async function fetchLatestTaskForWorkflow(
  workflowId: string,
  signal?: AbortSignal,
) {
  const [executions, workflows] = await Promise.all([
    fetchWorkflowExecutions(workflowId, 1, signal),
    fetchWorkflows(signal),
  ]);
  const latest = executions[0];
  return latest
    ? executionToTask(latest, workflowNameMap(workflows))
    : null;
}

export function fetchTaskNodeExecutions(
  executionId: string,
  signal?: AbortSignal,
): Promise<NodeExecution[]> {
  if (!UUID_PATTERN.test(executionId)) {
    // The API only accepts UUIDs, so asking it would just 400.
    return Promise.resolve([]);
  }
  return fetchNodeExecutions(executionId, 200, signal);
}

/** Re-run one node (and everything downstream) of a prior run. */
export function rerunFromNode(
  executionId: string,
  node: string,
): Promise<WorkflowExecutionCreateResponse> {
  return createWorkflowExecution({
    resume_from_execution_id: executionId,
    resume_from_node: node,
    use_current_spec: true,
  });
}

export function startTask(
  workflowId: string,
  input: JsonObject,
  projectDir?: string,
  worktree?: string,
): Promise<WorkflowExecutionCreateResponse> {
  return createWorkflowExecution({
    workflow_slug: workflowId,
    input,
    project_dir: projectDir?.trim() || undefined,
    worktree: worktree?.trim() || undefined,
  });
}

export function taskOneLine(task: Task) {
  const value = truncate(firstMeaningfulValue(task.output), 180);
  if (task.status === "failed" || task.status === "blocked") {
    return value || "Execution failed";
  }
  if (task.status === "canceled") {
    return value || "Execution canceled";
  }
  return value;
}

export function formatTaskElapsed(task: Task) {
  const started = Date.parse(task.startedAt || "");
  const finished = task.finishedAt
    ? Date.parse(task.finishedAt)
    : Date.now();
  if (Number.isNaN(started) || Number.isNaN(finished)) {
    return "—";
  }
  const seconds = Math.max(0, Math.floor((finished - started) / 1000));
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m ${seconds % 60}s`;
  }
  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`;
}
