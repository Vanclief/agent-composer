import type { JsonObject } from "./api";

export type CanvasNodeKind =
  | "llm"
  | "trigger"
  | "transform"
  | "input"
  | "output";
export type PortType = "text" | "json" | "file" | "any";
export type RunDisplayStatus = "idle" | "run" | "ok" | "warn" | "err";

export interface CanvasPort {
  id: string;
  label: string;
  type: PortType;
}

export interface CanvasField {
  k: string;
  v: string;
  mono?: boolean;
}

export interface CanvasNodeLastRun {
  input?: JsonObject;
  output?: JsonObject;
  status?: RunDisplayStatus;
  ms?: number;
  error?: string | null;
}

export interface CanvasNodeConfig {
  model?: string;
  instruction?: string;
  harnessId?: string;
  reasoningEffort?: string;
  /** Pretty-printed resolved output schema, keyed by output name. */
  outputSchema?: string;
  kind?: string;
  operation?: string;
  [key: string]: unknown;
}

export interface CanvasNode {
  id: string;
  kind: CanvasNodeKind;
  name: string;
  sub: string;
  x: number;
  y: number;
  config: CanvasNodeConfig;
  inputs: CanvasPort[];
  outputs: CanvasPort[];
  body: CanvasField[];
  last: CanvasNodeLastRun;
  parentGroup?: string | null;
  isGroup?: boolean;
  groupLabel?: string;
  childCount?: number;
  /** Defined in another workflow's YAML — not editable from here. */
  foreign?: boolean;
}

export interface CanvasEdge {
  id: string;
  from: string;
  fromPort: string;
  to: string;
  toPort: string;
}

export interface ParsedWorkflow {
  nodes: CanvasNode[];
  edges: CanvasEdge[];
  order: string[];
}

export interface SpecHeader {
  id?: string;
  name?: string;
  version?: string;
  description?: string;
  inputs?: Record<string, string>;
  outputs?: Record<string, SpecOutput>;
}

export interface SpecOutput {
  schema?: string;
  from?: string;
}

export interface SpecHarness {
  id?: string;
  model?: string;
  reasoning_effort?: string;
  [key: string]: unknown;
}

export interface SpecNodeConfig {
  harness?: SpecHarness;
  instruction?: string;
  [key: string]: unknown;
}

export interface SpecNode {
  kind?: string;
  workflow_slug?: string;
  operation?: string;
  executes?: string;
  over?: string;
  updates?: string;
  breaks_on?: string;
  max_iterations?: number;
  routes_on?: string;
  when_true?: string;
  when_false?: string;
  inputs?: Record<string, unknown>;
  outputs?: Record<string, unknown>;
  config?: SpecNodeConfig;
}

export interface SpecInstance {
  node?: string;
  inputs?: Record<string, unknown>;
}

export interface SpecDocument {
  workflow?: SpecHeader;
  schemas?: Record<string, unknown>;
  nodes?: Record<string, SpecNode>;
  flow?: {
    instances?: Record<string, SpecInstance>;
  };
}
