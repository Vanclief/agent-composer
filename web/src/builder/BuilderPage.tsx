import {
  useEffect,
  useMemo,
  useState,
} from "react";
import { TopBar } from "../layout/TopBar";
import { LeftRail } from "../layout/LeftRail";
import { appRailItems } from "../layout/appRail";
import { LeftPanel } from "../layout/LeftPanel";
import { RightPanel } from "../layout/RightPanel";
import { useNavigate, useParams } from "react-router-dom";
import {
  fetchWorkflowExecutions,
  fetchWorkflowSpecs,
  fetchWorkflows,
} from "../api";
import { parseBlueprintYAML } from "../api/blueprints";
import type {
  WorkflowExecution,
  WorkflowSummary,
} from "../types/api";
import type { ParsedWorkflow } from "../types/workflow";
import { PlayIcon } from "./Icons";
import { NodeConfigPanel } from "./Inspector";
import { WorkflowCanvas } from "./WorkflowCanvas";
import { RunInputModal } from "./RunInputModal";
import { useLaunchLocation } from "./useLaunchLocation";
import { startTask } from "../tasks/data";
import {
  executionDuration,
  StatusMarker,
} from "../workflows/StatusMarker";

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
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(
    null,
  );
  const [executions, setExecutions] = useState<WorkflowExecution[]>([]);
  const [showRun, setShowRun] = useState(false);
  const [starting, setStarting] = useState(false);
  const { shellRoot, worktree, locationSlot } =
    useLaunchLocation(starting);

  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    setLoading(true);
    setError("");

    async function load() {
      try {
        const nextWorkflows = await fetchWorkflows(controller.signal);
        const nextSpecs = await fetchWorkflowSpecs(
          nextWorkflows,
          controller.signal,
        );
        if (!active) {
          return;
        }
        setWorkflows(nextWorkflows);
        setWorkflowSpecs(nextSpecs);
      } catch (caught) {
        if (!active) {
          return;
        }
        setError(caught instanceof Error ? caught.message : String(caught));
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    }

    void load();
    return () => {
      active = false;
      controller.abort();
    };
  }, []);

  useEffect(() => {
    let active = true;

    async function loadExecutions() {
      try {
        const recent = await fetchWorkflowExecutions(undefined, 60);
        if (active) {
          setExecutions(recent);
        }
      } catch {
        // The workflow list still renders without run state.
      }
    }

    void loadExecutions();
    const interval = window.setInterval(() => void loadExecutions(), 4000);
    return () => {
      active = false;
      window.clearInterval(interval);
    };
  }, []);

  // Latest run per workflow — what the list shows beside each name.
  const latestByWorkflow = useMemo(() => {
    const map = new Map<string, WorkflowExecution>();
    for (const execution of executions) {
      const seen = map.get(execution.workflow_id);
      const at = Date.parse(
        execution.started_at || execution.created_at || "",
      );
      const seenAt = seen
        ? Date.parse(seen.started_at || seen.created_at || "")
        : -1;
      if (!seen || at > seenAt) {
        map.set(execution.workflow_id, execution);
      }
    }
    return map;
  }, [executions]);

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
  const pageError =
    error ||
    (!loading && activeWorkflowId && !activeWorkflow
      ? `Workflow "${activeWorkflowId}" was not found.`
      : !loading && activeWorkflow && parsed.nodes.length === 0
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

  async function runWorkflow(input: Record<string, unknown>) {
    if (!activeWorkflow || starting) {
      return;
    }
    setStarting(true);
    setError("");
    try {
      await startTask(activeWorkflow.id, input, shellRoot, worktree);
      setShowRun(false);
      const recent = await fetchWorkflowExecutions(undefined, 60);
      setExecutions(recent);
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

      <LeftRail items={appRailItems()} active="library" />

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
              disabled
              title="Creating workflows from the UI is coming soon — add YAML to your workflows directory for now"
            >
              + New workflow
            </button>
          </>
        }
      >
        <ol className="builder-workflow-steps">
              {loading && (
                <div className="builder-drawer__message">Loading…</div>
              )}
              {!loading && filteredWorkflows.length === 0 && (
                <div className="builder-drawer__message">
                  No workflows found
                </div>
              )}
              {filteredWorkflows.map((workflow) => {
                const latest = latestByWorkflow.get(workflow.id);
                const status = latest?.status ?? "never";
                return (
                  <li
                    key={workflow.id}
                    className={[
                      "monitor-steps__stop",
                      `monitor-steps__stop--${status}`,
                      workflow.id === activeWorkflowId
                        ? "monitor-steps__stop--current"
                        : "",
                    ]
                      .filter(Boolean)
                      .join(" ")}
                  >
                    <span className="monitor-steps__rail">
                      <i className="monitor-steps__dot">
                        <StatusMarker status={status} />
                      </i>
                    </span>
                    <button
                      type="button"
                      className="monitor-steps__label"
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
                      <small>
                        {latest ? status : "never run"}
                        <span>
                          {latest ? executionDuration(latest) : ""}
                        </span>
                      </small>
                    </button>
                  </li>
                );
              })}
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
          activeWorkflowId ? "No workflow nodes" : "Nothing selected"
        }
        emptyDescription={
          activeWorkflowId
            ? "Select a valid workflow definition to render it here."
            : "Pick a workflow on the left to start building."
        }
        bottomOverlay={
          <div
            className="builder-change-composer"
            title="natural-language editing coming soon"
          >
            <input
              type="text"
              placeholder="Describe a change…"
              disabled
              aria-label="Describe a change"
            />
            <button type="button" disabled>
              Apply
            </button>
          </div>
        }
      />

      <RightPanel>
        <NodeConfigPanel node={selectedNode} />
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
