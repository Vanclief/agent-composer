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
  WireStyle,
  WorkflowFlowEdge,
  WorkflowFlowNode,
} from "./flowTypes";
import { StackIcon } from "./Icons";
import { estimateNodeHeight, layoutWorkflow } from "./layout";
import type { RunEntry } from "./runData";
import { RETURN_PORT_ID, scopeWorkflow } from "./scope";
import { WorkflowEdge } from "./WorkflowEdge";
import { WorkflowNode } from "./WorkflowNode";

const nodeTypes = {
  workflow: WorkflowNode,
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

/** The caption on a loop's feedback wire, from the group's spec. */
function returnWireLabel(focus: CanvasNode) {
  const breaksOn =
    typeof focus.config.breaksOn === "string" ? focus.config.breaksOn : "";
  const updates =
    typeof focus.config.updates === "string" ? focus.config.updates : "";
  const maxIterations =
    typeof focus.config.maxIterations === "number"
      ? focus.config.maxIterations
      : 0;
  const parts = [breaksOn ? `↻ until ${breaksOn}` : "↻ repeats"];
  if (updates) {
    parts.push(`updates ${updates}`);
  }
  if (maxIterations) {
    parts.push(`max ${maxIterations}`);
  }
  return parts.join(" · ");
}

/** The contract shown beside the breadcrumb (kind + loop bounds). */
function crumbMeta(focus: CanvasNode) {
  const parts: string[] = [];
  if (typeof focus.config.kind === "string" && focus.config.kind) {
    parts.push(focus.config.kind);
  }
  if (typeof focus.config.breaksOn === "string" && focus.config.breaksOn) {
    parts.push(`until ${focus.config.breaksOn}`);
  }
  if (typeof focus.config.routesOn === "string" && focus.config.routesOn) {
    parts.push(`routes on ${focus.config.routesOn}`);
  }
  if (typeof focus.config.maxIterations === "number" && focus.config.maxIterations) {
    parts.push(`max ${focus.config.maxIterations}`);
  }
  return parts.join(" · ");
}

export function WorkflowCanvas({
  parsed,
  loading,
  error,
  currentRun,
  runs,
  readOnly,
  showRunStatus,
  focusGroupId = null,
  onFocusGroup,
  rootCrumb,
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
  /** Group whose body the canvas shows; null for the top level. */
  focusGroupId?: string | null;
  /** Drill into a group (or back out with null). */
  onFocusGroup?: (groupId: string | null) => void;
  /** Label of the breadcrumb's top-level crumb (workflow name). */
  rootCrumb?: string;
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
  const flowInstance = useRef<
    ReactFlowInstance<BuilderFlowNode, WorkflowFlowEdge> | undefined
  >(undefined);

  // A stale focus id (edited spec, old link) falls back to the top
  // level inside scopeWorkflow, so there is nothing to guard here.
  const scoped = useMemo(
    () => scopeWorkflow(parsed, focusGroupId),
    [parsed, focusGroupId],
  );

  useEffect(() => {
    const laidOut = layoutWorkflow(scoped.view);
    setFlowNodes(toFlowNodes(laidOut.nodes, readOnly));
    setCanvasEdges(laidOut.edges);
    // Selection is the parent's (often the URL's); a stale id simply
    // matches nothing, so there is nothing to clear here.
  }, [scoped, readOnly]);

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
  }, [flowNodes.length, scoped]);

  const nodesById = useMemo(
    () => new Map(flowNodes.map((node) => [node.id, node])),
    [flowNodes],
  );
  const renderedEdges = useMemo<WorkflowFlowEdge[]>(() => {
    // The loop's feedback wire runs underneath the whole graph.
    const graphBottom = Math.max(
      0,
      ...flowNodes.map(
        (node) =>
          node.position.y + estimateNodeHeight(node.data.canvas),
      ),
    );
    return canvasEdges.map((edge) => {
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
      const active = sourceStatus === "run" || targetStatus === "run";
      const isReturn = edge.fromPort === RETURN_PORT_ID;
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
          wireStyle: (isReturn ? "return" : "bezier") as WireStyle,
          ...(isReturn && scoped.focus
            ? {
                implicit: true,
                label: returnWireLabel(scoped.focus),
                dropY: graphBottom + 56,
              }
            : {}),
        },
      };
    });
  }, [
    canvasEdges,
    currentRun,
    flowNodes,
    nodesById,
    scoped,
    showRunStatus,
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
  // Composed workflows open on their own page — they are edited
  // there; local loop/conditional bodies drill in place.
  const enterGroup = useCallback(
    (nodeId: string) => {
      const canvas = nodesById.get(nodeId)?.data.canvas;
      if (!canvas) {
        return;
      }
      const workflowId = canvas.body.find(
        (field) => field.k === "workflow",
      )?.v;
      if (workflowId && onOpenWorkflow) {
        onOpenWorkflow(workflowId);
        return;
      }
      if (onFocusGroup) {
        onFocusGroup(nodeId);
      }
    },
    [nodesById, onFocusGroup, onOpenWorkflow],
  );
  const onNodeClick: NodeMouseHandler<BuilderFlowNode> = (
    _event,
    node,
  ) => {
    onSelectNode(node.id);
  };
  const onNodeDoubleClick: NodeMouseHandler<BuilderFlowNode> = (
    _event,
    node,
  ) => {
    const canvas = node.data.canvas;
    if (canvas.isGroup) {
      enterGroup(canvas.id);
      return;
    }
    const workflowId = canvas.body.find(
      (field) => field.k === "workflow",
    )?.v;
    if (workflowId && onOpenWorkflow) {
      onOpenWorkflow(workflowId);
      return;
    }
    if (onOpenNode) {
      onOpenNode(canvas.id);
    }
  };

  // Esc climbs one level out of the drilled group.
  useEffect(() => {
    if (!scoped.focus || !onFocusGroup) {
      return;
    }
    const parentId = scoped.focus.parentGroup ?? null;
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (
        event.key !== "Escape" ||
        target?.closest("input, textarea, select, [contenteditable]")
      ) {
        return;
      }
      onFocusGroup(parentId);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [scoped, onFocusGroup]);

  const runtime = useMemo(
    () => ({
      currentRun,
      runs,
      showRunStatus,
      onSelectRun,
      onEnterGroup: enterGroup,
      onOpenNode,
    }),
    [
      currentRun,
      enterGroup,
      onOpenNode,
      onSelectRun,
      runs,
      showRunStatus,
    ],
  );

  const meta = scoped.focus ? crumbMeta(scoped.focus) : "";

  return (
    <BuilderRuntimeProvider value={runtime}>
      <main className="builder-canvas" data-component="WorkflowCanvas">
        {topOverlay}
        {scoped.focus && onFocusGroup && (
          <nav className="builder-crumbs" aria-label="Group breadcrumb">
            <button
              type="button"
              title="Back to the whole workflow (Esc)"
              onClick={() => onFocusGroup(null)}
            >
              {rootCrumb || "workflow"}
            </button>
            {scoped.trail.map((group) => (
              <span key={group.id} className="builder-crumbs__step">
                <span className="builder-crumbs__sep">/</span>
                <button
                  type="button"
                  onClick={() => onFocusGroup(group.id)}
                >
                  {group.name}
                </button>
              </span>
            ))}
            <span className="builder-crumbs__sep">/</span>
            <span className="builder-crumbs__current">
              {scoped.focus.name}
            </span>
            {meta && (
              <span className="builder-crumbs__meta">{meta}</span>
            )}
          </nav>
        )}
        {error && <div className="builder-error">{error}</div>}
        {!loading && flowNodes.length === 0 && (
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
          nodes={flowNodes}
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
