import { useState } from "react";
import { composeWorkflow } from "../api";

export interface EditResult {
  workflow_id?: string;
  action?: string;
  summary?: string;
}

interface ComposeState {
  status: "running" | "succeeded" | "failed";
  summary?: string;
  error?: string;
}

/**
 * "Describe a change…" — the agent-writes-the-structure entry point.
 * Each submit is one composer conversation on the server (harness and
 * model come from Settings); the compiler gates every install, so a
 * bad edit never lands.
 */
export function ChangeComposer({
  workflowId,
  onApplied,
}: {
  /** Workflow being edited; empty means "create a new one". */
  workflowId: string;
  /** Called after a successful edit so the caller can reload specs. */
  onApplied: (result: EditResult) => void;
}) {
  const [request, setRequest] = useState("");
  const [state, setState] = useState<ComposeState | null>(null);

  const busy = state?.status === "running";

  async function submit() {
    const trimmed = request.trim();
    if (!trimmed || busy) {
      return;
    }
    setState({ status: "running" });
    try {
      const response = await composeWorkflow(workflowId, trimmed);
      setState({
        status: "succeeded",
        summary: response.draft
          ? `${response.summary || "Draft ready."} Review the draft, then Save.`
          : response.summary || "No changes proposed.",
      });
      setRequest("");
      onApplied(response);
    } catch (caught) {
      setState({
        status: "failed",
        error:
          caught instanceof Error ? caught.message : String(caught),
      });
    }
  }

  return (
    <div className="builder-change-composer">
      {state && (
        <div
          className={`builder-change-composer__status builder-change-composer__status--${state.status}`}
        >
          {state.status === "running" && (
            <>
              <span className="builder-stream-cursor" />
              <span>The composer agent is working…</span>
            </>
          )}
          {state.status === "succeeded" && (
            <span>{state.summary}</span>
          )}
          {state.status === "failed" && <span>{state.error}</span>}
          {state.status !== "running" && (
            <button
              type="button"
              aria-label="Dismiss"
              onClick={() => setState(null)}
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
