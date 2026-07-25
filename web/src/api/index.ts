import type {
  AppConfig,
  NodeExecutionListResponse,
  WorkflowExecution,
  WorkflowExecutionCreateRequest,
  WorkflowExecutionCreateResponse,
  WorkflowExecutionListResponse,
  WorkflowListResponse,
  WorkflowSpecResponse,
  WorkflowSummary,
} from "../types/api";
import { fetchJSON, postJSON } from "./client";

export function fetchConfig(signal?: AbortSignal) {
  return fetchJSON<AppConfig>("/api/config", undefined, signal);
}

export async function fetchWorkflows(
  signal?: AbortSignal,
): Promise<WorkflowSummary[]> {
  const body = await fetchJSON<WorkflowListResponse>(
    "/api/workflows",
    undefined,
    signal,
  );
  return body?.workflows ?? [];
}

export async function fetchWorkflowSpec(
  workflowId: string,
  signal?: AbortSignal,
) {
  const body = await fetchJSON<WorkflowSpecResponse>(
    `/api/workflows/${encodeURIComponent(workflowId)}`,
    undefined,
    signal,
  );
  return body?.spec ?? "";
}

export async function fetchWorkflowSpecs(
  workflows: WorkflowSummary[],
  signal?: AbortSignal,
) {
  const entries = await Promise.all(
    workflows.map(async (workflow) => {
      try {
        const spec = await fetchWorkflowSpec(workflow.id, signal);
        return [workflow.id, spec] as const;
      } catch {
        return null;
      }
    }),
  );
  return Object.fromEntries(
    entries.filter((entry): entry is readonly [string, string] => entry !== null),
  );
}

export async function fetchWorkflowExecutions(
  workflowId?: string,
  limit = 20,
  signal?: AbortSignal,
) {
  const body = await fetchJSON<WorkflowExecutionListResponse>(
    "/api/workflow/executions",
    {
      workflow_id: workflowId,
      limit,
    },
    signal,
  );
  return body?.workflow_executions ?? [];
}

export function fetchWorkflowExecution(
  executionId: string,
  signal?: AbortSignal,
) {
  return fetchJSON<WorkflowExecution>(
    `/api/workflow/executions/${encodeURIComponent(executionId)}`,
    undefined,
    signal,
  );
}

export function createWorkflowExecution(
  request: WorkflowExecutionCreateRequest,
) {
  return postJSON<WorkflowExecutionCreateResponse>(
    "/api/workflow/executions",
    request,
  );
}

export async function fetchNodeExecutions(
  workflowExecutionId?: string,
  limit = 200,
  signal?: AbortSignal,
) {
  const body = await fetchJSON<NodeExecutionListResponse>(
    "/api/workflow/node-executions",
    {
      workflow_execution_id: workflowExecutionId,
      limit,
    },
    signal,
  );
  return body?.node_executions ?? [];
}
