import {
  type MouseEvent,
  useEffect,
  useRef,
  useState,
} from "react";
import type { RunDisplayStatus } from "../types/workflow";
import type { RunEntry } from "./runData";

export function StatusPill({
  status,
  tokens,
  milliseconds,
  onClick,
  runId,
}: {
  status: RunDisplayStatus;
  tokens?: number;
  milliseconds?: number;
  onClick?: (event: MouseEvent<HTMLButtonElement>) => void;
  runId?: string;
}) {
  const details: string[] = [];
  if (status === "ok") {
    if (tokens) {
      details.push(`${tokens.toLocaleString()} tok`);
    }
    if (milliseconds) {
      details.push(
        milliseconds >= 1000
          ? `${(milliseconds / 1000).toFixed(1)}s`
          : `${milliseconds}ms`,
      );
    }
  }

  const label =
    status === "run"
      ? "running"
      : status === "err"
        ? "error"
        : status === "warn"
          ? "warn"
          : status === "ok"
            ? details.join(" · ") || "ok"
            : "idle";

  if (onClick) {
    return (
      <button
        type="button"
        className={`builder-pill builder-pill--${status} builder-pill--clickable nodrag nopan`}
        onClick={onClick}
        title={runId ? `Run ${runId}` : undefined}
      >
        <span className="builder-pill__dot" />
        {label}
        <span className="builder-chevron">⌄</span>
      </button>
    );
  }

  return (
    <span
      className={`builder-pill builder-pill--${status}`}
      title={runId ? `Run ${runId}` : undefined}
    >
      <span className="builder-pill__dot" />
      {label}
    </span>
  );
}

export function RunMenu({
  runs,
  currentFullId,
  onPick,
  onClose,
  onViewAll,
  nodeId,
  align = "left",
}: {
  runs: RunEntry[];
  currentFullId?: string;
  onPick: (fullId: string) => void;
  onClose: () => void;
  onViewAll: () => void;
  nodeId?: string;
  align?: "left" | "right";
}) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function closeOnOutside(event: globalThis.MouseEvent) {
      if (
        ref.current &&
        event.target instanceof globalThis.Node &&
        !ref.current.contains(event.target)
      ) {
        onClose();
      }
    }
    document.addEventListener("mousedown", closeOnOutside);
    return () => document.removeEventListener("mousedown", closeOnOutside);
  }, [onClose]);

  return (
    <div
      ref={ref}
      className={`builder-runmenu builder-runmenu--${align} nodrag nopan`}
      onMouseDown={(event) => event.stopPropagation()}
    >
      <div className="builder-runmenu__head">
        <span>{nodeId ? `Runs · ${nodeId}` : "Workflow runs"}</span>
        <span>{runs.length}</span>
      </div>
      {runs.length === 0 && (
        <div className="builder-runmenu__empty">No runs yet</div>
      )}
      {runs.map((run) => {
        const snapshot = nodeId ? run.nodes[nodeId] : undefined;
        const status = snapshot?.status ?? run.status;
        return (
          <button
            type="button"
            key={run.fullId}
            className={`builder-runmenu__item ${
              run.fullId === currentFullId ? "active" : ""
            }`}
            onClick={() => onPick(run.fullId)}
          >
            <span
              className={`builder-runmenu__status builder-runmenu__status--${status}`}
            />
            <span className="builder-runmenu__id">{run.id}</span>
            <span className="builder-runmenu__metric">
              {nodeId && snapshot
                ? snapshot.tokens
                  ? `${snapshot.tokens.toLocaleString()} tok`
                  : snapshot.ms
                    ? `${(snapshot.ms / 1000).toFixed(1)}s`
                    : "—"
                : `${(run.duration / 1000).toFixed(1)}s · ${run.tokens.toLocaleString()} tok`}
            </span>
            <span className="builder-runmenu__when">{run.when}</span>
          </button>
        );
      })}
      <div className="builder-runmenu__foot">
        <span>Showing latest {runs.length}</span>
        <button
          type="button"
          onClick={() => {
            onClose();
            onViewAll();
          }}
        >
          View all runs →
        </button>
      </div>
    </div>
  );
}

export function RunMenuButton({
  run,
  runs,
  onPick,
  onViewAll,
}: {
  run: RunEntry;
  runs: RunEntry[];
  onPick: (fullId: string) => void;
  onViewAll: () => void;
}) {
  const [open, setOpen] = useState(false);

  return (
    <span className="builder-runmenu-anchor">
      <button
        type="button"
        className="builder-ghost-button"
        onClick={() => setOpen((value) => !value)}
      >
        <span aria-hidden="true">↶</span>
        {run.id}
        <span className="builder-chevron">⌄</span>
      </button>
      {open && (
        <RunMenu
          runs={runs}
          currentFullId={run.fullId}
          onPick={(fullId) => {
            setOpen(false);
            onPick(fullId);
          }}
          onClose={() => setOpen(false)}
          onViewAll={onViewAll}
          align="right"
        />
      )}
    </span>
  );
}
