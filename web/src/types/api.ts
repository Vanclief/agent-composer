export type JsonObject = Record<string, unknown>;

export interface CursorPage {
  next_cursor?: string;
  has_next_page: boolean;
  hash?: string;
}

export interface AppConfig {
  shell_root: string;
}

export interface WorkflowSummary {
  id: string;
  name: string;
  description?: string;
  inputs: Record<string, string>;
  outputs: Record<string, string>;
}

export interface WorkflowListResponse {
  workflows: WorkflowSummary[];
}

export interface WorkflowSpecResponse {
  workflow_id: string;
  spec: string;
}

export type WorkflowExecutionStatus =
  | "queued"
  | "running"
  | "succeeded"
  | "failed"
  | "canceled"
  | "blocked";

export type NodeExecutionStatus = WorkflowExecutionStatus;

export interface SnapshotPort {
  Name: string;
  TypeRef: string;
  Schema?: JsonObject;
}

export interface SnapshotOutputBinding {
  Name: string;
  Schema?: JsonObject;
  From: SnapshotBinding;
}

export interface SnapshotBinding {
  Kind: "workflow_input" | "instance" | string;
  WorkflowInput?: string;
  InstanceID?: string;
  OutputName?: string;
}

export interface SnapshotNode {
  InstanceID?: string;
  NodeName?: string;
  Kind?: string;
  Operation?: string;
  Executes?: string;
  Over?: string;
  Updates?: string;
  BreaksOn?: string;
  MaxIterations?: number;
  RoutesOn?: string;
  WhenTrue?: string;
  WhenFalse?: string;
  Instruction?: string;
  Harness?: string;
  Model?: string;
  ReasoningEffort?: string;
  HarnessConfig?: unknown;
  Inputs?: Record<string, SnapshotPort>;
  InputOrder?: string[];
  InputBindings?: Record<string, SnapshotBinding>;
  Outputs?: Record<string, SnapshotPort>;
  Workflow?: WorkflowSnapshot;
  OutputName?: string;
  OutputSchema?: JsonObject;
  StructuredOutputSchema?: JsonObject;
  StructuredOutputSchemaRaw?: unknown;
  WrapStructuredOutput?: boolean;
  LoopTarget?: SnapshotNode;
  WhileTarget?: SnapshotWhileTarget;
  TrueTarget?: SnapshotNode;
  FalseTarget?: SnapshotNode;
}

export interface SnapshotWhileTarget {
  InstanceID?: string;
  NodeName?: string;
  Instruction?: string;
  Harness?: string;
  Model?: string;
  ReasoningEffort?: string;
  HarnessConfig?: unknown;
  Inputs?: Record<string, SnapshotPort>;
  Workflow?: WorkflowSnapshot;
  UpdateOutputName?: string;
  UpdateOutputSchema?: JsonObject;
  BreakOutputName?: string;
  StructuredOutputSchema?: JsonObject;
  StructuredOutputSchemaRaw?: unknown;
}

export interface WorkflowSnapshot {
  WorkflowID?: string;
  WorkflowVersion?: string;
  Description?: string;
  Inputs?: Record<string, SnapshotPort>;
  Outputs?: Record<string, SnapshotOutputBinding>;
  Nodes?: Record<string, SnapshotNode>;
  Order?: string[];
}

export interface WorkflowExecution {
  id: string;
  workflow_id: string;
  workflow_version: string;
  workflow_snapshot: WorkflowSnapshot | string;
  input_snapshot?: JsonObject;
  output_snapshot?: JsonObject;
  status: WorkflowExecutionStatus;
  shell_root?: string;
  started_at?: string;
  finished_at?: string;
  metadata?: JsonObject;
  created_at: string;
}

export interface WorkflowExecutionListResponse extends CursorPage {
  workflow_executions: WorkflowExecution[];
}

interface WorkflowExecutionCreateOptions {
  input: JsonObject;
  shell_root?: string;
  /** Branch name — the run executes in that branch's worktree. */
  worktree?: string;
  /** Start point when the worktree branch is new (default HEAD). */
  base?: string;
}

export interface WorktreeInfo {
  path: string;
  branch?: string;
  head?: string;
  is_main: boolean;
  detached: boolean;
}

export interface BranchInfo {
  name: string;
  is_local: boolean;
  is_remote: boolean;
}

export interface WorktreeListResponse {
  exists: boolean;
  is_git: boolean;
  repo?: string;
  worktrees: WorktreeInfo[] | null;
  branches?: BranchInfo[] | null;
}

export interface DirectoryEntry {
  name: string;
  path: string;
  has_git: boolean;
}

export interface DirectoryBrowseResponse {
  path: string;
  parent?: string;
  directories: DirectoryEntry[] | null;
}

export interface WorktreeCreateResponse {
  path: string;
  branch: string;
  created: boolean;
}

export type WorkflowExecutionCreateRequest =
  | (WorkflowExecutionCreateOptions & {
      workflow_id: string;
      file?: never;
      resume_from_execution_id?: never;
    })
  | (WorkflowExecutionCreateOptions & {
      workflow_id?: never;
      file: string;
      resume_from_execution_id?: never;
    })
  | {
      /** Re-run one node (and everything downstream) of a prior run. */
      resume_from_execution_id: string;
      resume_from_node: string;
      use_current_spec?: boolean;
      input?: JsonObject;
      shell_root?: string;
      worktree?: string;
      base?: string;
      workflow_id?: never;
      file?: never;
    };

export interface WorkflowExecutionCreateResponse {
  execution_id?: string;
  workflow_id: string;
  workflow_version: string;
  status: WorkflowExecutionStatus;
}

export interface NodeExecution {
  id: string;
  workflow_execution_id: string;
  parent_node_execution_id: string;
  node_id: string;
  kind: string;
  status: NodeExecutionStatus;
  node_snapshot: SnapshotNode | string;
  input_snapshot?: JsonObject;
  output_snapshot?: JsonObject;
  trace?: JsonObject;
  iteration_index?: number;
  branch_name?: string;
  started_at?: string;
  finished_at?: string;
  metadata?: JsonObject;
  created_at: string;
}

export interface NodeExecutionListResponse extends CursorPage {
  node_executions: NodeExecution[];
}

/** runtime/types.Message has no json tags, so keys are PascalCase. */
export interface ConversationToolCall {
  Name?: string;
  CallID?: string;
  Arguments?: string;
  JSONArguments?: unknown;
}

export interface ConversationMessage {
  Role: "system" | "user" | "assistant" | "tool" | string;
  Content?: string;
  Name?: string;
  ToolCallID?: string;
  ToolCall?: ConversationToolCall | null;
}

export interface TraceEvent {
  kind: "reasoning" | "message" | "command" | "tool" | "error" | string;
  content: string;
  detail?: string;
}

export interface Conversation {
  id: string;
  node_execution_id?: string;
  agent_name: string;
  harness: string;
  model: string;
  instructions?: string;
  messages: ConversationMessage[] | null;
  trace?: TraceEvent[] | null;
  status: string;
  harness_error?: string;
  input_tokens: number;
  output_tokens: number;
  cached_tokens: number;
  cost: number;
  created_at: string;
}

export interface ConversationListResponse {
  conversations: Conversation[] | null;
}
