import {
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  Link,
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import {
  parseSnapshot,
  WORKFLOW_INPUTS_NODE_ID,
} from "../api/specs";
import { Inspector } from "../builder/Inspector";
import {
  buildRunEntry,
  nodeSnapshotFrom,
  type RunEntry,
} from "../builder/runData";
import { StatusPill } from "../builder/RunMenu";
import { WorkflowCanvas } from "../builder/WorkflowCanvas";
import { WorkflowOverview } from "../builder/WorkflowOverview";
import { RightPanel } from "../layout/RightPanel";
import {
  fetchTaskNodeExecutions,
  rerunFromNode,
} from "../tasks/data";
import { ConversationView } from "./ConversationView";
import type {
  WorkflowExecution,
  WorkflowSummary,
} from "../types/api";
import type { ParsedWorkflow } from "../types/workflow";

const EMPTY_WORKFLOW: ParsedWorkflow = {
  nodes: [],
  edges: [],
  order: [],
};

/**
 * One workflow execution rendered as a canvas plus its inspector.
 *
 * Owns everything that depends on the selected execution — snapshot
 * parsing, node executions, node selection — so any surface that can
 * point at an execution gets the identical view.
 */
export function ExecutionCanvas({
  execution,
  workflows,
  loading = false,
  emptyTitle,
  emptyDescription,
  topOverlay,
  onResumed,
}: {
  execution?: WorkflowExecution;
  workflows: WorkflowSummary[];
  loading?: boolean;
  emptyTitle: string;
  emptyDescription: string;
  topOverlay?: ReactNode;
  /** Called with the new execution id after "re-run from here". */
  onResumed?: (executionId: string) => void;
}) {
  const [parsed, setParsed] = useState<ParsedWorkflow>(EMPTY_WORKFLOW);
  const [currentRun, setCurrentRun] = useState<RunEntry | null>(null);
  const [canvasLoading, setCanvasLoading] = useState(false);
  const [error, setError] = useState("");
  // Selection lives in the URL path so a reload lands on the same
  // view: …/node/:nodeId selects a node, a trailing /convo opens its
  // conversation in place of the canvas.
  const navigate = useNavigate();
  const location = useLocation();
  const {
    workflowId: routeWorkflowId = "",
    executionId: routeExecutionId = "",
    nodeId: routeNodeId = "",
  } = useParams();
  const selectedNodeId = routeNodeId || null;
  const conversationNodeId =
    routeNodeId && location.pathname.endsWith("/convo")
      ? routeNodeId
      : null;

  // Legacy query-param URLs have no path base yet — selection waits
  // for the monitor's canonicalization pass.
  const goTo = useCallback(
    (suffix: string) => {
      if (!routeWorkflowId || !routeExecutionId) {
        return;
      }
      navigate(
        {
          pathname:
            `/workflows/${encodeURIComponent(routeWorkflowId)}` +
            `/executions/${encodeURIComponent(routeExecutionId)}` +
            suffix,
          search: location.search,
        },
        { replace: true },
      );
    },
    [location.search, navigate, routeExecutionId, routeWorkflowId],
  );

  const selectNode = useCallback(
    (nodeId: string | null) => {
      goTo(nodeId ? `/node/${encodeURIComponent(nodeId)}` : "");
    },
    [goTo],
  );
  // The drilled-open group lives in the query (?in=<id>), preserved
  // by goTo's navigations. Drilling drops the node selection — it
  // belongs to the view being left.
  const [searchParams] = useSearchParams();
  const focusGroupId = searchParams.get("in");
  const focusGroup = useCallback(
    (groupId: string | null) => {
      if (!routeWorkflowId || !routeExecutionId) {
        return;
      }
      const params = new URLSearchParams(location.search);
      if (groupId) {
        params.set("in", groupId);
      } else {
        params.delete("in");
      }
      navigate(
        {
          pathname:
            `/workflows/${encodeURIComponent(routeWorkflowId)}` +
            `/executions/${encodeURIComponent(routeExecutionId)}`,
          search: params.toString(),
        },
        { replace: true },
      );
    },
    [location.search, navigate, routeExecutionId, routeWorkflowId],
  );
  // The execution whose snapshot is currently parsed. Polling hands us
  // a fresh object every few seconds; only a different id may re-parse
  // the graph or reset the node selection.
  const parsedExecutionId = useRef("");

  useEffect(() => {
    if (!execution) {
      parsedExecutionId.current = "";
      setParsed(EMPTY_WORKFLOW);
      setCurrentRun(null);
      return;
    }
    const controller = new AbortController();
    let active = true;

    if (parsedExecutionId.current !== execution.id) {
      // Only an actual switch clears the selection — the first parse
      // after a reload must keep whatever the URL brought along.
      if (parsedExecutionId.current) {
        selectNode(null);
      }
      parsedExecutionId.current = execution.id;
      setCanvasLoading(true);
      setError("");

      try {
        setParsed(parseSnapshot(execution));
      } catch (caught) {
        parsedExecutionId.current = "";
        setParsed(EMPTY_WORKFLOW);
        setCurrentRun(null);
        setCanvasLoading(false);
        setError(
          caught instanceof Error ? caught.message : String(caught),
        );
        return () => {
          active = false;
          controller.abort();
        };
      }
    }

    async function loadRunData() {
      const entry = buildRunEntry(
        execution!,
        await fetchTaskNodeExecutions(execution!.id, controller.signal),
      );

      // Reused nodes are references into the source run — resolve
      // them so the canvas and inspector show the original results.
      const meta = execution!.metadata as
        | {
            resumed_from?: string;
            reused_nodes?: Record<string, string>;
          }
        | undefined;
      if (meta?.resumed_from && meta.reused_nodes) {
        const sourceNodes = await fetchTaskNodeExecutions(
          meta.resumed_from,
          controller.signal,
        );
        const byId = new Map(
          sourceNodes.map((node) => [node.id, node]),
        );
        for (const [nodeId, nodeExecutionId] of Object.entries(
          meta.reused_nodes,
        )) {
          const source = byId.get(String(nodeExecutionId));
          if (source) {
            entry.nodes[nodeId] = {
              ...nodeSnapshotFrom(source),
              reusedFrom: meta.resumed_from,
            };
          }
        }
      }

      return entry;
    }

    loadRunData()
      .then((entry) => {
        if (active) {
          setCurrentRun(entry);
        }
      })
      .catch((caught: unknown) => {
        if (!active) {
          return;
        }
        setCurrentRun(buildRunEntry(execution!, []));
        setError(caught instanceof Error ? caught.message : String(caught));
      })
      .finally(() => {
        if (active) {
          setCanvasLoading(false);
        }
      });

    return () => {
      active = false;
      controller.abort();
    };
  }, [execution, selectNode]);

  const workflowSlug = execution?.workflow_slug ?? "";
  const workflowUUID = execution?.workflow_id ?? "";
  // Editing needs the definition to still exist — runs of deleted
  // workflows render fine from their snapshot but have nothing to
  // edit. Identity is the permanent id; the slug only stands in for
  // pre-identity history, so a deleted-then-recreated slug never
  // matches.
  const activeWorkflow = useMemo<WorkflowSummary | undefined>(() => {
    if (!workflowSlug) {
      return undefined;
    }
    return workflows.find((workflow) =>
      workflowUUID && workflow.id
        ? workflow.id === workflowUUID
        : workflow.slug === workflowSlug,
    );
  }, [workflowSlug, workflowUUID, workflows]);
  const installed = activeWorkflow !== undefined;
  const shownWorkflow = activeWorkflow ?? {
    slug: workflowSlug,
    name: workflowSlug,
    inputs: {},
    outputs: {},
  };

  // The furthest node that has produced output — the overview shows it
  // when the run has no workflow-level outputs (running/failed runs).
  const lastNodeOutput = useMemo(() => {
    if (!currentRun) {
      return undefined;
    }
    for (let index = parsed.order.length - 1; index >= 0; index--) {
      const nodeId = parsed.order[index];
      if (!nodeId || nodeId === WORKFLOW_INPUTS_NODE_ID) {
        continue;
      }
      const outputs = currentRun.nodes[nodeId]?.outputSnapshot;
      if (outputs && Object.keys(outputs).length > 0) {
        return { nodeId, outputs };
      }
    }
    return undefined;
  }, [parsed, currentRun]);

  const runs = currentRun ? [currentRun] : [];
  const selectedNode =
    parsed.nodes.find((node) => node.id === selectedNodeId) ?? null;
  const canvasError =
    error ||
    (!loading && !canvasLoading && execution && parsed.nodes.length === 0
      ? "The execution snapshot contains no renderable nodes."
      : "");
  const conversationNode = conversationNodeId
    ? parsed.nodes.find(
        (node) =>
          node.id === conversationNodeId && node.kind === "llm",
      )
    : null;
  const conversationExecutionId = conversationNodeId
    ? currentRun?.nodes[conversationNodeId]?.nodeExecutionId
    : undefined;

  function openExecution(executionId: string) {
    if (onResumed) {
      onResumed(executionId);
      return;
    }
    navigate(
      {
        pathname:
          `/workflows/${encodeURIComponent(routeWorkflowId)}` +
          `/executions/${encodeURIComponent(executionId)}`,
        search: location.search,
      },
      { replace: true },
    );
  }

  async function rerunFrom(nodeId: string) {
    if (!execution) {
      return;
    }
    setError("");
    try {
      const response = await rerunFromNode(execution.id, nodeId);
      if (!response.execution_id) {
        throw new Error("The server did not return an execution ID.");
      }
      openExecution(response.execution_id);
    } catch (caught) {
      setError(
        caught instanceof Error ? caught.message : String(caught),
      );
    }
  }

  function openConversation(nodeId: string) {
    const target = parsed.nodes.find((node) => node.id === nodeId);
    // Only LLM nodes hold a conversation — connectors and other
    // transforms have nothing to show.
    if (
      target?.kind === "llm" &&
      currentRun?.nodes[nodeId]?.nodeExecutionId
    ) {
      goTo(`/node/${encodeURIComponent(nodeId)}/convo`);
    }
  }

  return (
    <>
      {conversationNode && conversationExecutionId ? (
        <ConversationView
          nodeName={conversationNode.name}
          nodeExecutionId={conversationExecutionId}
          onBack={() =>
            goTo(`/node/${encodeURIComponent(conversationNodeId ?? "")}`)
          }
        />
      ) : (
        <WorkflowCanvas
          parsed={parsed}
          loading={loading || canvasLoading}
          error={canvasError}
          currentRun={currentRun}
          runs={runs}
          readOnly
          showRunStatus
          focusGroupId={focusGroupId}
          onFocusGroup={focusGroup}
          rootCrumb={shownWorkflow.name || shownWorkflow.slug}
          onSelectNode={selectNode}
          onSelectRun={() => undefined}
          onOpenNode={openConversation}
          emptyTitle={emptyTitle}
          emptyDescription={emptyDescription}
          topOverlay={
            (workflowSlug || topOverlay) && (
              <div className="canvas-head">
                {workflowSlug && (
                  <div className="canvas-head__title">
                    <h2>
                      {shownWorkflow.name || shownWorkflow.slug}
                    </h2>
                    {execution?.workflow_version && (
                      <span className="canvas-head__version">
                        v{execution.workflow_version}
                      </span>
                    )}
                    {currentRun && (
                      <StatusPill
                        status={currentRun.status}
                        runId={currentRun.id}
                      />
                    )}
                    {installed && (
                      <Link
                        className="canvas-head__edit"
                        to={`/workflow/${encodeURIComponent(
                          shownWorkflow.slug,
                        )}/build`}
                        title="Open this workflow in the editor"
                      >
                        Edit
                      </Link>
                    )}
                  </div>
                )}
                {topOverlay}
              </div>
            )
          }
        />
      )}

      <RightPanel>
        {selectedNode ? (
          <Inspector
            node={selectedNode}
            currentRun={currentRun}
            runs={runs}
            onSelectRun={() => undefined}
            onRerunFrom={(nodeId) => void rerunFrom(nodeId)}
          />
        ) : (
          <WorkflowOverview
            workflow={workflowSlug ? shownWorkflow : undefined}
            runs={runs}
            currentRun={currentRun}
            onSelectRun={() => undefined}
            onSelectNode={selectNode}
            resumedFrom={
              (execution?.metadata as { resumed_from?: string })
                ?.resumed_from
            }
            resumeNode={
              (execution?.metadata as { resume_node?: string })
                ?.resume_node
            }
            onOpenExecution={openExecution}
            lastNodeOutput={lastNodeOutput}
          />
        )}
      </RightPanel>
    </>
  );
}
