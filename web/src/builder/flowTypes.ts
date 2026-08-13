import type { Edge, Node } from "@xyflow/react";
import type { CanvasNode, PortType } from "../types/workflow";

export type WireStyle = "bezier" | "orthogonal" | "straight" | "return";

export type WorkflowNodeData = Record<string, unknown> & {
  canvas: CanvasNode;
};

export type WorkflowFlowNode = Node<WorkflowNodeData, "workflow">;
export type BuilderFlowNode = WorkflowFlowNode;

export type WorkflowEdgeData = Record<string, unknown> & {
  active: boolean;
  color: string;
  portType: PortType;
  wireStyle: WireStyle;
  /** Wiring the view implies (group plumbing), not a spec binding. */
  implicit?: boolean;
  /** Caption rendered on the wire (loop feedback contract). */
  label?: string;
  /** "return" wires: the y their below-the-graph run drops to. */
  dropY?: number;
};

export type WorkflowFlowEdge = Edge<WorkflowEdgeData, "workflow">;
