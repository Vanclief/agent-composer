import type { Edge, Node } from "@xyflow/react";
import type { CanvasNode, PortType } from "../types/workflow";

export type WireStyle = "bezier" | "orthogonal" | "straight";

export type WorkflowNodeData = Record<string, unknown> & {
  canvas: CanvasNode;
};

export type GroupBoxNodeData = Record<string, unknown>;

export type WorkflowFlowNode = Node<WorkflowNodeData, "workflow">;
export type GroupBoxFlowNode = Node<GroupBoxNodeData, "groupBox">;
export type BuilderFlowNode = WorkflowFlowNode | GroupBoxFlowNode;

export type WorkflowEdgeData = Record<string, unknown> & {
  active: boolean;
  color: string;
  portType: PortType;
  wireStyle: WireStyle;
};

export type WorkflowFlowEdge = Edge<WorkflowEdgeData, "workflow">;
