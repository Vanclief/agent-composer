import {
  type MouseEvent,
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
  MiniMap,
  Panel,
  ReactFlow,
  type EdgeTypes,
  type NodeChange,
  type NodeMouseHandler,
  type NodeTypes,
  type ReactFlowInstance,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import {
  createWorkflowExecution,
  fetchConfig,
  fetchNodeExecutions,
  fetchWorkflowExecution,
  fetchWorkflowExecutions,
  fetchWorkflows,
  fetchWorkflowSpecs,
} from "../api";
import {
  parseBlueprintYAML,
  parseSnapshot,
} from "../api/blueprints";
import type {
  WorkflowExecution,
  WorkflowSummary,
} from "../types/api";
import type {
  CanvasEdge,
  CanvasNode,
  ParsedWorkflow,
} from "../types/workflow";
import {
  useLocation,
  useNavigate,
  useParams,
} from "react-router-dom";
import { BuilderRuntimeProvider } from "./BuilderContext";
import { KIND_VISUAL, NODE_LIBRARY } from "./constants";
import type {
  BuilderFlowNode,
  GroupBoxFlowNode,
  WireStyle,
  WorkflowFlowEdge,
  WorkflowFlowNode,
} from "./flowTypes";
import {
  BlocksIcon,
  BoltIcon,
  CogIcon,
  HistoryIcon,
  KindIcon,
  PlayIcon,
  StackIcon,
  StopIcon,
} from "./Icons";
import { Inspector } from "./Inspector";
import { layoutWorkflow, NODE_WIDTH } from "./layout";
import {
  buildRunEntry,
  isTerminalStatus,
  type RunEntry,
} from "./runData";
import { RunInputModal } from "./RunInputModal";
import { RunMenuButton } from "./RunMenu";
import { ShellRootPicker } from "./ShellRootPicker";
import { WorkflowEdge } from "./WorkflowEdge";
import { GroupBoxNode, WorkflowNode } from "./WorkflowNode";

const nodeTypes = {
  workflow: WorkflowNode,
  groupBox: GroupBoxNode,
} satisfies NodeTypes;

const edgeTypes = {
  workflow: WorkflowEdge,
} satisfies EdgeTypes;

const TERMINAL_ERROR_STATUSES = new Set([
  "failed",
  "blocked",
  "canceled",
]);

function workflowPath(workflowId: string, runId?: string) {
  const path = `/workflow/${encodeURIComponent(workflowId)}`;
  return runId ? `${path}?run=${encodeURIComponent(runId)}` : path;
}

function readStoredRoots() {
  try {
    const value = JSON.parse(
      localStorage.getItem("agc.shellRoots") || "[]",
    ) as unknown;
    if (Array.isArray(value)) {
      return value.filter(
        (item): item is string =>
          typeof item === "string" && Boolean(item.trim()),
      );
    }
  } catch {
    // Ignore invalid values left by an older browser session.
  }
  return [];
}

function toFlowNodes(nodes: CanvasNode[]): WorkflowFlowNode[] {
  return nodes.map((node) => ({
    id: node.id,
    type: "workflow",
    position: { x: node.x, y: node.y },
    data: { canvas: node },
    draggable: true,
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
      Math.max(...children.map((node) => node.position.y + 300)) +
      padding;

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

export function BuilderPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { workflowId: routeWorkflowId } = useParams();

  const [workflows, setWorkflows] = useState<WorkflowSummary[]>([]);
  const [workflowSpecs, setWorkflowSpecs] = useState<
    Record<string, string>
  >({});
  const [catalogReady, setCatalogReady] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [drawer, setDrawer] = useState<"workflows" | "nodes">(
    "workflows",
  );
  const [workflowSearch, setWorkflowSearch] = useState("");
  const [nodeSearch, setNodeSearch] = useState("");

  const [flowNodes, setFlowNodes] = useState<WorkflowFlowNode[]>([]);
  const [canvasEdges, setCanvasEdges] = useState<CanvasEdge[]>([]);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(
    null,
  );
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(
    new Set(),
  );
  const [wireStyle, setWireStyle] = useState<WireStyle>("bezier");
  const [showGrid, setShowGrid] = useState(true);
  const [showMiniMap, setShowMiniMap] = useState(true);

  const [runHistory, setRunHistory] = useState<RunEntry[]>([]);
  const [selectedRunFullId, setSelectedRunFullId] = useState("");
  const [running, setRunning] = useState(false);
  const [liveExecutionId, setLiveExecutionId] = useState("");
  const [showRunModal, setShowRunModal] = useState(false);

  const [shellRoots, setShellRoots] = useState<string[]>(
    readStoredRoots,
  );
  const [shellRoot, setShellRoot] = useState(
    () => localStorage.getItem("agc.shellRoot") || "",
  );
  const [defaultShellRoot, setDefaultShellRoot] = useState("");

  const flowInstance = useRef<
    ReactFlowInstance<BuilderFlowNode, WorkflowFlowEdge> | undefined
  >(undefined);

  const activeWorkflowId = useMemo(() => {
    if (
      routeWorkflowId &&
      workflows.some((workflow) => workflow.id === routeWorkflowId)
    ) {
      return routeWorkflowId;
    }
    return "";
  }, [routeWorkflowId, workflows]);

  const activeWorkflow = workflows.find(
    (workflow) => workflow.id === activeWorkflowId,
  );
  const currentRun =
    runHistory.find((run) => run.fullId === selectedRunFullId) ??
    runHistory[0] ??
    null;
  const selectedNode =
    flowNodes.find((node) => node.id === selectedNodeId)?.data.canvas ??
    null;

  const refreshCatalog = useCallback(async () => {
    setError("");
    try {
      const nextWorkflows = await fetchWorkflows();
      const nextSpecs = await fetchWorkflowSpecs(nextWorkflows);
      setWorkflows(nextWorkflows);
      setWorkflowSpecs(nextSpecs);
      return nextWorkflows;
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
      setWorkflows([]);
      setWorkflowSpecs({});
      return [];
    } finally {
      setCatalogReady(true);
    }
  }, []);

  useEffect(() => {
    document.title = "AGC — Agent Workflow Builder";
    void refreshCatalog();
  }, [refreshCatalog]);

  useEffect(() => {
    try {
      localStorage.setItem("agc.shellRoots", JSON.stringify(shellRoots));
    } catch {
      // Storage can be disabled without preventing workflow execution.
    }
  }, [shellRoots]);

  useEffect(() => {
    try {
      localStorage.setItem("agc.shellRoot", shellRoot);
    } catch {
      // Storage can be disabled without preventing workflow execution.
    }
  }, [shellRoot]);

  useEffect(() => {
    let active = true;
    fetchConfig()
      .then((config) => {
        if (!active || !config.shell_root) {
          return;
        }
        setDefaultShellRoot(config.shell_root);
        setShellRoots((roots) =>
          roots.includes(config.shell_root)
            ? roots
            : [config.shell_root, ...roots],
        );
        setShellRoot((current) => current || config.shell_root);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!catalogReady || workflows.length === 0) {
      return;
    }
    if (!activeWorkflowId) {
      navigate(workflowPath(workflows[0]!.id), { replace: true });
    }
  }, [
    activeWorkflowId,
    catalogReady,
    navigate,
    workflows,
  ]);

  useEffect(() => {
    if (!catalogReady) {
      return;
    }
    if (!activeWorkflowId) {
      setLoading(false);
      setFlowNodes([]);
      setCanvasEdges([]);
      setRunHistory([]);
      return;
    }

    let active = true;
    setLoading(true);
    setError("");
    setSelectedNodeId(null);
    setExpandedGroups(new Set());
    setFlowNodes([]);
    setCanvasEdges([]);
    setRunHistory([]);
    setSelectedRunFullId("");

    async function loadWorkflow() {
      let executions: WorkflowExecution[] = [];
      let executionError = "";
      try {
        executions = await fetchWorkflowExecutions(activeWorkflowId, 20);
      } catch (caught) {
        executionError =
          caught instanceof Error ? caught.message : String(caught);
      }

      let parsed: ParsedWorkflow = parseBlueprintYAML(
        workflowSpecs[activeWorkflowId] || "",
        workflowSpecs,
      );
      if (parsed.nodes.length === 0 && executions[0]) {
        parsed = parseSnapshot(executions[0]);
      }
      const laidOut = layoutWorkflow(parsed);

      const runs = await Promise.all(
        executions.map(async (execution) => {
          try {
            const nodeExecutions = await fetchNodeExecutions(
              execution.id,
              200,
            );
            return buildRunEntry(execution, nodeExecutions);
          } catch {
            return buildRunEntry(execution, []);
          }
        }),
      );

      if (!active) {
        return;
      }

      setFlowNodes(toFlowNodes(laidOut.nodes));
      setCanvasEdges(laidOut.edges);
      setRunHistory(runs);

      const requestedRun = new URLSearchParams(
        window.location.search,
      ).get("run");
      const focusedRun =
        runs.find(
          (run) =>
            run.fullId === requestedRun || run.id === requestedRun,
        ) ??
        runs[0] ??
        null;
      setSelectedRunFullId(focusedRun?.fullId ?? "");
      if (focusedRun) {
        navigate(workflowPath(activeWorkflowId, focusedRun.id), {
          replace: true,
        });
      } else if (requestedRun) {
        navigate(workflowPath(activeWorkflowId), { replace: true });
      }

      if (executionError) {
        setError(executionError);
      } else if (laidOut.nodes.length === 0) {
        setError("The workflow spec contains no renderable nodes.");
      }
      setLoading(false);
    }

    void loadWorkflow().catch((caught: unknown) => {
      if (!active) {
        return;
      }
      setError(caught instanceof Error ? caught.message : String(caught));
      setLoading(false);
    });

    return () => {
      active = false;
    };
  }, [
    activeWorkflowId,
    catalogReady,
    navigate,
    workflowSpecs,
  ]);

  useEffect(() => {
    if (!activeWorkflowId || runHistory.length === 0) {
      return;
    }
    const requestedRun = new URLSearchParams(location.search).get("run");
    if (!requestedRun) {
      return;
    }
    const run = runHistory.find(
      (item) =>
        item.fullId === requestedRun || item.id === requestedRun,
    );
    if (run && run.fullId !== selectedRunFullId) {
      setSelectedRunFullId(run.fullId);
    }
  }, [
    activeWorkflowId,
    location.search,
    runHistory,
    selectedRunFullId,
  ]);

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
  }, [activeWorkflowId, flowNodes.length]);

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
        const sourceStatus = currentRun?.nodes[edge.from]?.status;
        const targetStatus = currentRun?.nodes[edge.to]?.status;
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
            wireStyle,
          },
        };
      });
  }, [
    canvasEdges,
    currentRun,
    nodesById,
    visibleWorkflowNodes,
    wireStyle,
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

  const selectRun = useCallback(
    (fullId: string) => {
      const run = runHistory.find((item) => item.fullId === fullId);
      if (!run || !activeWorkflowId) {
        return;
      }
      setSelectedRunFullId(fullId);
      navigate(workflowPath(activeWorkflowId, run.id), {
        replace: true,
      });
    },
    [activeWorkflowId, navigate, runHistory],
  );

  const viewRuns = useCallback(() => {
    navigate("/runs");
  }, [navigate]);

  useEffect(() => {
    if (!running || !activeWorkflowId || !liveExecutionId) {
      return;
    }

    let active = true;
    let polling = false;

    async function poll() {
      if (polling) {
        return;
      }
      polling = true;
      try {
        const recent = await fetchWorkflowExecutions(
          activeWorkflowId,
          5,
        );
        const execution =
          recent.find((item) => item.id === liveExecutionId) ??
          (await fetchWorkflowExecution(liveExecutionId));
        const nodeExecutions = await fetchNodeExecutions(
          execution.id,
          200,
        );
        const entry = buildRunEntry(execution, nodeExecutions);

        if (!active) {
          return;
        }
        setRunHistory((history) => {
          const present = history.some(
            (run) => run.fullId === entry.fullId,
          );
          return present
            ? history.map((run) =>
                run.fullId === entry.fullId ? entry : run,
              )
            : [entry, ...history].slice(0, 20);
        });
        setSelectedRunFullId(entry.fullId);
        navigate(workflowPath(activeWorkflowId, entry.id), {
          replace: true,
        });

        if (isTerminalStatus(execution.status)) {
          setRunning(false);
          setLiveExecutionId("");
          if (TERMINAL_ERROR_STATUSES.has(execution.status)) {
            const failedNode = nodeExecutions.find(
              (node) =>
                node.status === "failed" ||
                node.status === "blocked" ||
                node.status === "canceled",
            );
            const failure = failedNode?.trace?.error;
            if (failure) {
              setError(
                `${failedNode.node_id}: ${
                  typeof failure === "string"
                    ? failure
                    : JSON.stringify(failure)
                }`,
              );
            }
          }
        }
      } catch {
        // A transient poll failure is retried on the next interval.
      } finally {
        polling = false;
      }
    }

    void poll();
    const interval = window.setInterval(() => void poll(), 2500);
    return () => {
      active = false;
      window.clearInterval(interval);
    };
  }, [
    activeWorkflowId,
    liveExecutionId,
    navigate,
    running,
  ]);

  const startRun = useCallback(
    async (input: Record<string, unknown>) => {
      if (running || !activeWorkflowId) {
        return;
      }
      setShowRunModal(false);
      setRunning(true);
      setError("");

      try {
        const response = await createWorkflowExecution({
          workflow_id: activeWorkflowId,
          input,
          shell_root: shellRoot.trim() || undefined,
        });
        if (!response.execution_id) {
          throw new Error("The server did not return an execution ID.");
        }
        setLiveExecutionId(response.execution_id);
      } catch (caught) {
        setRunning(false);
        setLiveExecutionId("");
        setError(caught instanceof Error ? caught.message : String(caught));
      }
    },
    [activeWorkflowId, running, shellRoot],
  );

  const onNodeClick: NodeMouseHandler<BuilderFlowNode> = (
    _event,
    node,
  ) => {
    if (node.type === "workflow") {
      setSelectedNodeId(node.id);
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
    if (canvas.isGroup) {
      toggleGroup(canvas.id);
      return;
    }
    const linkedWorkflow = canvas.body.find(
      (field) => field.k === "workflow",
    )?.v;
    if (
      linkedWorkflow &&
      workflows.some((workflow) => workflow.id === linkedWorkflow)
    ) {
      navigate(workflowPath(linkedWorkflow));
    }
  };

  const filteredWorkflows = workflows.filter((workflow) => {
    const query = workflowSearch.trim().toLowerCase();
    if (!query) {
      return true;
    }
    return [workflow.id, workflow.name, workflow.description ?? ""].some(
      (value) => value.toLowerCase().includes(query),
    );
  });

  const filteredLibrary = NODE_LIBRARY.map((section) => ({
    ...section,
    items: section.items.filter((item) => {
      const query = nodeSearch.trim().toLowerCase();
      return (
        !query ||
        item.name.toLowerCase().includes(query) ||
        item.sub.toLowerCase().includes(query)
      );
    }),
  })).filter((section) => section.items.length > 0);

  const runtime = useMemo(
    () => ({
      currentRun,
      expandedGroups,
      runs: runHistory,
      onSelectRun: selectRun,
      onToggleGroup: toggleGroup,
      onViewRuns: viewRuns,
    }),
    [
      currentRun,
      expandedGroups,
      runHistory,
      selectRun,
      toggleGroup,
      viewRuns,
    ],
  );

  function nextWireStyle(event: MouseEvent<HTMLButtonElement>) {
    event.stopPropagation();
    setWireStyle((current) =>
      current === "bezier"
        ? "orthogonal"
        : current === "orthogonal"
          ? "straight"
          : "bezier",
    );
  }

  return (
    <BuilderRuntimeProvider value={runtime}>
      <div className="builder-app">
        <header className="builder-topbar">
          <div className="builder-brand">
            <div className="builder-logo" aria-hidden="true">
              <svg
                width="14"
                height="14"
                viewBox="0 0 14 14"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.6"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <circle cx="3.5" cy="3.5" r="1.6" />
                <circle cx="10.5" cy="7" r="1.6" />
                <circle cx="3.5" cy="10.5" r="1.6" />
                <path d="M5 4.2 L9 6.3 M5 9.8 L9 7.7" />
              </svg>
            </div>
            <b>agc</b>
          </div>
          <div className="builder-crumbs">
            <span>Workflows</span>
            <span>/</span>
            <b>{activeWorkflowId || "none"}</b>
          </div>
          <div className="builder-spacer" />
          <ShellRootPicker
            value={shellRoot}
            options={shellRoots}
            defaultRoot={defaultShellRoot}
            onChange={setShellRoot}
            onAddOption={(value) =>
              setShellRoots((roots) =>
                roots.includes(value) ? roots : [...roots, value],
              )
            }
            onRemoveOption={(value) => {
              setShellRoots((roots) =>
                roots.filter((root) => root !== value),
              );
              if (shellRoot === value) {
                setShellRoot(defaultShellRoot);
              }
            }}
            disabled={running}
          />
          {currentRun && (
            <RunMenuButton
              run={currentRun}
              runs={runHistory}
              onPick={selectRun}
              onViewAll={viewRuns}
            />
          )}
          <button
            type="button"
            className={`builder-run-button ${running ? "running" : ""}`}
            disabled={!activeWorkflowId || running}
            onClick={() => setShowRunModal(true)}
          >
            {running ? (
              <>
                <StopIcon /> Running…
              </>
            ) : (
              <>
                <PlayIcon /> Run workflow
              </>
            )}
          </button>
        </header>

        <aside className="builder-rail">
          <button
            type="button"
            className={drawer === "workflows" ? "active" : ""}
            onClick={() => {
              setDrawer("workflows");
              void refreshCatalog();
            }}
            title="Workflows"
          >
            <StackIcon />
          </button>
          <button
            type="button"
            className={drawer === "nodes" ? "active" : ""}
            onClick={() => setDrawer("nodes")}
            title="Node library"
          >
            <BlocksIcon />
          </button>
          <button type="button" disabled title="Triggers (coming soon)">
            <BoltIcon />
          </button>
          <div className="builder-rail__divider" />
          <button
            type="button"
            title="History"
            onClick={() => navigate("/runs")}
          >
            <HistoryIcon />
          </button>
          <div className="builder-spacer" />
          <button type="button" disabled title="Settings (coming soon)">
            <CogIcon />
          </button>
        </aside>

        <aside className="builder-drawer scrollnice">
          {drawer === "workflows" ? (
            <>
              <div className="builder-drawer__head">
                <h3>Workflows</h3>
              </div>
              <div className="builder-drawer__search">
                <input
                  className="builder-input"
                  placeholder="Search workflows…"
                  value={workflowSearch}
                  onChange={(event) =>
                    setWorkflowSearch(event.target.value)
                  }
                />
              </div>
              <div className="builder-workflow-list">
                {!catalogReady && (
                  <div className="builder-drawer__message">Loading…</div>
                )}
                {catalogReady && filteredWorkflows.length === 0 && (
                  <div className="builder-drawer__message">
                    No workflows found
                  </div>
                )}
                {filteredWorkflows.map((workflow) => (
                  <button
                    type="button"
                    key={workflow.id}
                    className={
                      workflow.id === activeWorkflowId ? "active" : ""
                    }
                    onClick={() =>
                      navigate(workflowPath(workflow.id))
                    }
                    title={workflow.description || workflow.id}
                  >
                    <span className="builder-workflow-list__dot" />
                    <span>{workflow.name || workflow.id}</span>
                  </button>
                ))}
              </div>
              <div className="builder-drawer__divider" />
              <div className="builder-drawer__foot">
                {workflows.length} workflow
                {workflows.length === 1 ? "" : "s"}
              </div>
            </>
          ) : (
            <>
              <div className="builder-drawer__head">
                <h3>Node library</h3>
                <span className="mono">drag → canvas</span>
              </div>
              <div className="builder-drawer__search">
                <input
                  className="builder-input"
                  placeholder="Search nodes…"
                  value={nodeSearch}
                  onChange={(event) => setNodeSearch(event.target.value)}
                />
              </div>
              <div className="builder-library">
                {filteredLibrary.map((section) => (
                  <div key={section.section}>
                    <h4>{section.section}</h4>
                    {section.items.map((item) => {
                      const visual = KIND_VISUAL[item.kind];
                      return (
                        <div
                          key={item.name}
                          className="builder-library__item"
                          draggable
                        >
                          <span
                            className="builder-library__icon"
                            style={{
                              background: visual.background,
                              color: visual.foreground,
                            }}
                          >
                            <KindIcon kind={item.kind} size={12} />
                          </span>
                          <span>
                            <b>{item.name}</b>
                            <small>{item.sub}</small>
                          </span>
                        </div>
                      );
                    })}
                  </div>
                ))}
              </div>
            </>
          )}
        </aside>

        <main className="builder-canvas">
          {error && <div className="builder-error">{error}</div>}
          {!loading && renderedNodes.length === 0 && (
            <div className="builder-canvas__empty">
              <StackIcon size={40} />
              <b>
                {workflows.length === 0
                  ? "No workflows found"
                  : "No nodes found for this workflow"}
              </b>
              <span>
                Install or select a workflow to render its graph here.
              </span>
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
            onPaneClick={() => setSelectedNodeId(null)}
            minZoom={0.4}
            maxZoom={1.6}
            nodesConnectable={false}
            edgesReconnectable={false}
            deleteKeyCode={null}
            fitView
            fitViewOptions={{ padding: 0.16, maxZoom: 1.1 }}
            proOptions={{ hideAttribution: true }}
          >
            {showGrid && (
              <Background
                variant={BackgroundVariant.Dots}
                gap={22}
                size={1}
                color="oklch(0.85 0.008 270)"
              />
            )}
            <Controls
              position="top-right"
              showInteractive={false}
              fitViewOptions={{ padding: 0.16, maxZoom: 1.1 }}
            />
            <Panel
              position="top-right"
              className="builder-display-tools"
            >
              <button type="button" onClick={nextWireStyle}>
                Wire: {wireStyle}
              </button>
              <button
                type="button"
                onClick={() => setShowGrid((value) => !value)}
              >
                Grid: {showGrid ? "on" : "off"}
              </button>
              <button
                type="button"
                onClick={() => setShowMiniMap((value) => !value)}
              >
                Map: {showMiniMap ? "on" : "off"}
              </button>
            </Panel>
            {showMiniMap && (
              <MiniMap<BuilderFlowNode>
                position="bottom-left"
                pannable
                zoomable
                nodeBorderRadius={2}
                nodeStrokeWidth={1}
                nodeColor={(node) => {
                  if (node.type === "groupBox") {
                    return "transparent";
                  }
                  const status = currentRun?.nodes[node.id]?.status;
                  if (status === "run") {
                    return "var(--accent)";
                  }
                  if (status === "ok") {
                    return "var(--st-ok)";
                  }
                  if (status === "err") {
                    return "var(--st-err)";
                  }
                  return "var(--ink-4)";
                }}
              />
            )}
          </ReactFlow>
        </main>

        <aside className="builder-right">
          <Inspector
            node={selectedNode}
            currentRun={currentRun}
            runs={runHistory}
            onSelectRun={selectRun}
            onViewRuns={viewRuns}
          />
        </aside>

        {showRunModal && activeWorkflow && (
          <RunInputModal
            workflowId={activeWorkflow.id}
            inputDefinitions={activeWorkflow.inputs}
            onRun={(input) => void startRun(input)}
            onClose={() => setShowRunModal(false)}
          />
        )}
      </div>
    </BuilderRuntimeProvider>
  );
}
