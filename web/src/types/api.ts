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
}

export type WorkflowExecutionCreateRequest =
  | (WorkflowExecutionCreateOptions & {
      workflow_id: string;
      file?: never;
    })
  | (WorkflowExecutionCreateOptions & {
      workflow_id?: never;
      file: string;
    });

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
