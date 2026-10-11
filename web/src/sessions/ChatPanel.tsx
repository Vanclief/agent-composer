/**
 * The chat with a session's main agent.
 *
 * One long-lived harness session owns the project directory, so the
 * transcript here is a view of what the daemon already remembers —
 * never the source of truth. Everything reads from the sessions store,
 * which is why switching tabs swaps transcripts with no fetch.
 */
import { useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { fetchHarnesses, fetchSettings } from "../api";
import { REASONING_EFFORTS } from "../builder/Inspector";
import { ModelPicker } from "../builder/ModelPicker";
import type { HarnessInfo } from "../types/api";
import {
  clearTranscript,
  sendMessage,
  setSessionAgent,
  turns,
  useSessionsStore,
} from "./mock";
import type { Session } from "./types";

/** Roughly six lines of the input's 12.5px/1.45 text, plus padding. */
const MAX_INPUT_HEIGHT = 124;

/** A reader who scrolled back this far is reading, not following. */
const STICK_TO_BOTTOM = 80;

/** What the mock agent actually understands — worth showing up front. */
const EXAMPLES = ["start dev", "why did api crash?", "status", "restart api"];

/** Code spans and bold are the only markdown the agent's replies use. */
const INLINE = /`([^`]+)`|\*\*([^*]+?)\*\*/g;

function renderInline(text: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  let cursor = 0;
  let key = 0;
  for (const match of text.matchAll(INLINE)) {
    const at = match.index ?? 0;
    if (at > cursor) {
      nodes.push(text.slice(cursor, at));
    }
    const code = match[1];
    const bold = match[2];
    if (code !== undefined) {
      nodes.push(
        <code key={key++} className="mono">
          {code}
        </code>,
      );
    } else if (bold !== undefined) {
      nodes.push(<b key={key++}>{bold}</b>);
    }
    cursor = at + match[0].length;
  }
  if (cursor < text.length) {
    nodes.push(text.slice(cursor));
  }
  return nodes;
}

/** Send — the arrow every chat box has taught people to look for. */
function SendGlyph() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M8 13 V3" />
      <path d="M4 7 L8 3 L12 7" />
    </svg>
  );
}

/** A prompt caret — marks a chip as something the agent ran, not said. */
function TerminalGlyph({ size = 10 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M3 4 L6.5 8 L3 12" />
      <path d="M8.5 12 H13" />
    </svg>
  );
}

export function ChatPanel({ session }: { session: Session }) {
  useSessionsStore();
  const transcript = turns(session.id);
  const last = transcript[transcript.length - 1];
  const busy = last?.status === "running";

  const [draft, setDraft] = useState("");
  const [harnesses, setHarnesses] = useState<HarnessInfo[]>([]);
  const listRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const followRef = useRef(true);

  // Installed harnesses come from the server; a tab that has never
  // chosen an agent inherits the Composer default, else the first one
  // installed — the same order the server resolves.
  useEffect(() => {
    const controller = new AbortController();
    Promise.all([
      fetchSettings(controller.signal),
      fetchHarnesses(controller.signal),
    ])
      .then(([settings, list]) => {
        const installed = (list?.harnesses ?? []).filter(
          (info) => info.available,
        );
        setHarnesses(installed);
        if (session.harness && session.model) {
          return;
        }
        const harness =
          session.harness ||
          settings?.composer?.harness ||
          installed[0]?.id ||
          "";
        const model =
          session.model ||
          settings?.composer?.model ||
          installed.find((info) => info.id === harness)?.models?.[0] ||
          "";
        if (harness || model) {
          setSessionAgent(session.id, {
            harness,
            model,
            effort: session.effort || "medium",
          });
        }
      })
      .catch(() => {
        // Without a server the picker stays empty; the mock answers anyway.
      });
    return () => controller.abort();
    // Only re-resolve when the tab changes, not on every keystroke.
  }, [session.id]);

  function pickAgent(harness: string, model: string, effort: string) {
    setSessionAgent(session.id, { harness, model, effort });
  }

  const knownModels =
    harnesses.find((info) => info.id === session.harness)?.models ?? [];

  // A new tab starts with an empty box, its own transcript, and focus.
  useEffect(() => {
    setDraft("");
    followRef.current = true;
    const list = listRef.current;
    if (list) {
      list.scrollTop = list.scrollHeight;
    }
    inputRef.current?.focus();
  }, [session.id]);

  // Follow the conversation, unless the reader scrolled away from it.
  useEffect(() => {
    const list = listRef.current;
    if (!list || !followRef.current) {
      return;
    }
    list.scrollTop = list.scrollHeight;
  }, [transcript.length, last?.status]);

  // The box grows with the draft and then scrolls inside itself.
  useEffect(() => {
    const input = inputRef.current;
    if (!input) {
      return;
    }
    input.style.height = "auto";
    input.style.height = `${Math.min(input.scrollHeight, MAX_INPUT_HEIGHT)}px`;
  }, [draft]);

  function submit() {
    const text = draft.trim();
    if (!text || busy) {
      return;
    }
    setDraft("");
    followRef.current = true;
    // Rejection only happens when the tab is gone; the store owns the turn.
    void sendMessage(session.id, text).catch(() => {});
  }

  return (
    <section className="session-chat" data-component="ChatPanel">
      <div className="session-chat__head">
        <h3>Main agent</h3>
        <span className="session-chat__ref mono">
          {session.sessionRef || "new session"}
        </span>
        <button
          type="button"
          className="builder-ghost-button session-chat__clear"
          disabled={transcript.length === 0}
          onClick={() => clearTranscript(session.id)}
        >
          Clear
        </button>
      </div>

      <div
        className="session-chat__turns scrollnice"
        ref={listRef}
        onScroll={(event) => {
          const node = event.currentTarget;
          const gap = node.scrollHeight - node.scrollTop - node.clientHeight;
          followRef.current = gap <= STICK_TO_BOTTOM;
        }}
      >
        {transcript.length === 0 && (
          <div className="session-chat__empty">
            <b>Talk to the main agent for {session.name}</b>
            <span>
              It runs in this directory and can start, stop and read the
              logs of the processes beside it. Try:
            </span>
            <div className="session-chat__examples">
              {EXAMPLES.map((example) => (
                <button
                  key={example}
                  type="button"
                  className="session-chat__example mono"
                  onClick={() => {
                    setDraft(example);
                    inputRef.current?.focus();
                  }}
                >
                  {example}
                </button>
              ))}
            </div>
          </div>
        )}

        {transcript.map((turn) => (
          <div key={turn.id} className="session-chat__turn">
            <div className="session-chat__request">{turn.request}</div>

            {turn.actions && turn.actions.length > 0 && (
              <div className="session-chat__actions">
                {turn.actions.map((action, index) => (
                  <span
                    key={`${action}-${index}`}
                    className="session-chat__action mono"
                  >
                    <TerminalGlyph />
                    {action}
                  </span>
                ))}
              </div>
            )}

            {turn.status === "running" ? (
              <div className="session-chat__thinking">
                <span className="session-chat__dot" />
                thinking…
              </div>
            ) : turn.status === "failed" ? (
              <div className="session-chat__reply session-chat__reply--failed">
                {turn.error || "The agent did not answer."}
              </div>
            ) : (
              <div className="session-chat__reply">
                {renderInline(turn.reply ?? "")}
                {turn.model && (
                  <small className="session-chat__meta mono">
                    {turn.model}
                    {turn.effort ? ` · ${turn.effort}` : ""}
                  </small>
                )}
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="session-chat__composer">
        <textarea
          ref={inputRef}
          rows={1}
          placeholder={`Ask the agent about ${session.name}…`}
          aria-label="Message the main agent"
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== "Enter") {
              return;
            }
            if (event.metaKey || event.ctrlKey || !event.shiftKey) {
              event.preventDefault();
              submit();
            }
          }}
        />
        <div className="session-chat__toolbar">
          <select
            className="builder-select mono session-chat__pill"
            aria-label="Harness"
            value={session.harness}
            disabled={busy || harnesses.length === 0}
            onChange={(event) => {
              const next = event.target.value;
              pickAgent(
                next,
                harnesses.find((info) => info.id === next)?.models?.[0] ?? "",
                session.effort,
              );
            }}
          >
            {harnesses.length === 0 && (
              <option value={session.harness}>
                {session.harness || "no harnesses"}
              </option>
            )}
            {session.harness &&
              harnesses.length > 0 &&
              !harnesses.some((info) => info.id === session.harness) && (
                <option value={session.harness}>
                  {session.harness} (not installed)
                </option>
              )}
            {harnesses.map((info) => (
              <option key={info.id} value={info.id}>
                {info.id}
              </option>
            ))}
          </select>
          <ModelPicker
            value={session.model}
            models={knownModels}
            disabled={busy || harnesses.length === 0}
            onChange={(model) =>
              pickAgent(session.harness, model, session.effort)
            }
          />
          <select
            className="builder-select mono session-chat__pill session-chat__effort"
            aria-label="Reasoning effort"
            title="Reasoning effort"
            value={session.effort}
            disabled={busy || harnesses.length === 0}
            onChange={(event) =>
              pickAgent(session.harness, session.model, event.target.value)
            }
          >
            {!REASONING_EFFORTS.includes(session.effort) && (
              <option value={session.effort}>{session.effort}</option>
            )}
            {REASONING_EFFORTS.map((level) => (
              <option key={level} value={level}>
                {level}
              </option>
            ))}
          </select>
          <span className="session-chat__toolbar-spacer" />
          <span className="session-chat__hint">
            {busy ? "working…" : "Enter to send · Shift+Enter for a new line"}
          </span>
          <button
            type="button"
            className="session-chat__send"
            aria-label="Send"
            title="Send (Enter)"
            disabled={busy || !draft.trim()}
            onClick={submit}
          >
            <SendGlyph />
          </button>
        </div>
      </div>
    </section>
  );
}
