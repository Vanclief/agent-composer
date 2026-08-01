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
  discardWorkflowDraft,
  fetchWorkflowSpecs,
  fetchWorkflows,
  saveWorkflowDraft,
  updateWorkflowNode,
} from "../api";
import { parseBlueprintYAML } from "../api/blueprints";
import type { WorkflowSummary } from "../types/api";
import type { ParsedWorkflow } from "../types/workflow";
import { PlayIcon } from "./Icons";
import { ModeToggle } from "../nav/ModeToggle";
import { SettingsRailButton } from "../nav/SettingsButton";
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
  // Unsaved composer proposals, keyed by workflow id. A draft is what
  // the canvas shows until it is saved or discarded.
  const [workflowDrafts, setWorkflowDrafts] = useState<
    Record<string, string>
  >({});
  const [savingDraft, setSavingDraft] = useState(false);
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
    const next = await fetchWorkflowSpecs(nextWorkflows, signal);
    setWorkflows(nextWorkflows);
    setWorkflowSpecs(next.specs);
    setWorkflowDrafts(next.drafts);
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
  const activeDraft = workflowDrafts[activeWorkflowId] ?? "";
  // The canvas shows the draft when one exists — that is what Save
  // would install.
  const shownSpec = activeDraft || activeSpec;
  const parsed = useMemo(() => {
    if (!activeWorkflow || !shownSpec) {
      return EMPTY_WORKFLOW;
    }
    return parseBlueprintYAML(shownSpec, workflowSpecs);
  }, [shownSpec, activeWorkflow, workflowSpecs]);
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
    // The proposal landed as a draft — re-read, then follow it to its
    // workflow (a created one shows up as draft-only in the list).
    void loadWorkflows().then(() => {
      if (result.workflow_id && result.workflow_id !== activeWorkflowId) {
        navigate(
          `/workflow/${encodeURIComponent(result.workflow_id)}/build`,
        );
      }
    });
  }

  async function saveDraft() {
    if (!activeDraft || savingDraft) {
      return;
    }
    setSavingDraft(true);
    setError("");
    try {
      const response = await saveWorkflowDraft(activeWorkflowId);
      setWorkflowSpecs((current) => ({
        ...current,
        [activeWorkflowId]: response.spec,
      }));
      setWorkflowDrafts((current) => {
        const { [activeWorkflowId]: _saved, ...rest } = current;
        return rest;
      });
      // A first save turns a draft-only workflow into a real one.
      const nextWorkflows = await fetchWorkflows();
      setWorkflows(nextWorkflows);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    } finally {
      setSavingDraft(false);
    }
  }

  async function discardDraft() {
    if (!activeDraft || savingDraft) {
      return;
    }
    setError("");
    try {
      await discardWorkflowDraft(activeWorkflowId);
      setWorkflowDrafts((current) => {
        const { [activeWorkflowId]: _dropped, ...rest } = current;
        return rest;
      });
      if (!activeSpec) {
        // A discarded draft-only workflow is gone entirely.
        const nextWorkflows = await fetchWorkflows();
        setWorkflows(nextWorkflows);
        navigate("/build");
      }
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    }
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
            disabled={
              !activeWorkflow || starting || !activeSpec
            }
            title={
              activeWorkflow && !activeSpec
                ? "Save the draft first — runs always execute the saved version"
                : undefined
            }
            onClick={() => setShowRun(true)}
          >
            <PlayIcon /> {starting ? "Starting…" : "Run workflow"}
          </button>
        }
      />

      <LeftRail
        items={appRailItems()}
        active="workflows"
        footer={<SettingsRailButton />}
      />

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
                <b>
                  {workflow.name || workflow.id}
                  {(workflow.has_draft ||
                    workflowDrafts[workflow.id]) && (
                    <i className="builder-workflow-list__draft">
                      draft
                    </i>
                  )}
                </b>
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
        topOverlay={
          activeDraft && (
            <div className="canvas-head">
              <div className="canvas-head__title builder-draft-bar">
                <h2>Draft — not saved</h2>
                <button
                  type="button"
                  className="builder-run-button"
                  disabled={savingDraft}
                  onClick={() => void saveDraft()}
                >
                  {savingDraft ? "Saving…" : "Save"}
                </button>
                <button
                  type="button"
                  className="builder-ghost-button"
                  disabled={savingDraft}
                  onClick={() => void discardDraft()}
                >
                  Discard
                </button>
              </div>
            </div>
          )
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
        {/* Node edits write to the saved file — while a draft is on
            screen they would edit something you are not looking at,
            so the inspector goes read-only until Save or Discard. */}
        <NodeConfigPanel
          node={selectedNode}
          onSave={
            activeWorkflow && !activeDraft ? saveNodeConfig : undefined
          }
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
