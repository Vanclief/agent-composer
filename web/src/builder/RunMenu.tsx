import { forwardRef, type MouseEvent, type ReactNode } from "react";
import type { RunDisplayStatus } from "../types/workflow";
import { Menu, MenuItem } from "../ui/Menu";
import type { RunEntry } from "./runData";

export const StatusPill = forwardRef<
  HTMLButtonElement,
  {
    status: RunDisplayStatus;
    tokens?: number;
    milliseconds?: number;
    /** Render as a button (menu triggers inject their own onClick). */
    interactive?: boolean;
    onClick?: (event: MouseEvent<HTMLButtonElement>) => void;
    runId?: string;
  }
>(function StatusPill(
  { status, tokens, milliseconds, interactive, onClick, runId },
  ref,
) {
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

  if (interactive || onClick) {
    return (
      <button
        ref={ref}
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
});

/**
 * Run picker dropdown. The trigger must be a single element that
 * forwards its ref (StatusPill with an onClick qualifies).
 */
export function RunMenuDropdown({
  trigger,
  runs,
  currentFullId,
  onPick,
  nodeId,
  align = "end",
}: {
  trigger: ReactNode;
  runs: RunEntry[];
  currentFullId?: string;
  onPick: (fullId: string) => void;
  nodeId?: string;
  align?: "start" | "end";
}) {
  return (
    <Menu trigger={trigger} align={align}>
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
          <MenuItem
            key={run.fullId}
            className={`builder-runmenu__item ${
              run.fullId === currentFullId ? "active" : ""
            }`}
            onSelect={() => onPick(run.fullId)}
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
          </MenuItem>
        );
      })}
      <div className="builder-runmenu__foot">
        <span>Showing latest {runs.length}</span>
      </div>
    </Menu>
  );
}
