import type {
  AppConfig,
  AppSettings,
  ComposeResponse,
  HarnessListResponse,
  Conversation,
  ConversationListResponse,
  DirectoryBrowseResponse,
  NodeExecutionListResponse,
  WorkflowExecution,
  WorkflowExecutionCreateRequest,
  WorkflowExecutionCreateResponse,
  WorkflowExecutionListResponse,
  WorkflowListResponse,
  WorkflowSpecResponse,
  WorkflowSummary,
  WorktreeCreateResponse,
  WorktreeListResponse,
} from "../types/api";
import { deleteJSON, fetchJSON, postJSON, putJSON } from "./client";

export function fetchConfig(signal?: AbortSignal) {
  return fetchJSON<AppConfig>("/api/config", undefined, signal);
}

export function fetchSettings(signal?: AbortSignal) {
  return fetchJSON<AppSettings>("/api/settings", undefined, signal);
}

export function updateSettings(settings: AppSettings) {
  return putJSON<AppSettings>("/api/settings", settings);
}

/** Scaffold a new named workflow — it starts life as a draft. */
export function createWorkflow(name: string, description: string) {
  return postJSON<{ workflow_id: string; name: string; draft: string }>(
    "/api/workflows",
    { name, description },
  );
}

/** One composer conversation — resolves when the edit has landed. */
export function composeWorkflow(workflowId: string, request: string) {
  return postJSON<ComposeResponse>("/api/workflows/compose", {
    workflow_id: workflowId,
    request,
  });
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
  return { spec: body?.spec ?? "", draft: body?.draft ?? "" };
}

/** Saved specs plus any unsaved drafts, keyed by workflow id. */
export async function fetchWorkflowSpecs(
  workflows: WorkflowSummary[],
  signal?: AbortSignal,
) {
  const entries = await Promise.all(
    workflows.map(async (workflow) => {
      try {
        const body = await fetchWorkflowSpec(workflow.id, signal);
        return [workflow.id, body] as const;
      } catch {
        return null;
      }
    }),
  );
  const specs: Record<string, string> = {};
  const drafts: Record<string, string> = {};
  for (const entry of entries) {
    if (!entry) {
      continue;
    }
    specs[entry[0]] = entry[1].spec;
    if (entry[1].draft) {
      drafts[entry[0]] = entry[1].draft;
    }
  }
  return { specs, drafts };
}

/** Promote a draft: bump the version and replace the saved file. */
export function saveWorkflowDraft(workflowId: string) {
  return postJSON<{ workflow_id: string; version: string; spec: string }>(
    `/api/workflows/${encodeURIComponent(workflowId)}/save`,
    {},
  );
}

export function discardWorkflowDraft(workflowId: string) {
  return deleteJSON<{ workflow_id: string; deleted: boolean }>(
    `/api/workflows/${encodeURIComponent(workflowId)}/draft`,
  );
}

export function updateWorkflowNode(
  workflowId: string,
  node: string,
  update: { model?: string; harness?: string; instruction?: string },
) {
  return putJSON<{ workflow_id: string; node: string; spec: string }>(
    `/api/workflows/${encodeURIComponent(workflowId)}/nodes/${encodeURIComponent(node)}`,
    update,
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

export function fetchHarnesses(signal?: AbortSignal) {
  return fetchJSON<HarnessListResponse>(
    "/api/harnesses",
    undefined,
    signal,
  );
}

export function browseDirectories(
  path: string,
  signal?: AbortSignal,
) {
  return fetchJSON<DirectoryBrowseResponse>(
    "/api/filesystem/directories",
    { path },
    signal,
  );
}

export function fetchWorktrees(
  repo: string,
  signal?: AbortSignal,
  fetchOrigin = false,
) {
  return fetchJSON<WorktreeListResponse>(
    "/api/worktrees",
    { repo, fetch: fetchOrigin ? "true" : undefined },
    signal,
  );
}

export function createWorktree(
  repo: string,
  branch: string,
  base?: string,
) {
  return postJSON<WorktreeCreateResponse>("/api/worktrees", {
    repo,
    branch,
    base: base?.trim() || undefined,
  });
}

export function removeWorktree(
  repo: string,
  path: string,
  force = false,
) {
  return deleteJSON<{ removed: string }>("/api/worktrees", {
    repo,
    path,
    force: force ? "true" : undefined,
  });
}

export async function fetchConversations(
  nodeExecutionId: string,
  signal?: AbortSignal,
): Promise<Conversation[]> {
  const body = await fetchJSON<ConversationListResponse>(
    "/api/workflow/conversations",
    { node_execution_id: nodeExecutionId },
    signal,
  );
  return body?.conversations ?? [];
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
