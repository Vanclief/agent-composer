import {
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import { PlayIcon } from "../builder/Icons";
import {
  CopyButton,
  formatValue,
} from "../builder/Inspector";
import { TopBar } from "../layout/TopBar";
import { LeftRail } from "../layout/LeftRail";
import { appRailItems } from "../layout/appRail";
import { ExecutionCanvas } from "./ExecutionCanvas";
import { LeftPanel } from "../layout/LeftPanel";
import { RunInputModal } from "../builder/RunInputModal";
import { RunLocation } from "../builder/RunLocation";
import { ModeToggle } from "../nav/ModeToggle";
import { SettingsRailButton } from "../nav/SettingsButton";
import { useLaunchLocation } from "../builder/useLaunchLocation";
import type { WorkflowSummary } from "../types/api";
import {
  fetchTaskConsoleData,
  executionToTask,
  startTask,
  type Task,
  taskOneLine,
} from "../tasks/data";

const POLL_MS = 5000;
const ACTIVE_STATUSES = new Set(["queued", "running"]);
const RUNNING_STATUSES = new Set(["queued", "running"]);
const FAILED_STATUSES = new Set(["failed", "blocked", "canceled"]);

type TaskFilter = "all" | "running" | "succeeded" | "failed";

function taskTimestamp(task: Task) {
  return Date.parse(task.finishedAt || task.startedAt || "") || 0;
}

function formatAbsoluteTime(value?: string) {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : date.toLocaleString();
}

function formatHours(milliseconds: number) {
  const hours = milliseconds / 3_600_000;
  if (hours < 1) {
    return `${Math.max(1, Math.round(milliseconds / 60_000))}m`;
  }
  if (hours < 48) {
    return `${hours.toFixed(1)}h`;
  }
  // Zombie rows can sit "running" for months; hours stops being readable.
  return `${Math.round(hours / 24)}d`;
}

/**
 * Status and duration for anything that ran — a task or one of the
 * workflow executions inside it. Derived from real timestamps only;
 * the status is always the backend's, verbatim.
 */
function runMetrics(subject: {
  status: string;
  startedAt?: string;
  finishedAt?: string;
}) {
  const started = Date.parse(subject.startedAt || "");
  const finished = subject.finishedAt
    ? Date.parse(subject.finishedAt)
    : Date.now();
  const elapsed =
    Number.isNaN(started) || Number.isNaN(finished)
      ? 0
      : Math.max(0, finished - started);

  return {
    status: subject.status,
    duration: elapsed ? formatHours(elapsed) : "—",
  };
}

function taskMetrics(task: Task) {
  return runMetrics({
    status: task.status,
    startedAt: task.startedAt,
    finishedAt: task.finishedAt,
  });
}

function ActivityTaskRow({
  task,
  workflowNames,
  selected,
  onSelect,
}: {
  task: Task;
  workflowNames: Map<string, string>;
  selected: boolean;
  onSelect: () => void;
}) {
  const active = ACTIVE_STATUSES.has(task.status);
  const metrics = taskMetrics(task);
  const extraWorkflows = task.workflowIds.length - 1;
  return (
    <button
      type="button"
      className={[
        "task-row",
        active ? "task-row--running" : "",
        selected ? "task-row--selected" : "",
      ]
        .filter(Boolean)
        .join(" ")}
      aria-pressed={selected}
      onClick={onSelect}
      title={task.title}
    >
      <span
        className={
          active
            ? "task-row__pulse"
            : `task-row__status task-row__status--${task.status}`
        }
      />
      <span className="task-row__name">
        <span className="task-row__title">
          <b>{task.title}</b>
          {extraWorkflows > 0 && (
            <span className="task-row__chain">+{extraWorkflows}</span>
          )}
        </span>
        <small className={`task-row__meta task-row__meta--${task.status}`}>
          {task.status}
          <span>{metrics.duration}</span>
        </small>
      </span>
    </button>
  );
}

function TaskDetail({
  task,
  workflowNames,
  onOpenWorkflows,
}: {
  task: Task;
  workflowNames: Map<string, string>;
  onOpenWorkflows: (executionId?: string) => void;
}) {
  const input = formatValue(task.input);
  const metrics = taskMetrics(task);
  let output = formatValue(task.output);
  if (output === "—" && ACTIVE_STATUSES.has(task.status)) {
    output = "Waiting for output…";
  } else if (output === "—" && FAILED_STATUSES.has(task.status)) {
    output = taskOneLine(task) || "Execution failed";
  }

  return (
    <article className="task-detail">
      <header className="task-detail__header">
        <div className="task-detail__heading">
          <span className="task-detail__eyebrow">Task detail</span>
          <h1>{task.title}</h1>
          <div className="task-detail__identity">
            <span className={`task-status-pill task-status-pill--${task.status}`}>
              <span />
              {task.status}
            </span>
          </div>
        </div>
      </header>

      <section className="task-detail__facts" aria-label="Task facts">
        <div>
          <span>Started</span>
          <b>{formatAbsoluteTime(task.startedAt)}</b>
        </div>
        <div>
          <span>Finished</span>
          <b>{formatAbsoluteTime(task.finishedAt)}</b>
        </div>
        <div>
          <span>Duration</span>
          <b>{metrics.duration}</b>
        </div>
        <div>
          <span>Task ID</span>
          <span className="task-detail__id">
            <b className="mono">{task.id}</b>
            <CopyButton value={task.id} />
          </span>
        </div>
      </section>

      <section className="task-detail__section">
        <div className="task-detail__section-head">
          <h2>Workflows</h2>
          <span>{task.executions.length}</span>
          <button
            type="button"
            className="builder-view-button task-detail__monitor"
            onClick={() => onOpenWorkflows()}
          >
            Open in canvas →
          </button>
        </div>
        <div className="task-detail__workflows">
          <div
            className="task-detail__workflow task-detail__workflow--header"
            aria-hidden="true"
          >
            <span />
            <span>Workflow</span>
            <span>Status</span>
            <span className="task-row__cell--num">Duration</span>
          </div>
          {task.executions.map((execution, index) => {
            const workflowId =
              execution.workflow_id ||
              task.workflowIds[index] ||
              "unknown-workflow";
            const workflowName =
              workflowNames.get(workflowId) || workflowId;
            const metrics = runMetrics({
              status: execution.status,
              startedAt: execution.started_at || execution.created_at,
              finishedAt: execution.finished_at,
            });
            return (
              <button
                type="button"
                key={execution.id}
                title={`Open ${workflowName} in the canvas`}
                onClick={() => onOpenWorkflows(execution.id)}
                className="task-detail__workflow task-detail__workflow--clickable"
              >
                <span
                  className={`task-row__status task-row__status--${metrics.status}`}
                />
                <span className="task-detail__workflow-name">
                  <b>{workflowName}</b>
                  <small className="mono">{execution.id}</small>
                </span>
                <span className={`task-row__cell--${metrics.status}`}>
                  {metrics.status}
                </span>
                <span className="task-row__cell--num">
                  {metrics.duration}
                </span>
              </button>
            );
          })}
        </div>
      </section>

      <div className="task-detail__io-grid">
        <section className="task-detail__section">
          <div className="task-detail__section-head">
            <h2>Input</h2>
            <CopyButton value={input} />
          </div>
          <pre className="task-detail__io">{input}</pre>
        </section>
        <section className="task-detail__section">
          <div className="task-detail__section-head">
            <h2>Output</h2>
            <CopyButton value={output} />
          </div>
          <pre
            className={[
              "task-detail__io",
              FAILED_STATUSES.has(task.status)
                ? "task-detail__io--error"
                : "task-detail__io--output",
            ].join(" ")}
          >
            {output}
          </pre>
        </section>
      </div>
    </article>
  );
}

export function MainMonitorPage({
  view,
}: {
  view: "tasks" | "workflows";
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const [tasks, setTasks] = useState<Task[]>([]);
  const [workflows, setWorkflows] = useState<WorkflowSummary[]>([]);
  const [ready, setReady] = useState(false);
  const [error, setError] = useState("");
  const [taskFilter, setTaskFilter] = useState<TaskFilter>("all");
  const [taskQuery, setTaskQuery] = useState("");
  // Everything you navigated to lives in the URL, so a reload lands
  // on the same view. Runs are proper paths —
  // /workflows/:workflowId/executions/:executionId[/node/:nodeId] —
  // while ?scope limits the workflows list to one task's runs.
  // Legacy ?execution links canonicalize once the run list is known.
  const {
    workflowId: routeWorkflowId = "",
    executionId: routeExecutionId = "",
  } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const legacyExecutionId = searchParams.get("execution") ?? "";
  const selectedExecutionId = routeExecutionId || legacyExecutionId;
  const scopeTaskId = searchParams.get("scope") ?? "";

  const patchParams = (
    mutate: (params: URLSearchParams) => void,
  ) => {
    setSearchParams(
      (params) => {
        mutate(params);
        return params;
      },
      { replace: true },
    );
  };

  // Legacy links used /?view=workflows before the view was a route.
  useEffect(() => {
    if (view === "tasks" && searchParams.get("view") === "workflows") {
      const params = new URLSearchParams(searchParams);
      params.delete("view");
      const legacyTask = params.get("task");
      if (legacyTask) {
        params.delete("task");
        params.set("execution", legacyTask);
      }
      navigate(
        { pathname: "/workflows", search: `?${params.toString()}` },
        { replace: true },
      );
    }
  }, [navigate, searchParams, view]);
  const [showNewTask, setShowNewTask] = useState(false);
  const [newTaskWorkflowId, setNewTaskWorkflowId] = useState("");
  const [starting, setStarting] = useState(false);
  const { shellRoot, worktree, locationSlot } =
    useLaunchLocation(starting);

  useEffect(() => {
    document.title =
      view === "workflows" ? "AGC — Workflows" : "AGC — Tasks";
    let active = true;

    async function load() {
      try {
        const data = await fetchTaskConsoleData(50);
        if (!active) {
          return;
        }
        setTasks(data.tasks);
        setWorkflows(data.workflows);
        setError("");
      } catch (caught) {
        if (!active) {
          return;
        }
        setError(caught instanceof Error ? caught.message : String(caught));
      } finally {
        if (active) {
          setReady(true);
        }
      }
    }

    void load();
    const interval = window.setInterval(() => void load(), POLL_MS);
    return () => {
      active = false;
      window.clearInterval(interval);
    };
  }, [view]);

  useEffect(() => {
    const state = location.state as {
      newTaskWorkflowId?: string;
    } | null;
    if (!state?.newTaskWorkflowId || workflows.length === 0) {
      return;
    }
    const workflow = workflows.find(
      (item) => item.id === state.newTaskWorkflowId,
    );
    if (workflow) {
      setNewTaskWorkflowId(workflow.id);
      setShowNewTask(true);
    }
    navigate(location.pathname + location.search, {
      replace: true,
      state: null,
    });
  }, [
    location.pathname,
    location.search,
    location.state,
    navigate,
    workflows,
  ]);

  const workflowNames = useMemo(
    () =>
      new Map(
        workflows.map((workflow) => [
          workflow.id,
          workflow.name || workflow.id,
        ]),
    ),
    [workflows],
  );
  // A task groups executions; the workflows view flattens them so each
  // workflow run is its own row.
  const workflowRows = useMemo(
    () =>
      tasks.flatMap((task) =>
        task.executions.map((execution) =>
          executionToTask(execution, workflowNames),
        ),
      ),
    [tasks, workflowNames],
  );
  const scopeTask = scopeTaskId
    ? tasks.find((task) => task.id === scopeTaskId)
    : undefined;
  const rows =
    view === "tasks"
      ? tasks
      : scopeTask
        ? scopeTask.executions.map((execution) =>
            executionToTask(execution, workflowNames),
          )
        : workflowRows;
  const orderedTasks = useMemo(
    () =>
      [...rows].sort((left, right) => {
        const activityRank =
          Number(!ACTIVE_STATUSES.has(left.status)) -
          Number(!ACTIVE_STATUSES.has(right.status));
        return activityRank || taskTimestamp(right) - taskTimestamp(left);
      }),
    [rows],
  );
  const filteredTasks = useMemo(() => {
    const query = taskQuery.trim().toLocaleLowerCase();
    return orderedTasks.filter((task) => {
      const matchesStatus =
        taskFilter === "all" ||
        (taskFilter === "running" &&
          RUNNING_STATUSES.has(task.status)) ||
        (taskFilter === "failed" &&
          FAILED_STATUSES.has(task.status)) ||
        task.status === taskFilter;
      if (!matchesStatus) {
        return false;
      }
      if (!query) {
        return true;
      }
      const workflowLabels = task.workflowIds.map(
        (workflowId) =>
          workflowNames.get(workflowId) || workflowId,
      );
      return [
        task.title,
        task.id,
        ...task.workflowIds,
        ...workflowLabels,
      ]
        .join(" ")
        .toLocaleLowerCase()
        .includes(query);
    });
  }, [orderedTasks, taskFilter, taskQuery, workflowNames]);
  const selectedTask =
    rows.find((task) => task.id === selectedExecutionId) || null;
  const runShellRoot =
    view === "workflows"
      ? selectedTask?.executions[0]?.shell_root
      : undefined;
  // Edit mode opens on the workflow of the run you are looking at —
  // resolved through the library by permanent uuid (slug only for
  // pre-uuid history), so deleted workflows offer nothing to edit and
  // a recycled slug never points at an unrelated workflow.
  const selectedExecution =
    view === "workflows" ? selectedTask?.executions[0] : undefined;
  const selectedWorkflowId = selectedExecution
    ? workflows.find((workflow) =>
        selectedExecution.workflow_uuid && workflow.uuid
          ? workflow.uuid === selectedExecution.workflow_uuid
          : workflow.id === selectedExecution.workflow_id,
      )?.id
    : undefined;

  // Selecting a run navigates to its canonical path. The run's own
  // workflow slug names the path; "" clears back to /workflows.
  const executionPath = (executionId: string) => {
    const task = rows.find((row) => row.id === executionId);
    const workflowSlug = task?.executions[0]?.workflow_id;
    if (!workflowSlug) {
      return "/workflows";
    }
    return `/workflows/${encodeURIComponent(
      workflowSlug,
    )}/executions/${encodeURIComponent(executionId)}`;
  };
  const setSelectedExecutionId = (executionId: string) => {
    const pathname = executionId
      ? executionPath(executionId)
      : "/workflows";
    const params = new URLSearchParams(searchParams);
    params.delete("execution");
    params.delete("node");
    params.delete("convo");
    navigate(
      { pathname, search: params.toString() ? `?${params}` : "" },
      { replace: true },
    );
  };

  // Legacy ?execution links become canonical paths, keeping ?node and
  // ?convo as their path segments.
  useEffect(() => {
    if (routeExecutionId || !legacyExecutionId || !ready) {
      return;
    }
    const task = rows.find((row) => row.id === legacyExecutionId);
    const workflowSlug = task?.executions[0]?.workflow_id;
    if (!workflowSlug) {
      return;
    }
    const params = new URLSearchParams(searchParams);
    const node = params.get("node") ?? "";
    const convo = params.get("convo") ?? "";
    params.delete("execution");
    params.delete("node");
    params.delete("convo");
    let pathname = `/workflows/${encodeURIComponent(
      workflowSlug,
    )}/executions/${encodeURIComponent(legacyExecutionId)}`;
    if (node || convo) {
      pathname += `/node/${encodeURIComponent(node || convo)}`;
    }
    if (convo) {
      pathname += "/convo";
    }
    navigate(
      { pathname, search: params.toString() ? `?${params}` : "" },
      { replace: true },
    );
  }, [
    legacyExecutionId,
    navigate,
    ready,
    routeExecutionId,
    rows,
    searchParams,
  ]);

  useEffect(() => {
    if (!ready) {
      return;
    }
    if (
      selectedExecutionId &&
      filteredTasks.some((task) => task.id === selectedExecutionId)
    ) {
      return;
    }
    // A workflow-only path (/workflows/:workflowId) means "this
    // workflow's latest run"; otherwise the newest run wins.
    const preferred = routeWorkflowId
      ? filteredTasks.find(
          (task) =>
            task.executions[0]?.workflow_id === routeWorkflowId,
        )
      : undefined;
    const nextId = (preferred ?? filteredTasks[0])?.id || "";
    if (nextId !== selectedExecutionId) {
      setSelectedExecutionId(nextId);
    }
  }, [filteredTasks, ready, routeWorkflowId, selectedExecutionId]);

  // Never-saved drafts have nothing runnable — runs always execute
  // the saved version.
  const runnableWorkflows = workflows.filter(
    (workflow) => !workflow.draft_only,
  );
  const newTaskWorkflow =
    runnableWorkflows.find(
      (workflow) => workflow.id === newTaskWorkflowId,
    ) ??
    runnableWorkflows[0] ??
    null;

  function openWorkflowsFor(task: Task, executionId?: string) {
    const params = new URLSearchParams({
      execution: executionId ?? task.executions[0]?.id ?? "",
      scope: task.id,
    });
    navigate({ pathname: "/workflows", search: `?${params}` });
  }

  async function runTask(input: Record<string, unknown>) {
    if (!newTaskWorkflow || starting) {
      return;
    }
    setStarting(true);
    setError("");
    try {
      const response = await startTask(
        newTaskWorkflow.id,
        input,
        shellRoot,
        worktree,
      );
      if (!response.execution_id) {
        throw new Error("The server did not return an execution ID.");
      }
      setShowNewTask(false);
      // Watch the new run at its canonical path.
      navigate(
        `/workflows/${encodeURIComponent(
          newTaskWorkflow.id,
        )}/executions/${encodeURIComponent(response.execution_id)}`,
      );
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    } finally {
      setStarting(false);
    }
  }

  return (
    <div
      className="task-console has-rail"
      data-component="MainMonitorPage"
    >
      <TopBar
        mode={
          view === "workflows" ? (
            <ModeToggle
              mode="monitor"
              editTo={
                selectedWorkflowId
                  ? `/workflow/${encodeURIComponent(
                      selectedWorkflowId,
                    )}/build`
                  : undefined
              }
            />
          ) : undefined
        }
        context={
          runShellRoot && <RunLocation shellRoot={runShellRoot} />
        }
        actions={
          <button
            type="button"
            className="builder-run-button"
            disabled={!ready}
            onClick={() => {
              // The picker opens on the workflow you are looking at —
              // the selected run's, or the one named in the path.
              const current = selectedWorkflowId || routeWorkflowId;
              if (
                current &&
                runnableWorkflows.some(
                  (workflow) => workflow.id === current,
                )
              ) {
                setNewTaskWorkflowId(current);
              }
              setShowNewTask(true);
            }}
          >
            <PlayIcon />{" "}
            {view === "workflows" ? "Launch workflow" : "New task"}
          </button>
        }
      />

      <main
        className={`task-console__workspace ${
          view === "workflows" ? "task-console__workspace--canvas" : ""
        }`}
      >
        <LeftRail
          items={appRailItems()}
          active={view}
          footer={<SettingsRailButton />}
        />
        <LeftPanel
          header={
            <>
            <div className="task-activity__filters">
            <input
              type="search"
              aria-label="Filter tasks by name"
              placeholder="Filter by name…"
              value={taskQuery}
              onChange={(event) => setTaskQuery(event.target.value)}
            />
            <select
              aria-label="Filter tasks by status"
              value={taskFilter}
              onChange={(event) =>
                setTaskFilter(event.target.value as TaskFilter)
              }
            >
              <option value="all">All statuses</option>
              <option value="running">Running</option>
              <option value="succeeded">Succeeded</option>
              <option value="failed">Failed</option>
            </select>
            </div>
            </>
          }
        >
          {error && <div className="task-console__error">{error}</div>}
          {!ready && <div className="task-empty">Loading tasks…</div>}
          {ready && filteredTasks.length === 0 && (
            <div className="task-empty">No tasks match these filters.</div>
          )}
          {ready && filteredTasks.length > 0 && (
            <div className="task-activity__count" aria-hidden="true">
              {filteredTasks.length} of {rows.length} {view}
            </div>
          )}
            {filteredTasks.map((task) => (
              <ActivityTaskRow
                key={task.id}
                task={task}
                workflowNames={workflowNames}
                selected={task.id === selectedExecutionId}
                onSelect={() => setSelectedExecutionId(task.id)}
              />
          ))}
        </LeftPanel>

        {view === "workflows" ? (
          <ExecutionCanvas
            execution={selectedTask?.executions[0]}
            workflows={workflows}
            loading={!ready}
            emptyTitle={
              selectedTask ? "No execution snapshot" : "Nothing selected"
            }
            emptyDescription={
              selectedTask
                ? "This run has no renderable execution snapshot."
                : "Pick a workflow on the left to see its run."
            }
            topOverlay={
              scopeTask && (
                <div className="monitor-scope">
                  <span>Workflows in</span>
                  <b>{scopeTask.title}</b>
                  <button
                    type="button"
                    onClick={() =>
                      patchParams((params) => params.delete("scope"))
                    }
                    aria-label="Show all workflows"
                  >
                    ×
                  </button>
                </div>
              )
            }
          />
        ) : (
          <section
            className="task-console__detail"
            data-component="TaskDetail"
          >
            {selectedTask ? (
              <TaskDetail
                task={selectedTask}
                workflowNames={workflowNames}
                onOpenWorkflows={(executionId) =>
                  openWorkflowsFor(selectedTask, executionId)
                }
              />
            ) : (
              <div className="task-detail__empty">
                <b>Select a task</b>
                <span>Its activity details will appear here.</span>
              </div>
            )}
          </section>
        )}
      </main>

      {showNewTask && newTaskWorkflow && (
        <RunInputModal
          key={newTaskWorkflow.id}
          title={
            view === "workflows" ? "Launch workflow" : "New task"
          }
          workflowId={newTaskWorkflow.id}
          inputDefinitions={newTaskWorkflow.inputs}
          headerSlot={
            <div className="builder-field-row">
              <div className="builder-modal__field-head">
                <label htmlFor="new-task-workflow">Run</label>
              </div>
              <select
                id="new-task-workflow"
                className="builder-select task-picker__select"
                value={newTaskWorkflow.id}
                disabled={starting}
                onChange={(event) =>
                  setNewTaskWorkflowId(event.target.value)
                }
              >
                {runnableWorkflows.map((workflow) => (
                  <option key={workflow.id} value={workflow.id}>
                    {workflow.name || workflow.id}
                  </option>
                ))}
              </select>
              {newTaskWorkflow.description && (
                <small className="task-picker__hint">
                  {newTaskWorkflow.description}
                </small>
              )}
            </div>
          }
          locationSlot={locationSlot}
          onRun={(input) => void runTask(input)}
          onClose={() => setShowNewTask(false)}
        />
      )}
    </div>
  );
}
