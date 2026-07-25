import type {
  AppConfig,
  Hook,
  HookCreateRequest,
  HookListResponse,
  HookUpdateRequest,
  NodeExecution,
  NodeExecutionListResponse,
  WorkflowExecution,
  WorkflowExecutionCreateRequest,
  WorkflowExecutionCreateResponse,
  WorkflowExecutionListResponse,
  WorkflowListResponse,
  WorkflowSpecResponse,
  WorkflowSummary,
} from "../types/api";
import { deleteJSON, fetchJSON, postJSON, putJSON } from "./client";

interface ListParams {
  cursor?: string;
  limit?: number;
}

export function fetchConfig() {
  return fetchJSON<AppConfig>("/api/config");
}

export function fetchHooks(
  params: ListParams & {
    search?: string;
    event_type?: string;
    agent_name?: string;
  } = {},
) {
  return fetchJSON<HookListResponse>("/api/hooks", {
    cursor: params.cursor,
    limit: params.limit,
    search: params.search,
    event_type: params.event_type,
    agent_name: params.agent_name,
  });
}

export function fetchHook(id: string) {
  return fetchJSON<Hook>(`/api/hooks/${encodeURIComponent(id)}`);
}

export function createHook(request: HookCreateRequest) {
  return postJSON<Hook>("/api/hooks", request);
}

export function updateHook(id: string, request: HookUpdateRequest) {
  return putJSON<Hook>(`/api/hooks/${encodeURIComponent(id)}`, request);
}

export function deleteHook(id: string) {
  return deleteJSON<string>(`/api/hooks/${encodeURIComponent(id)}`);
}

export async function fetchWorkflows(): Promise<WorkflowSummary[]> {
  const body = await fetchJSON<WorkflowListResponse>("/api/workflows");
  return body?.workflows ?? [];
}

export async function fetchWorkflowSpec(workflowId: string) {
  const body = await fetchJSON<WorkflowSpecResponse>(
    `/api/workflows/${encodeURIComponent(workflowId)}`,
  );
  return body?.spec ?? "";
}

export async function fetchWorkflowSpecs(workflows: WorkflowSummary[]) {
  const entries = await Promise.all(
    workflows.map(async (workflow) => {
      try {
        const spec = await fetchWorkflowSpec(workflow.id);
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
) {
  const body = await fetchJSON<WorkflowExecutionListResponse>(
    "/api/workflow/executions",
    {
      workflow_id: workflowId,
      limit,
    },
  );
  return body?.workflow_executions ?? [];
}

export function fetchWorkflowExecution(executionId: string) {
  return fetchJSON<WorkflowExecution>(
    `/api/workflow/executions/${encodeURIComponent(executionId)}`,
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
) {
  const body = await fetchJSON<NodeExecutionListResponse>(
    "/api/workflow/node-executions",
    {
      workflow_execution_id: workflowExecutionId,
      limit,
    },
  );
  return body?.node_executions ?? [];
}

export function fetchNodeExecution(nodeExecutionId: string) {
  return fetchJSON<NodeExecution>(
    `/api/workflow/node-executions/${encodeURIComponent(nodeExecutionId)}`,
  );
}

export { apiPath, deleteJSON, fetchJSON, postJSON, putJSON } from "./client";
