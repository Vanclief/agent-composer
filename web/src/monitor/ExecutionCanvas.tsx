import {
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSearchParams } from "react-router-dom";
import { parseSnapshot } from "../api/blueprints";
import { Inspector } from "../builder/Inspector";
import { buildRunEntry, type RunEntry } from "../builder/runData";
import { WorkflowCanvas } from "../builder/WorkflowCanvas";
import { WorkflowOverview } from "../builder/WorkflowOverview";
import { RightPanel } from "../layout/RightPanel";
import { fetchTaskNodeExecutions } from "../tasks/data";
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
}: {
  execution?: WorkflowExecution;
  workflows: WorkflowSummary[];
  loading?: boolean;
  emptyTitle: string;
  emptyDescription: string;
  topOverlay?: ReactNode;
}) {
  const [parsed, setParsed] = useState<ParsedWorkflow>(EMPTY_WORKFLOW);
  const [currentRun, setCurrentRun] = useState<RunEntry | null>(null);
  const [canvasLoading, setCanvasLoading] = useState(false);
  const [error, setError] = useState("");
  // Selection lives in the URL so a reload lands on the same view:
  // ?node=<nodeId> selects a node, ?convo=<nodeId> opens its
  // conversation in place of the canvas.
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedNodeId = searchParams.get("node");
  const conversationNodeId = searchParams.get("convo");

  const updateParams = useCallback(
    (mutate: (params: URLSearchParams) => void) => {
      setSearchParams(
        (params) => {
          mutate(params);
          return params;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  const selectNode = useCallback(
    (nodeId: string | null) => {
      updateParams((params) => {
        if (nodeId) {
          params.set("node", nodeId);
        } else {
          params.delete("node");
        }
        params.delete("convo");
      });
    },
    [updateParams],
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

    fetchTaskNodeExecutions(execution.id, controller.signal)
      .then((nodes) => {
        if (active) {
          setCurrentRun(buildRunEntry(execution, nodes));
        }
      })
      .catch((caught: unknown) => {
        if (!active) {
          return;
        }
        setCurrentRun(buildRunEntry(execution, []));
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

  const workflowId = execution?.workflow_id ?? "";
  const activeWorkflow = useMemo<WorkflowSummary | undefined>(() => {
    if (!workflowId) {
      return undefined;
    }
    return (
      workflows.find((workflow) => workflow.id === workflowId) ?? {
        id: workflowId,
        name: workflowId,
        inputs: {},
        outputs: {},
      }
    );
  }, [workflowId, workflows]);

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

  function openConversation(nodeId: string) {
    const target = parsed.nodes.find((node) => node.id === nodeId);
    // Only LLM nodes hold a conversation — connectors and other
    // transforms have nothing to show.
    if (
      target?.kind === "llm" &&
      currentRun?.nodes[nodeId]?.nodeExecutionId
    ) {
      updateParams((params) => {
        params.set("node", nodeId);
        params.set("convo", nodeId);
      });
    }
  }

  return (
    <>
      {conversationNode && conversationExecutionId ? (
        <ConversationView
          nodeName={conversationNode.name}
          nodeExecutionId={conversationExecutionId}
          onBack={() =>
            updateParams((params) => params.delete("convo"))
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
          onSelectNode={selectNode}
          onSelectRun={() => undefined}
          onOpenNode={openConversation}
          emptyTitle={emptyTitle}
          emptyDescription={emptyDescription}
          topOverlay={topOverlay}
        />
      )}

      <RightPanel>
        {selectedNode ? (
          <Inspector
            node={selectedNode}
            currentRun={currentRun}
            runs={runs}
            onSelectRun={() => undefined}
          />
        ) : (
          <WorkflowOverview
            workflow={activeWorkflow}
            runs={runs}
            currentRun={currentRun}
            onSelectRun={() => undefined}
            onSelectNode={selectNode}
          />
        )}
      </RightPanel>
    </>
  );
}
