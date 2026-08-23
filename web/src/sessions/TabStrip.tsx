import { activity } from "./mock";
import type { Session, SessionActivity } from "./types";
import { KEY_HINT } from "./useTabKeys";

/**
 * The one glyph a tab can afford, in the order that matters: a broken
 * process outranks a busy one, and anything happening outranks a reply
 * you have not read yet.
 */
function ActivityFlag({ state }: { state: SessionActivity }) {
  if (state.failed > 0) {
    return (
      <span
        className="tab-strip__flag tab-strip__flag--failed"
        aria-hidden="true"
      >
        !
      </span>
    );
  }
  if (state.running > 0) {
    return (
      <span
        className="tab-strip__flag tab-strip__flag--running"
        aria-hidden="true"
      >
        <i className="tab-strip__dot" />
        {state.running > 1 ? state.running : null}
      </span>
    );
  }
  if (state.unread) {
    return (
      <span
        className="tab-strip__flag tab-strip__flag--unread"
        aria-hidden="true"
      >
        <i className="tab-strip__ring" />
      </span>
    );
  }
  if (state.thinking) {
    return (
      <span
        className="tab-strip__flag tab-strip__flag--thinking"
        aria-hidden="true"
      >
        <i />
        <i />
        <i />
      </span>
    );
  }
  return null;
}

/** Everything the glyph stands for, spelled out for the tooltip. */
function describe(state: SessionActivity) {
  const notes: string[] = [];
  if (state.failed > 0) {
    notes.push(`${state.failed} failed`);
  }
  if (state.running > 0) {
    notes.push(`${state.running} running`);
  }
  if (state.unread) {
    notes.push("agent replied");
  }
  if (state.thinking) {
    notes.push("agent thinking");
  }
  return notes.join(" · ");
}

/**
 * The tmux status line: every session as a numbered tab along the
 * bottom of the window, the one in view highlighted. Selection and
 * ordering are driven from here or from the keyboard (useTabKeys).
 */
export function TabStrip({
  sessions,
  activeId,
  armed = false,
  onSelect,
  onNew,
}: {
  sessions: Session[];
  activeId: string;
  /** The ⌃B prefix is waiting for its next key. */
  armed?: boolean;
  onSelect: (id: string) => void;
  onNew: () => void;
}) {
  return (
    <footer className="tab-strip" data-component="TabStrip">
      <span className="tab-strip__brand mono">agc</span>
      {armed && (
        <span className="tab-strip__prefix mono" title="Prefix armed">
          ^B
        </span>
      )}
      <div className="tab-strip__tabs">
        {sessions.map((session, index) => {
          const state = activity(session.id);
          const label = `${index + 1}:${session.name}`;
          const note = describe(state);
          const active = session.id === activeId;
          return (
            <button
              key={session.id}
              type="button"
              className={`tab-strip__tab mono${active ? " active" : ""}`}
              title={note ? `${label} — ${note}` : label}
              aria-current={active ? "page" : undefined}
              onClick={() => onSelect(session.id)}
            >
              <span className="tab-strip__label">{label}</span>
              <ActivityFlag state={state} />
            </button>
          );
        })}
      </div>
      <button
        type="button"
        className="tab-strip__new"
        title="New session (⌃B c)"
        aria-label="New session"
        onClick={onNew}
      >
        +
      </button>
      <span className="tab-strip__spacer" />
      <span className="tab-strip__hint mono">{KEY_HINT}</span>
    </footer>
  );
}
