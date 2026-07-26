/**
 * The marker beside a workflow in a step list. Shared by the task
 * monitor and the builder so a workflow reads the same in both.
 */
export function StatusMarker({ status }: { status: string }) {
  if (status === "succeeded") {
    return (
      <svg viewBox="0 0 16 16" aria-hidden="true">
        <circle cx="8" cy="8" r="8" fill="var(--st-ok)" />
        <path
          d="M4.6 8.3 L6.9 10.6 L11.4 5.6"
          fill="none"
          stroke="var(--bg)"
          strokeWidth="1.9"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    );
  }
  if (status === "failed" || status === "blocked" || status === "canceled") {
    return (
      <svg viewBox="0 0 16 16" aria-hidden="true">
        <circle cx="8" cy="8" r="8" fill="var(--st-err)" />
        <path
          d="M5.4 5.4 L10.6 10.6 M10.6 5.4 L5.4 10.6"
          fill="none"
          stroke="var(--bg)"
          strokeWidth="1.9"
          strokeLinecap="round"
        />
      </svg>
    );
  }
  if (status === "running") {
    return (
      <svg viewBox="0 0 16 16" aria-hidden="true">
        <circle cx="8" cy="8" r="8" fill="var(--st-run)" />
        <circle
          className="monitor-steps__spinner"
          cx="8"
          cy="8"
          r="5"
          fill="none"
          stroke="var(--bg)"
          strokeWidth="1.9"
          strokeLinecap="round"
          strokeDasharray="16 12"
        />
      </svg>
    );
  }
  // queued, never run, or anything not yet reached
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true">
      <circle
        cx="8"
        cy="8"
        r="6.6"
        fill="var(--paper)"
        stroke="var(--ink-4)"
        strokeWidth="1.8"
        strokeDasharray="3 2.6"
      />
    </svg>
  );
}

/** Elapsed time for a workflow execution, or "" if it has not started. */
export function executionDuration(execution: {
  started_at?: string;
  created_at?: string;
  finished_at?: string;
}) {
  const started = Date.parse(
    execution.started_at || execution.created_at || "",
  );
  if (Number.isNaN(started)) {
    return "";
  }
  const finished = execution.finished_at
    ? Date.parse(execution.finished_at)
    : Date.now();
  if (finished < started) {
    return "";
  }
  const seconds = Math.max(0, Math.round((finished - started) / 1000));
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m`;
  }
  const hours = minutes / 60;
  return hours < 48 ? `${hours.toFixed(1)}h` : `${Math.round(hours / 24)}d`;
}
