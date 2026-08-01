import {
  useEffect,
  useRef,
  useState,
} from "react";
import { composeWorkflow } from "../api";

export interface EditResult {
  workflow_id?: string;
  action?: string;
  summary?: string;
  draft?: string;
}

interface Turn {
  id: number;
  request: string;
  status: "running" | "succeeded" | "failed";
  summary?: string;
  action?: string;
  model?: string;
  error?: string;
}

/**
 * Session transcripts per workflow. Module state, not component
 * state, so the history survives panel toggles and workflow switches;
 * a reload starts fresh (the draft itself is the durable state).
 */
const transcripts = new Map<string, Turn[]>();
let turnCounter = 0;

/** Transcript key — creations start under a placeholder key. */
function transcriptKey(workflowId: string) {
  return workflowId || "__new__";
}

/**
 * The Composer — the conversation with the blueprint-editing agent.
 * Every turn is one compose call; the workflow's draft carries the
 * state between turns, and nothing lands without Save.
 */
export function ComposerPanel({
  workflowId,
  onApplied,
  onClose,
}: {
  /** Workflow being edited; empty means "create a new one". */
  workflowId: string;
  /** Called after a turn that produced or updated a draft. */
  onApplied: (result: EditResult) => void;
  onClose: () => void;
}) {
  const key = transcriptKey(workflowId);
  const [turns, setTurns] = useState<Turn[]>(
    () => transcripts.get(key) ?? [],
  );
  const [request, setRequest] = useState("");
  const keyRef = useRef(key);
  keyRef.current = key;
  const listRef = useRef<HTMLDivElement>(null);

  // Switching workflows swaps in that workflow's transcript.
  useEffect(() => {
    setTurns(transcripts.get(key) ?? []);
  }, [key]);

  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight });
  }, [turns]);

  const busy = turns.some((turn) => turn.status === "running");

  // Writes a transcript and, when it is the one on screen, the view.
  function mutate(targetKey: string, next: Turn[]) {
    transcripts.set(targetKey, next);
    if (keyRef.current === targetKey) {
      setTurns(next);
    }
  }

  async function submit() {
    const trimmed = request.trim();
    if (!trimmed || busy) {
      return;
    }
    const turnId = ++turnCounter;
    const startedKey = key;
    mutate(startedKey, [
      ...(transcripts.get(startedKey) ?? []),
      { id: turnId, request: trimmed, status: "running" },
    ]);
    setRequest("");

    const finish = (patch: Partial<Turn>) => {
      const current = transcripts.get(startedKey) ?? [];
      mutate(
        startedKey,
        current.map((turn) =>
          turn.id === turnId ? { ...turn, ...patch } : turn,
        ),
      );
    };

    try {
      const response = await composeWorkflow(workflowId, trimmed);
      finish({
        status: "succeeded",
        summary: response.summary || "Done.",
        action: response.action,
        model: response.model,
      });
      if (startedKey === "__new__" && response.workflow_id) {
        const moved = transcripts.get("__new__") ?? [];
        transcripts.delete("__new__");
        transcripts.set(response.workflow_id, moved);
      }
      onApplied(response);
    } catch (caught) {
      finish({
        status: "failed",
        error:
          caught instanceof Error ? caught.message : String(caught),
      });
    }
  }

  return (
    <aside className="composer-panel" data-component="ComposerPanel">
      <div className="composer-panel__head">
        <h3>Composer</h3>
        <button
          type="button"
          className="builder-icon-button"
          aria-label="Close the composer"
          onClick={onClose}
        >
          ×
        </button>
      </div>

      <div className="composer-panel__turns scrollnice" ref={listRef}>
        {turns.length === 0 && (
          <div className="composer-panel__empty">
            <b>
              {workflowId
                ? "Describe a change"
                : "Describe a workflow"}
            </b>
            <span>
              The composer agent proposes blueprint changes as a
              draft — nothing lands until you save it.
            </span>
          </div>
        )}
        {turns.map((turn) => (
          <div key={turn.id} className="composer-turn">
            <div className="composer-turn__request">
              {turn.request}
            </div>
            {turn.status === "running" ? (
              <div className="composer-turn__reply">
                <span className="builder-stream-cursor" />
                Working…
              </div>
            ) : turn.status === "failed" ? (
              <div className="composer-turn__reply composer-turn__reply--failed">
                {turn.error}
              </div>
            ) : (
              <div className="composer-turn__reply">
                {turn.summary}
                <small>
                  {turn.action === "unchanged"
                    ? "no changes"
                    : `draft ${turn.action ?? "updated"}`}
                  {turn.model ? ` · ${turn.model}` : ""}
                </small>
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="composer-panel__input">
        <textarea
          rows={2}
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
            if (event.key === "Enter" && !event.shiftKey) {
              event.preventDefault();
              void submit();
            }
          }}
        />
        <button
          type="button"
          disabled={busy || !request.trim()}
          onClick={() => void submit()}
        >
          {busy ? "Working…" : "Send"}
        </button>
      </div>
    </aside>
  );
}
