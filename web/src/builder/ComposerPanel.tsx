import {
  useEffect,
  useRef,
  useState,
} from "react";
import {
  composeWorkflow,
  fetchHarnesses,
  fetchSettings,
} from "../api";
import type { HarnessInfo } from "../types/api";
import { ModelPicker } from "./ModelPicker";

export interface EditResult {
  workflow_slug?: string;
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

/**
 * The session's harness/model choice. Module state for the same
 * reason as transcripts: it survives panel toggles and workflow
 * switches. Null until the user picks — then every compose call
 * carries it; the settings default applies otherwise.
 */
let chosenAgent: { harness: string; model: string } | null = null;

/** Transcript key — creations start under a placeholder key. */
function transcriptKey(workflowId: string) {
  return workflowId || "__new__";
}

/**
 * The Composer — the conversation with the spec-editing agent.
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
  const [harnesses, setHarnesses] = useState<HarnessInfo[]>([]);
  const [harness, setHarness] = useState(
    () => chosenAgent?.harness ?? "",
  );
  const [model, setModel] = useState(() => chosenAgent?.model ?? "");
  const keyRef = useRef(key);
  keyRef.current = key;
  const listRef = useRef<HTMLDivElement>(null);

  // Switching workflows swaps in that workflow's transcript.
  useEffect(() => {
    setTurns(transcripts.get(key) ?? []);
  }, [key]);

  // The picker shows the agent that will actually run: the session's
  // earlier choice, else the settings default, else the first
  // installed harness — the same order the server resolves.
  useEffect(() => {
    const controller = new AbortController();
    Promise.all([
      fetchSettings(controller.signal),
      fetchHarnesses(controller.signal),
    ])
      .then(([settings, harnessList]) => {
        const installed = (harnessList?.harnesses ?? []).filter(
          (info) => info.available,
        );
        setHarnesses(installed);
        if (chosenAgent) {
          return;
        }
        const nextHarness =
          settings?.composer?.harness || installed[0]?.id || "";
        setHarness(nextHarness);
        setModel(
          settings?.composer?.model ||
            installed.find((info) => info.id === nextHarness)
              ?.models?.[0] ||
            "",
        );
      })
      .catch(() => {
        // The server resolves the default when the picker stays empty.
      });
    return () => controller.abort();
  }, []);

  function pickAgent(nextHarness: string, nextModel: string) {
    setHarness(nextHarness);
    setModel(nextModel);
    chosenAgent = { harness: nextHarness, model: nextModel };
  }

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
      const response = await composeWorkflow(
        workflowId,
        trimmed,
        harness && model ? { harness, model } : undefined,
      );
      finish({
        status: "succeeded",
        summary: response.summary || "Done.",
        action: response.action,
        model: response.model,
      });
      if (startedKey === "__new__" && response.workflow_slug) {
        const moved = transcripts.get("__new__") ?? [];
        transcripts.delete("__new__");
        transcripts.set(response.workflow_slug, moved);
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
              The composer agent proposes spec changes as a
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

      <div className="composer-panel__agent">
        {harnesses.length === 0 ? (
          <select
            className="builder-select mono"
            aria-label="Composer harness"
            disabled
          >
            <option>No harnesses installed</option>
          </select>
        ) : (
          <select
            className="builder-select mono"
            aria-label="Composer harness"
            value={harness}
            disabled={busy}
            onChange={(event) => {
              const next = event.target.value;
              pickAgent(
                next,
                harnesses.find((info) => info.id === next)
                  ?.models?.[0] ?? "",
              );
            }}
          >
            {harness &&
              !harnesses.some((info) => info.id === harness) && (
                <option value={harness}>
                  {harness} (not installed)
                </option>
              )}
            {harnesses.map((info) => (
              <option key={info.id} value={info.id}>
                {info.id}
              </option>
            ))}
          </select>
        )}
        <ModelPicker
          value={model}
          models={
            harnesses.find((info) => info.id === harness)?.models ??
            []
          }
          disabled={busy || harnesses.length === 0}
          onChange={(next) => pickAgent(harness, next)}
        />
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
