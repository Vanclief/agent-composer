/**
 * The processes running beside a session's agent, and their logs.
 *
 * The Procfile defines what a project can run; a process only exists
 * once something started it, so a row is a definition plus whatever
 * process currently carries its name. Selection tracks the NAME for
 * that reason — a restart mints a new id but keeps the name, and the
 * log viewer should follow the restart rather than the dead process.
 */
import { useEffect, useRef, useState } from "react";
import { PlayIcon, StopIcon } from "../builder/Icons";
import {
  definitions,
  processFor,
  processesFor,
  readLogs,
  removeProcess,
  runCommand,
  startProcess,
  stopProcess,
  useSessionsStore,
} from "./mock";
import type { ProcessStatus, Session } from "./types";

/** Past this the reader is looking back, so following would fight them. */
const STICK_TO_BOTTOM = 24;

/** Coarse relative time — a log header never needs more than one unit. */
function ago(iso: string) {
  const then = Date.parse(iso);
  if (Number.isNaN(then)) {
    return "";
  }
  const seconds = Math.max(0, Math.round((Date.now() - then) / 1000));
  if (seconds < 60) {
    return `${seconds}s ago`;
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m ago`;
  }
  const hours = Math.floor(minutes / 60);
  return hours < 24 ? `${hours}h ago` : `${Math.floor(hours / 24)}d ago`;
}

/*
 * The dot follows the reported status and nothing else — `exited` is a
 * finished process (a stop included), `failed` is the only red state.
 * Re-deriving that from the exit code would relabel a stop as an error.
 */
function dotModifier(status: ProcessStatus) {
  if (status === "running") {
    return "running";
  }
  if (status === "failed") {
    return "err";
  }
  return status === "exited" ? "ok" : "idle";
}

export function ProcessesPanel({ session }: { session: Session }) {
  useSessionsStore();
  const defs = definitions(session.id);
  const live = processesFor(session.id);
  const running = live.filter((process) => process.status === "running");

  const [selected, setSelected] = useState("");
  const [command, setCommand] = useState("");
  const [follow, setFollow] = useState(true);
  const logRef = useRef<HTMLPreElement>(null);

  // Each tab picks its own default; the run box is not carried over.
  useEffect(() => {
    setSelected("");
    setCommand("");
    setFollow(true);
  }, [session.id]);

  // Falling back every render is what makes a restart keep the viewer.
  const fallback = running[0]?.name ?? defs[0]?.name ?? "";
  const activeName = defs.some((item) => item.name === selected)
    ? selected
    : fallback;
  const active = activeName ? processFor(session.id, activeName) : undefined;
  const lines = active ? readLogs(active.id) : [];

  useEffect(() => {
    const node = logRef.current;
    if (!follow || !node) {
      return;
    }
    node.scrollTop = node.scrollHeight;
  }, [follow, activeName, lines.length]);

  function start(name: string) {
    startProcess(session.id, name);
    setSelected(name);
    setFollow(true);
  }

  return (
    <aside className="session-processes" data-component="ProcessesPanel">
      <div className="session-processes__head">
        <h3>Processes</h3>
        {running.length > 0 && (
          <span className="session-processes__count mono">
            {running.length} running
          </span>
        )}
      </div>

      <div className="session-processes__run">
        <input
          className="builder-input mono"
          placeholder="npm run lint"
          aria-label="Run a command in this session"
          value={command}
          onChange={(event) => setCommand(event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== "Enter") {
              return;
            }
            event.preventDefault();
            const started = runCommand(session.id, command);
            setCommand("");
            if (started) {
              setSelected(started.name);
              setFollow(true);
            }
          }}
        />
      </div>

      <ul className="session-processes__list scrollnice">
        {defs.map((definition) => {
          const process = processFor(session.id, definition.name);
          const status = process?.status ?? "stopped";
          const finished = status === "exited" || status === "failed";
          return (
            <li
              key={definition.name}
              className={`session-processes__row${
                definition.name === activeName
                  ? " session-processes__row--selected"
                  : ""
              }`}
            >
              <button
                type="button"
                className="session-processes__select"
                onClick={() => {
                  setSelected(definition.name);
                  setFollow(true);
                }}
              >
                <span
                  className={`session-processes__dot session-processes__dot--${dotModifier(
                    status,
                  )}`}
                />
                <b>{definition.name}</b>
                {definition.adHoc && (
                  <span className="session-processes__adhoc">ad hoc</span>
                )}
                <span className="session-processes__command mono">
                  {definition.command}
                </span>
              </button>

              <div className="session-processes__actions">
                {finished && process?.exitCode !== undefined && (
                  <span
                    className={`session-processes__exit mono${
                      status === "failed"
                        ? " session-processes__exit--failed"
                        : ""
                    }`}
                  >
                    exit {process.exitCode}
                  </span>
                )}
                {status === "running" && process ? (
                  <>
                    {process.port !== undefined && (
                      <span className="session-processes__port mono">
                        :{process.port}
                      </span>
                    )}
                    <button
                      type="button"
                      className="builder-icon-button"
                      aria-label={`Stop ${definition.name}`}
                      onClick={() => stopProcess(process.id)}
                    >
                      <StopIcon />
                    </button>
                  </>
                ) : (
                  <button
                    type="button"
                    className="builder-icon-button"
                    aria-label={`Start ${definition.name}`}
                    onClick={() => start(definition.name)}
                  >
                    <PlayIcon />
                  </button>
                )}
                {finished && process && (
                  <button
                    type="button"
                    className="builder-icon-button"
                    aria-label={`Forget ${definition.name}`}
                    onClick={() => removeProcess(process.id)}
                  >
                    ×
                  </button>
                )}
              </div>
            </li>
          );
        })}
      </ul>

      <div className="session-logs">
        <div className="session-logs__head">
          <b className="session-logs__name">{activeName || "logs"}</b>
          <span className="session-logs__meta mono">
            {active
              ? `${active.status}${
                  active.startedAt ? ` · started ${ago(active.startedAt)}` : ""
                }`
              : "never run"}
          </span>
          <button
            type="button"
            className={`builder-ghost-button session-logs__follow${
              follow ? " active" : ""
            }`}
            aria-pressed={follow}
            onClick={() => setFollow(!follow)}
          >
            Follow
          </button>
        </div>

        <pre
          className="session-logs__body mono scrollnice"
          ref={logRef}
          onScroll={(event) => {
            const node = event.currentTarget;
            const gap = node.scrollHeight - node.scrollTop - node.clientHeight;
            if (follow && gap > STICK_TO_BOTTOM) {
              setFollow(false);
            }
          }}
        >
          {!active && (
            <div className="session-logs__placeholder">
              Not started — press ▶ or ask the agent to start it.
            </div>
          )}
          {lines.map((line) => (
            <div
              key={line.seq}
              className={`session-logs__line${
                line.stream === "stderr" ? " session-logs__line--stderr" : ""
              }`}
            >
              {line.text}
            </div>
          ))}
          {active?.exitCode !== undefined && (
            <div className="session-logs__note">
              — exited with code {active.exitCode} —
            </div>
          )}
        </pre>
      </div>
    </aside>
  );
}
