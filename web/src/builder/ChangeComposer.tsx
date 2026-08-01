import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { fetchWorkflowExecution } from "../api";
import { startTask } from "../tasks/data";

/** The installed workflow that performs blueprint edits. */
export const EDITOR_WORKFLOW_ID = "blueprint-editor";

const POLL_MS = 2500;
const LIVE_STATUSES = new Set(["queued", "running"]);

export interface EditResult {
  workflow_id?: string;
  action?: string;
  summary?: string;
}

interface EditRun {
  executionId: string;
  status: "running" | "succeeded" | "failed";
  result?: EditResult;
  error?: string;
}

/**
 * "Describe a change…" — the agent-writes-the-structure entry point.
 * Each submit launches the blueprint-editor workflow, which edits the
 * registry itself (via the agc CLI, compile-checked); the run is a
 * normal recorded execution you can open like any other.
 */
export function ChangeComposer({
  workflowId,
  onApplied,
}: {
  /** Workflow being edited; empty means "create a new one". */
  workflowId: string;
  /** Called after a successful run so the caller can reload specs. */
  onApplied: (result: EditResult) => void;
}) {
  const [request, setRequest] = useState("");
  const [run, setRun] = useState<EditRun | null>(null);
  // The latest onApplied without re-arming the poller.
  const onAppliedRef = useRef(onApplied);
  onAppliedRef.current = onApplied;

  const busy = run?.status === "running";

  useEffect(() => {
    if (!run || run.status !== "running") {
      return;
    }
    let active = true;
    const interval = window.setInterval(() => {
      fetchWorkflowExecution(run.executionId)
        .then((execution) => {
          if (!active || !execution) {
            return;
          }
          if (LIVE_STATUSES.has(execution.status)) {
            return;
          }
          const result = (execution.output_snapshot?.result ??
            {}) as EditResult;
          if (execution.status === "succeeded") {
            setRun({
              executionId: run.executionId,
              status: "succeeded",
              result,
            });
            onAppliedRef.current(result);
          } else {
            setRun({
              executionId: run.executionId,
              status: "failed",
              error: `The edit run ${execution.status}.`,
            });
          }
        })
        .catch(() => undefined);
    }, POLL_MS);
    return () => {
      active = false;
      window.clearInterval(interval);
    };
  }, [run]);

  async function submit() {
    const trimmed = request.trim();
    if (!trimmed || busy) {
      return;
    }
    try {
      const response = await startTask(EDITOR_WORKFLOW_ID, {
        workflow_id: workflowId,
        request: trimmed,
      });
      if (!response.execution_id) {
        throw new Error("The server did not return an execution ID.");
      }
      setRequest("");
      setRun({
        executionId: response.execution_id,
        status: "running",
      });
    } catch (caught) {
      const message =
        caught instanceof Error ? caught.message : String(caught);
      setRun({
        executionId: "",
        status: "failed",
        error: message.includes("not found")
          ? `The ${EDITOR_WORKFLOW_ID} workflow is not installed — run: agc workflow import --file examples/blueprint_editor.yaml`
          : message,
      });
    }
  }

  return (
    <div className="builder-change-composer">
      {run && (
        <div
          className={`builder-change-composer__status builder-change-composer__status--${run.status}`}
        >
          {run.status === "running" && (
            <>
              <span className="builder-stream-cursor" />
              <span>
                Editing with the {EDITOR_WORKFLOW_ID} agent…
              </span>
            </>
          )}
          {run.status === "succeeded" && (
            <span>{run.result?.summary || "Done."}</span>
          )}
          {run.status === "failed" && <span>{run.error}</span>}
          {run.executionId && (
            <Link to={`/executions/${run.executionId}`}>
              {run.status === "running" ? "watch the run" : "view run"}{" "}
              →
            </Link>
          )}
          {run.status !== "running" && (
            <button
              type="button"
              aria-label="Dismiss"
              onClick={() => setRun(null)}
            >
              ×
            </button>
          )}
        </div>
      )}
      <div className="builder-change-composer__input">
        <input
          type="text"
          placeholder={
            workflowId
              ? "Describe a change to this workflow…"
              : "Describe the workflow you want…"
          }
          aria-label="Describe a change"
          value={request}
          disabled={busy}
          onChange={(event) => setRequest(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              void submit();
            }
          }}
        />
        <button
          type="button"
          disabled={busy || !request.trim()}
          onClick={() => void submit()}
        >
          {busy ? "Working…" : "Apply"}
        </button>
      </div>
    </div>
  );
}
