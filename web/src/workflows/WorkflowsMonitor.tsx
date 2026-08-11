import {
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { fetchWorkflowExecutions, fetchWorkflows } from "../api";
import { StopIcon } from "../builder/Icons";
import { RunLocation } from "../builder/RunLocation";
import { TopBar } from "../layout/TopBar";
import { LeftPanel } from "../layout/LeftPanel";
import { ExecutionCanvas } from "../monitor/ExecutionCanvas";
import {
  executionDuration,
  StatusMarker,
} from "./StatusMarker";
import type {
  WorkflowExecution,
  WorkflowSummary,
} from "../types/api";
import {
  fetchTask,
  fetchTaskMonitorData,
  fetchTaskNodeExecutions,
  formatTaskElapsed,
  type Task,
} from "../tasks/data";

const LIVE_STATUSES = new Set(["queued", "running"]);
const FAILED_STATUSES = new Set(["failed", "blocked", "canceled"]);

function executionStatus(
  _task: Task,
  execution: WorkflowExecution,
) {
  return execution.status;
}

export function WorkflowsMonitor() {
  const navigate = useNavigate();
  const { executionId = "" } = useParams();
  const taskId = executionId.trim();
  // Without a task we monitor every workflow that is currently running.
  const [liveExecutions, setLiveExecutions] = useState<
    WorkflowExecution[]
  >([]);
  const [task, setTask] = useState<Task | null>(null);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [workflows, setWorkflows] = useState<WorkflowSummary[]>([]);
  const [selectedExecutionId, setSelectedExecutionId] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const currentStopRef = useRef<HTMLLIElement | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    setLoading(true);
    setError("");
    setTask(null);

    if (!taskId) {
      Promise.all([
        fetchWorkflowExecutions(undefined, 60, controller.signal),
        fetchWorkflows(controller.signal),
      ])
        .then(([executions, nextWorkflows]) => {
          if (!active) {
            return;
          }
          const live = executions.filter((execution) =>
            LIVE_STATUSES.has(execution.status),
          );
          setLiveExecutions(live);
          setWorkflows(nextWorkflows);
          setSelectedExecutionId((current) =>
            live.some((execution) => execution.id === current)
              ? current
              : (live[0]?.id ?? ""),
          );
        })
        .catch((caught: unknown) => {
          if (!active) {
            return;
          }
          setError(
            caught instanceof Error ? caught.message : String(caught),
          );
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
    }

    fetchTaskMonitorData(taskId, controller.signal)
      .then((data) => {
        if (!active) {
          return;
        }
        setTask(data.task);
        setTasks(data.tasks);
        setWorkflows(data.workflows);
        const executions = data.task.executions;
        const execution =
          executions.find((item) => item.id === taskId) ??
          executions.find((item) => item.status === "running") ??
          executions.find((item) => FAILED_STATUSES.has(item.status)) ??
          executions.find((item) => item.status === "queued") ??
          executions[executions.length - 1];
        setSelectedExecutionId(execution?.id ?? "");
      })
      .catch((caught: unknown) => {
        if (!active) {
          return;
        }
        setError(caught instanceof Error ? caught.message : String(caught));
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
  }, [taskId]);

  const pageExecutions = taskId ? (task?.executions ?? []) : liveExecutions;
  const selectedExecution = pageExecutions.find(
    (execution) => execution.id === selectedExecutionId,
  );

  useEffect(() => {
    if (!task || !LIVE_STATUSES.has(task.status)) {
      return;
    }
    let active = true;
    let refreshing = false;

    async function refresh() {
      if (refreshing) {
        return;
      }
      refreshing = true;
      try {
        const nextTask = await fetchTask(taskId);
        if (active) {
          setTask(nextTask);
          setTasks((current) =>
            current.map((item) =>
              item.id === nextTask.id ? nextTask : item,
            ),
          );
        }
      } catch (caught) {
        if (active) {
          setError(
            caught instanceof Error ? caught.message : String(caught),
          );
        }
      } finally {
        refreshing = false;
      }
    }

    const interval = window.setInterval(() => void refresh(), 2500);
    return () => {
      active = false;
      window.clearInterval(interval);
    };
  }, [task, taskId]);

  useEffect(() => {
    document.title = task ? `${task.title} — AGC` : "AGC — Monitor";
  }, [task]);

  // Long pipelines scroll; keep the stop you are on in view.
  useEffect(() => {
    currentStopRef.current?.scrollIntoView({
      block: "nearest",
      behavior: "smooth",
    });
  }, [selectedExecutionId]);

  const activeWorkflowId = selectedExecution?.workflow_slug ?? "";
  const executions = pageExecutions;
  const liveIndex = executions.findIndex(
    (execution) => execution.status === "running",
  );
  const failedIndex = executions.findIndex((execution) =>
    FAILED_STATUSES.has(execution.status),
  );
  const highlightIndex = failedIndex >= 0 ? failedIndex : liveIndex;
  const highlightExecution =
    highlightIndex >= 0 ? executions[highlightIndex] : undefined;
  const highlightWorkflow = highlightExecution
    ? workflows.find(
        (workflow) => workflow.slug === highlightExecution.workflow_slug,
      )?.name || highlightExecution.workflow_slug
    : "";
  const totalStops = pageExecutions.length;
  const doneStops = pageExecutions.filter(
    (execution) => execution.status === "succeeded",
  ).length;

  return (
    <div
      className="builder-app monitor-app"
      data-component="WorkflowsMonitor"
    >
      <TopBar
        title={
          taskId
            ? task?.title || (loading ? "Loading task…" : "Task")
            : "Running workflows"
        }
        context={
          selectedExecution?.project_dir && (
            <RunLocation
              project={selectedExecution.project_dir}
            />
          )
        }
        actions={
          <>
            {task && (
              <span
                className={`task-status-pill task-status-pill--${task.status}`}
              >
                <span />
                {task.status}
              </span>
            )}
            {task && (
              <span className="monitor-header__elapsed mono">
                {formatTaskElapsed(task)}
              </span>
            )}
            {task && LIVE_STATUSES.has(task.status) && (
              <button
                type="button"
                className="builder-danger-button"
                disabled
                title="Cancelling a run needs a backend endpoint"
              >
                <StopIcon /> Stop
              </button>
            )}
          </>
        }
      />

      <LeftPanel
        className="monitor-strip"
        header={
          <>
        {taskId && (
        <Link
          to={`/?task=${encodeURIComponent(taskId)}`}
          className="monitor-strip__back"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 14 14"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.1"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <path d="M8.5 3 L4.5 7 L8.5 11" />
          </svg>
          All tasks
        </Link>
        )}

        {taskId && task && (
          <button
            type="button"
            className={[
              "monitor-live",
              failedIndex >= 0
                ? "monitor-live--failed"
                : liveIndex >= 0
                  ? "monitor-live--running"
                  : "monitor-live--done",
            ].join(" ")}
            disabled={highlightIndex < 0}
            onClick={() =>
              highlightExecution &&
              setSelectedExecutionId(highlightExecution.id)
            }
            title={highlightIndex >= 0 ? "Jump to this workflow" : undefined}
          >
            <span className="monitor-live__label">
              {failedIndex >= 0
                ? "Failed at"
                : liveIndex >= 0
                  ? "Running now"
                  : "Completed"}
            </span>
            {highlightExecution ? (
              <>
                <span className="monitor-live__name">
                  <StatusMarker
                    status={executionStatus(task, highlightExecution)}
                  />
                  <b>{highlightWorkflow}</b>
                </span>
                <span className="monitor-live__meta mono">
                  step {highlightIndex + 1} of {totalStops} ·{" "}
                  {executionDuration(highlightExecution) || "—"}
                </span>
              </>
            ) : (
              <span className="monitor-live__meta mono">
                {totalStops} workflow{totalStops === 1 ? "" : "s"} ·{" "}
                {formatTaskElapsed(task)}
              </span>
            )}
          </button>
        )}

        <div className="monitor-steps__head">
            <h2>{taskId ? "Workflows" : "Running now"}</h2>
            <span className="mono">
              {taskId ? `${doneStops}/${totalStops}` : totalStops}
            </span>
          </div>
          <div
            className="monitor-steps__progress"
            role="progressbar"
            aria-valuemin={0}
            aria-valuemax={totalStops}
            aria-valuenow={doneStops}
          >
            <span
              style={{
                width: `${totalStops ? (doneStops / totalStops) * 100 : 0}%`,
              }}
            />
          </div>
          </>
        }
      >
        <ol className="monitor-steps">
            {pageExecutions.map((execution, index) => {
              const status = task
                ? executionStatus(task, execution)
                : execution.status;
              const isCurrent = execution.id === selectedExecutionId;
              return (
                <li
                  key={execution.id}
                  ref={isCurrent ? currentStopRef : undefined}
                  className={[
                    "monitor-steps__stop",
                    `monitor-steps__stop--${status}`,
                    isCurrent ? "monitor-steps__stop--current" : "",
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
                    aria-current={isCurrent ? "step" : undefined}
                    onClick={() => setSelectedExecutionId(execution.id)}
                  >
                    <b>
                      {taskId && <em>{index + 1}</em>}
                      {workflows.find(
                        (workflow) =>
                          workflow.slug === execution.workflow_slug,
                      )?.name || execution.workflow_slug}
                    </b>
                    <small>
                      {status}
                      <span>{executionDuration(execution)}</span>
                    </small>
                  </button>
                </li>
              );
            })}
        </ol>
      </LeftPanel>

      <ExecutionCanvas
        execution={selectedExecution}
        workflows={workflows}
        loading={loading}
        onResumed={(executionId) =>
          navigate(`/executions/${encodeURIComponent(executionId)}`)
        }
        emptyTitle={
          taskId || totalStops > 0
            ? "No execution snapshot"
            : "Nothing running"
        }
        emptyDescription={
          taskId || totalStops > 0
            ? "This run has no renderable execution snapshot."
            : "Start a workflow and it will appear here while it runs."
        }
      />
    </div>
  );
}
