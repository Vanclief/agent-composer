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
import {
  blueprintVersion,
  parseBlueprintYAML,
} from "../api/blueprints";
import type { WorkflowSummary } from "../types/api";
import type { ParsedWorkflow } from "../types/workflow";
import { ChatIcon, PlayIcon } from "./Icons";
import { ModeToggle } from "../nav/ModeToggle";
import { SettingsRailButton } from "../nav/SettingsButton";
import {
  ComposerPanel,
  type EditResult,
} from "./ComposerPanel";
import { NewWorkflowModal } from "./NewWorkflowModal";
import { EditWorkflowModal } from "./EditWorkflowModal";
import { DeleteWorkflowModal } from "./DeleteWorkflowModal";
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
  const [showNewWorkflow, setShowNewWorkflow] = useState(false);
  const [showRename, setShowRename] = useState(false);
  const [showDelete, setShowDelete] = useState(false);
  const [starting, setStarting] = useState(false);
  // The Composer panel — open state sticks across visits.
  const [composerOpen, setComposerOpen] = useState(
    () => localStorage.getItem("agc.composer.open") !== "0",
  );
  function toggleComposer() {
    setComposerOpen((open) => {
      localStorage.setItem("agc.composer.open", open ? "0" : "1");
      return !open;
    });
  }
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
  const shownVersion = useMemo(
    () => (shownSpec ? blueprintVersion(shownSpec) : ""),
    [shownSpec],
  );
  const selectedNode =
    parsed.nodes.find((node) => node.id === selectedNodeId) ?? null;
  // A missing workflow (deleted from the registry, stale link) is an
  // empty state, not an error banner.
  const notInstalled = !loading && activeWorkflowId && !activeWorkflow;
  // A workflow with zero nodes is a legitimate state (a freshly
  // created draft), so it gets an empty state, not an error.
  const noNodes =
    !loading && Boolean(activeWorkflow) && parsed.nodes.length === 0;
  const pageError = error;

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
          `/workflows/${encodeURIComponent(
            activeWorkflow.id,
          )}/executions/${encodeURIComponent(response.execution_id)}`,
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
      className={[
        "builder-app builder-app--build has-rail",
        composerOpen ? "builder-app--composer-open" : "",
        selectedNode ? "" : "builder-app--no-inspector",
      ]
        .filter(Boolean)
        .join(" ")}
      data-component="BuilderPage"
    >
      <TopBar
        mode={<ModeToggle mode="edit" />}
        actions={
          <>
            <button
              type="button"
              className="builder-run-button"
              disabled={
                !activeWorkflow ||
                starting ||
                !activeSpec ||
                Boolean(activeDraft)
              }
              title={
                activeWorkflow && (activeDraft || !activeSpec)
                  ? "Save the draft first — runs always execute the saved version"
                  : undefined
              }
              onClick={() => setShowRun(true)}
            >
              <PlayIcon /> {starting ? "Starting…" : "Run workflow"}
            </button>
            <button
              type="button"
              className={[
                "builder-ghost-button",
                composerOpen ? "active" : "",
              ]
                .filter(Boolean)
                .join(" ")}
              title={
                composerOpen
                  ? "Close the composer"
                  : "Talk to the composer agent"
              }
              onClick={toggleComposer}
            >
              <ChatIcon /> Composer
            </button>
          </>
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
              title="Name a new workflow — it starts as a draft"
              onClick={() => setShowNewWorkflow(true)}
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
            : noNodes
              ? "No nodes yet"
              : activeWorkflowId
                ? "No workflow nodes"
                : "Nothing selected"
        }
        emptyDescription={
          notInstalled
            ? "This workflow is not in the registry anymore. Pick one on the left, or create a new one."
            : noNodes
              ? "Describe what this workflow should do in the Composer — it proposes the nodes as a draft."
              : activeWorkflowId
                ? "Select a valid workflow definition to render it here."
                : "Pick a workflow on the left, or create a new one."
        }
        topOverlay={
          activeWorkflow && (
            <div className="canvas-head">
              <div className="canvas-head__title">
                <h2>{activeWorkflow.name || activeWorkflow.id}</h2>
                {shownVersion && (
                  <span className="canvas-head__version">
                    v{shownVersion}
                  </span>
                )}
                <button
                  type="button"
                  className="canvas-head__edit"
                  title="Edit this workflow's name, id, or description"
                  onClick={() => setShowRename(true)}
                >
                  Edit
                </button>
                <button
                  type="button"
                  className="canvas-head__edit canvas-head__edit--danger"
                  title="Remove from the library — run history stays"
                  onClick={() => setShowDelete(true)}
                >
                  Delete
                </button>
              </div>
              {activeDraft && (
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
              )}
            </div>
          )
        }
      />

      {/* The Inspector only exists while a node is selected. Node
          edits write to the saved file — while a draft is on screen
          they would edit something you are not looking at, so it goes
          read-only until Save or Discard. */}
      {selectedNode && (
        <RightPanel>
          <NodeConfigPanel
            node={selectedNode}
            onSave={
              activeWorkflow && !activeDraft
                ? saveNodeConfig
                : undefined
            }
          />
        </RightPanel>
      )}

      {composerOpen && (
        <ComposerPanel
          workflowId={activeWorkflow ? activeWorkflowId : ""}
          onApplied={handleEditApplied}
          onClose={toggleComposer}
        />
      )}

      {showDelete && activeWorkflow && (
        <DeleteWorkflowModal
          workflowId={activeWorkflowId}
          name={activeWorkflow.name || ""}
          onDeleted={() => {
            setShowDelete(false);
            void loadWorkflows().then(() => navigate("/build"));
          }}
          onClose={() => setShowDelete(false)}
        />
      )}

      {showRename && activeWorkflow && (
        <EditWorkflowModal
          workflowId={activeWorkflowId}
          currentName={activeWorkflow.name || ""}
          currentDescription={activeWorkflow.description || ""}
          onRenamed={(renamedId) => {
            setShowRename(false);
            void loadWorkflows().then(() => {
              if (renamedId !== activeWorkflowId) {
                navigate(
                  `/workflow/${encodeURIComponent(renamedId)}/build`,
                );
              }
            });
          }}
          onClose={() => setShowRename(false)}
        />
      )}

      {showNewWorkflow && (
        <NewWorkflowModal
          onCreated={(newWorkflowId) => {
            setShowNewWorkflow(false);
            void loadWorkflows().then(() => {
              navigate(
                `/workflow/${encodeURIComponent(newWorkflowId)}/build`,
              );
            });
          }}
          onClose={() => setShowNewWorkflow(false)}
        />
      )}

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
