import {
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  applyNodeChanges,
  Background,
  BackgroundVariant,
  Controls,
  ReactFlow,
  type EdgeTypes,
  type NodeChange,
  type NodeMouseHandler,
  type NodeTypes,
  type ReactFlowInstance,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type {
  CanvasEdge,
  CanvasNode,
  ParsedWorkflow,
} from "../types/workflow";
import { BuilderRuntimeProvider } from "./BuilderContext";
import type {
  BuilderFlowNode,
  GroupBoxFlowNode,
  WireStyle,
  WorkflowFlowEdge,
  WorkflowFlowNode,
} from "./flowTypes";
import { StackIcon } from "./Icons";
import {
  estimateNodeHeight,
  layoutWorkflow,
  NODE_WIDTH,
} from "./layout";
import type { RunEntry } from "./runData";
import { WorkflowEdge } from "./WorkflowEdge";
import { GroupBoxNode, WorkflowNode } from "./WorkflowNode";

const nodeTypes = {
  workflow: WorkflowNode,
  groupBox: GroupBoxNode,
} satisfies NodeTypes;

const edgeTypes = {
  workflow: WorkflowEdge,
} satisfies EdgeTypes;

function toFlowNodes(nodes: CanvasNode[], readOnly: boolean) {
  return nodes.map((node): WorkflowFlowNode => ({
    id: node.id,
    type: "workflow",
    position: { x: node.x, y: node.y },
    data: { canvas: node },
    draggable: !readOnly,
    selectable: true,
    connectable: false,
    zIndex: 1,
    ariaLabel: `${node.name} workflow node`,
  }));
}

function buildGroupBoxes(
  nodes: WorkflowFlowNode[],
  expandedGroups: Set<string>,
) {
  const boxes: GroupBoxFlowNode[] = [];

  for (const group of nodes) {
    const canvas = group.data.canvas;
    if (!canvas.isGroup || !expandedGroups.has(group.id)) {
      continue;
    }
    const children = nodes.filter(
      (node) => node.data.canvas.parentGroup === group.id,
    );
    if (children.length === 0) {
      continue;
    }

    const padding = 24;
    const minimumX =
      Math.min(...children.map((node) => node.position.x)) - padding;
    const minimumY =
      Math.min(...children.map((node) => node.position.y)) - padding;
    const maximumX =
      Math.max(
        ...children.map((node) => node.position.x + NODE_WIDTH),
      ) + padding;
    const maximumY =
      Math.max(
        ...children.map(
          (node) =>
            node.position.y + estimateNodeHeight(node.data.canvas),
        ),
      ) + padding;

    boxes.push({
      id: `group-box:${group.id}`,
      type: "groupBox",
      position: { x: minimumX, y: minimumY },
      data: { label: canvas.name },
      style: {
        width: maximumX - minimumX,
        height: maximumY - minimumY,
      },
      draggable: false,
      selectable: false,
      connectable: false,
      focusable: false,
      zIndex: -1,
    });
  }

  return boxes;
}

function isVisibleNode(
  node: WorkflowFlowNode,
  nodesById: Map<string, WorkflowFlowNode>,
  expandedGroups: Set<string>,
) {
  let parentId = node.data.canvas.parentGroup;
  const visited = new Set<string>();
  while (parentId) {
    if (visited.has(parentId) || !expandedGroups.has(parentId)) {
      return false;
    }
    visited.add(parentId);
    parentId = nodesById.get(parentId)?.data.canvas.parentGroup;
  }
  return true;
}

export function WorkflowCanvas({
  parsed,
  loading,
  error,
  currentRun,
  runs,
  readOnly,
  showRunStatus,
  onSelectNode,
  onSelectRun,
  onOpenWorkflow,
  onOpenNode,
  emptyTitle,
  emptyDescription,
  topOverlay,
  bottomOverlay,
}: {
  parsed: ParsedWorkflow;
  loading: boolean;
  error: string;
  currentRun: RunEntry | null;
  runs: RunEntry[];
  readOnly: boolean;
  showRunStatus: boolean;
  onSelectNode: (nodeId: string | null) => void;
  onSelectRun: (fullId: string) => void;
  onOpenWorkflow?: (workflowId: string) => void;
  /** Double-click on a plain node (monitor: open its conversation). */
  onOpenNode?: (nodeId: string) => void;
  emptyTitle: string;
  emptyDescription: string;
  topOverlay?: ReactNode;
  bottomOverlay?: ReactNode;
}) {
  const [flowNodes, setFlowNodes] = useState<WorkflowFlowNode[]>([]);
  const [canvasEdges, setCanvasEdges] = useState<CanvasEdge[]>([]);
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(
    new Set(),
  );
  const flowInstance = useRef<
    ReactFlowInstance<BuilderFlowNode, WorkflowFlowEdge> | undefined
  >(undefined);

  useEffect(() => {
    const laidOut = layoutWorkflow(parsed);
    setFlowNodes(toFlowNodes(laidOut.nodes, readOnly));
    setCanvasEdges(laidOut.edges);
    setExpandedGroups(new Set());
    // Selection is the parent's (often the URL's); a stale id simply
    // matches nothing, so there is nothing to clear here.
  }, [parsed, readOnly]);

  useEffect(() => {
    if (flowNodes.length === 0) {
      return;
    }
    const animationFrame = window.requestAnimationFrame(() => {
      void flowInstance.current?.fitView({
        padding: 0.16,
        minZoom: 0.4,
        maxZoom: 1.1,
        duration: 250,
      });
    });
    return () => window.cancelAnimationFrame(animationFrame);
  }, [flowNodes.length, parsed]);

  const nodesById = useMemo(
    () => new Map(flowNodes.map((node) => [node.id, node])),
    [flowNodes],
  );
  const visibleWorkflowNodes = useMemo(
    () =>
      flowNodes.filter((node) =>
        isVisibleNode(node, nodesById, expandedGroups),
      ),
    [expandedGroups, flowNodes, nodesById],
  );
  const renderedNodes = useMemo<BuilderFlowNode[]>(() => {
    const boxes = buildGroupBoxes(
      visibleWorkflowNodes,
      expandedGroups,
    );
    return [...boxes, ...visibleWorkflowNodes];
  }, [expandedGroups, visibleWorkflowNodes]);
  const renderedEdges = useMemo<WorkflowFlowEdge[]>(() => {
    const visibleIds = new Set(
      visibleWorkflowNodes.map((node) => node.id),
    );
    return canvasEdges
      .filter(
        (edge) =>
          visibleIds.has(edge.from) && visibleIds.has(edge.to),
      )
      .map((edge) => {
        const sourceNode = nodesById.get(edge.from)?.data.canvas;
        const portType =
          sourceNode?.outputs.find(
            (port) => port.id === edge.fromPort,
          )?.type ?? "any";
        const sourceStatus = showRunStatus
          ? currentRun?.nodes[edge.from]?.status
          : undefined;
        const targetStatus = showRunStatus
          ? currentRun?.nodes[edge.to]?.status
          : undefined;
        const active =
          sourceStatus === "run" || targetStatus === "run";
        return {
          id: edge.id,
          type: "workflow",
          source: edge.from,
          sourceHandle: edge.fromPort,
          target: edge.to,
          targetHandle: edge.toPort,
          animated: active,
          selectable: false,
          deletable: false,
          focusable: false,
          data: {
            active,
            color: `var(--t-${portType})`,
            portType,
            wireStyle: "bezier" as WireStyle,
          },
        };
      });
  }, [
    canvasEdges,
    currentRun,
    nodesById,
    showRunStatus,
    visibleWorkflowNodes,
  ]);

  const onNodesChange = useCallback(
    (changes: NodeChange<BuilderFlowNode>[]) => {
      setFlowNodes((current) => {
        const currentIds = new Set(current.map((node) => node.id));
        const applicable = changes.filter(
          (change) => "id" in change && currentIds.has(change.id),
        ) as NodeChange<WorkflowFlowNode>[];
        return applyNodeChanges(applicable, current);
      });
    },
    [],
  );
  const toggleGroup = useCallback((nodeId: string) => {
    setExpandedGroups((current) => {
      const next = new Set(current);
      if (next.has(nodeId)) {
        next.delete(nodeId);
      } else {
        next.add(nodeId);
      }
      return next;
    });
  }, []);
  const onNodeClick: NodeMouseHandler<BuilderFlowNode> = (
    _event,
    node,
  ) => {
    if (node.type === "workflow") {
      onSelectNode(node.id);
    }
  };
  const onNodeDoubleClick: NodeMouseHandler<BuilderFlowNode> = (
    _event,
    node,
  ) => {
    if (node.type !== "workflow") {
      return;
    }
    const canvas = node.data.canvas;
    const workflowId = canvas.body.find(
      (field) => field.k === "workflow",
    )?.v;
    if (workflowId && onOpenWorkflow) {
      onOpenWorkflow(workflowId);
      return;
    }
    if (!canvas.isGroup && onOpenNode) {
      onOpenNode(canvas.id);
    }
  };
  const runtime = useMemo(
    () => ({
      currentRun,
      expandedGroups,
      runs,
      showRunStatus,
      onSelectRun,
      onToggleGroup: toggleGroup,
      onOpenNode,
    }),
    [
      currentRun,
      expandedGroups,
      onOpenNode,
      onSelectRun,
      runs,
      showRunStatus,
      toggleGroup,
    ],
  );

  return (
    <BuilderRuntimeProvider value={runtime}>
      <main className="builder-canvas" data-component="WorkflowCanvas">
        {topOverlay}
        {error && <div className="builder-error">{error}</div>}
        {!loading && renderedNodes.length === 0 && (
          <div className="builder-canvas__empty">
            <StackIcon size={40} />
            <b>{emptyTitle}</b>
            <span>{emptyDescription}</span>
          </div>
        )}
        {loading && (
          <div className="builder-canvas__empty">
            <span>Loading workflow…</span>
          </div>
        )}
        <ReactFlow<BuilderFlowNode, WorkflowFlowEdge>
          nodes={renderedNodes}
          edges={renderedEdges}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          onInit={(instance) => {
            flowInstance.current = instance;
          }}
          onNodesChange={onNodesChange}
          onNodeClick={onNodeClick}
          onNodeDoubleClick={onNodeDoubleClick}
          onPaneClick={() => onSelectNode(null)}
          minZoom={0.4}
          maxZoom={1.6}
          zoomOnDoubleClick={false}
          nodesDraggable={!readOnly}
          nodesConnectable={false}
          edgesReconnectable={false}
          deleteKeyCode={null}
          fitView
          fitViewOptions={{ padding: 0.16, maxZoom: 1.1 }}
          proOptions={{ hideAttribution: true }}
        >
          <Background
            variant={BackgroundVariant.Dots}
            gap={20}
            size={1.6}
            color="var(--ink-4)"
          />
          <Controls
            position="top-right"
            showInteractive={false}
            fitViewOptions={{ padding: 0.16, maxZoom: 1.1 }}
          />
        </ReactFlow>
        {bottomOverlay}
      </main>
    </BuilderRuntimeProvider>
  );
}
