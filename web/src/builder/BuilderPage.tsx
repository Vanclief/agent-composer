import {
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { TopBar } from "../layout/TopBar";
import { LeftRail } from "../layout/LeftRail";
import { appRailItems } from "../layout/appRail";
import { LeftPanel } from "../layout/LeftPanel";
import { RightPanel } from "../layout/RightPanel";
import {
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import {
  fetchWorkflowSpecs,
  fetchWorkflows,
  updateWorkflowNode,
} from "../api";
import { parseBlueprintYAML } from "../api/blueprints";
import type { WorkflowSummary } from "../types/api";
import type { ParsedWorkflow } from "../types/workflow";
import { PlayIcon } from "./Icons";
import { ModeToggle } from "../nav/ModeToggle";
import {
  ChangeComposer,
  type EditResult,
} from "./ChangeComposer";
import { NodeConfigPanel } from "./Inspector";
import { WorkflowCanvas } from "./WorkflowCanvas";
import { RunInputModal } from "./RunInputModal";
import { useLaunchLocation } from "./useLaunchLocation";
import { startTask } from "../tasks/data";

const EMPTY_WORKFLOW: ParsedWorkflow = {
  nodes: [],
  edges: [],
  order: [],
};

export function BuilderPage() {
  const navigate = useNavigate();
  const { workflowId = "" } = useParams();
  const activeWorkflowId = workflowId.trim();
  const [workflows, setWorkflows] = useState<WorkflowSummary[]>([]);
  const [workflowSpecs, setWorkflowSpecs] = useState<
    Record<string, string>
  >({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [workflowSearch, setWorkflowSearch] = useState("");
  // Selection lives in the URL (?node=<id>), same as the run pages.
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedNodeId = searchParams.get("node");
  const setSelectedNodeId = (nodeId: string | null) => {
    setSearchParams(
      (params) => {
        if (nodeId) {
          params.set("node", nodeId);
        } else {
          params.delete("node");
        }
        return params;
      },
      { replace: true },
    );
  };
  const [showRun, setShowRun] = useState(false);
  const [starting, setStarting] = useState(false);
  const { shellRoot, worktree, locationSlot } =
    useLaunchLocation(starting);

  const loadWorkflows = useCallback(async (signal?: AbortSignal) => {
    const nextWorkflows = await fetchWorkflows(signal);
    const nextSpecs = await fetchWorkflowSpecs(nextWorkflows, signal);
    setWorkflows(nextWorkflows);
    setWorkflowSpecs(nextSpecs);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    setLoading(true);
    setError("");

    loadWorkflows(controller.signal)
      .catch((caught: unknown) => {
        if (active) {
          setError(
            caught instanceof Error ? caught.message : String(caught),
          );
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false);
        }
      });

    return () => {
      active = false;
      controller.abort();
    };
  }, [loadWorkflows]);

  const activeWorkflow = workflows.find(
    (workflow) => workflow.id === activeWorkflowId,
  );
  const activeSpec = workflowSpecs[activeWorkflowId] ?? "";
  const parsed = useMemo(() => {
    if (!activeWorkflow || !activeSpec) {
      return EMPTY_WORKFLOW;
    }
    return parseBlueprintYAML(activeSpec, workflowSpecs);
  }, [activeSpec, activeWorkflow, workflowSpecs]);
  const selectedNode =
    parsed.nodes.find((node) => node.id === selectedNodeId) ?? null;
  // A missing workflow (deleted from the registry, stale link) is an
  // empty state, not an error banner.
  const notInstalled = !loading && activeWorkflowId && !activeWorkflow;
  const pageError =
    error ||
    (!loading && activeWorkflow && parsed.nodes.length === 0
      ? "The workflow YAML contains no renderable nodes."
      : "");

  const filteredWorkflows = workflows.filter((workflow) => {
    const query = workflowSearch.trim().toLowerCase();
    return (
      !query ||
      [workflow.id, workflow.name, workflow.description ?? ""].some(
        (value) => value.toLowerCase().includes(query),
      )
    );
  });

  useEffect(() => {
    document.title = activeWorkflow
      ? `Build ${activeWorkflow.name || activeWorkflow.id} — AGC`
      : "AGC — Build";
  }, [activeWorkflow]);

  function handleEditApplied(result: EditResult) {
    // The registry changed on disk — re-read it, then follow the edit
    // to its workflow (a created one, or a renamed target).
    void loadWorkflows().then(() => {
      if (result.workflow_id && result.workflow_id !== activeWorkflowId) {
        navigate(
          `/workflow/${encodeURIComponent(result.workflow_id)}/build`,
        );
      }
    });
  }

  async function saveNodeConfig(
    nodeName: string,
    update: { model?: string; harness?: string; instruction?: string },
  ) {
    const response = await updateWorkflowNode(
      activeWorkflowId,
      nodeName,
      update,
    );
    // The server returns the whole updated spec — the canvas and
    // inspector re-derive from it.
    setWorkflowSpecs((current) => ({
      ...current,
      [activeWorkflowId]: response.spec,
    }));
  }

  async function runWorkflow(input: Record<string, unknown>) {
    if (!activeWorkflow || starting) {
      return;
    }
    setStarting(true);
    setError("");
    try {
      const response = await startTask(
        activeWorkflow.id,
        input,
        shellRoot,
        worktree,
      );
      setShowRun(false);
      if (response.execution_id) {
        navigate(
          `/executions/${encodeURIComponent(response.execution_id)}`,
        );
      }
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    } finally {
      setStarting(false);
    }
  }

  return (
    <div
      className="builder-app builder-app--build has-rail"
      data-component="BuilderPage"
    >
      <TopBar
        title={activeWorkflow?.name || activeWorkflowId || "No workflow"}
        mode={<ModeToggle mode="edit" />}
        actions={
          <button
            type="button"
            className="builder-run-button"
            disabled={!activeWorkflow || starting}
            onClick={() => setShowRun(true)}
          >
            <PlayIcon /> {starting ? "Starting…" : "Run workflow"}
          </button>
        }
      />

      <LeftRail items={appRailItems()} active="workflows" />

      <LeftPanel
        className="builder-drawer"
        header={
          <>
              <div className="builder-drawer__head">
              <h3>Workflows</h3>
              <span className="mono">{workflows.length}</span>
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
            <button
              type="button"
              className="builder-drawer__new"
              title="Describe the workflow you want and the editor agent builds it"
              onClick={() => navigate("/build")}
            >
              + New workflow
            </button>
          </>
        }
      >
        <ol className="builder-workflow-list">
          {loading && (
            <div className="builder-drawer__message">Loading…</div>
          )}
          {!loading && filteredWorkflows.length === 0 && (
            <div className="builder-drawer__message">
              No workflows found
            </div>
          )}
          {filteredWorkflows.map((workflow) => (
            <li key={workflow.id}>
              <button
                type="button"
                className={
                  workflow.id === activeWorkflowId ? "active" : ""
                }
                title={workflow.description || workflow.id}
                onClick={() =>
                  navigate(
                    `/workflow/${encodeURIComponent(
                      workflow.id,
                    )}/build`,
                  )
                }
              >
                <b>{workflow.name || workflow.id}</b>
                {workflow.description && (
                  <small>{workflow.description}</small>
                )}
              </button>
            </li>
          ))}
        </ol>
      </LeftPanel>

      <WorkflowCanvas
        parsed={parsed}
        loading={loading}
        error={pageError}
        currentRun={null}
        runs={[]}
        readOnly={false}
        showRunStatus={false}
        onSelectNode={setSelectedNodeId}
        onSelectRun={() => undefined}
        onOpenWorkflow={(linkedWorkflowId) =>
          navigate(
            `/workflow/${encodeURIComponent(linkedWorkflowId)}/build`,
          )
        }
        emptyTitle={
          notInstalled
            ? "Not installed"
            : activeWorkflowId
              ? "No workflow nodes"
              : "Nothing selected"
        }
        emptyDescription={
          notInstalled
            ? "This workflow is not in the registry anymore. Pick one on the left, or describe a new one below."
            : activeWorkflowId
              ? "Select a valid workflow definition to render it here."
              : "Pick a workflow on the left, or describe a new one below."
        }
        bottomOverlay={
          <ChangeComposer
            key={activeWorkflowId}
            workflowId={activeWorkflow ? activeWorkflowId : ""}
            onApplied={handleEditApplied}
          />
        }
      />

      <RightPanel>
        <NodeConfigPanel
          node={selectedNode}
          onSave={activeWorkflow ? saveNodeConfig : undefined}
        />
      </RightPanel>

      {showRun && activeWorkflow && (
        <RunInputModal
          key={activeWorkflow.id}
          title={`Run ${activeWorkflow.name || activeWorkflow.id}`}
          workflowId={activeWorkflow.id}
          inputDefinitions={activeWorkflow.inputs}
          locationSlot={locationSlot}
          onRun={(input) => void runWorkflow(input)}
          onClose={() => setShowRun(false)}
        />
      )}
    </div>
  );
}
