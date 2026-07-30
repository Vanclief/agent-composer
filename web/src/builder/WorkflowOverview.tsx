import { useEffect, useState } from "react";
import { fetchWorktrees } from "../api";
import type { WorkflowSummary } from "../types/api";
import { BranchIcon, FolderIcon } from "./Icons";
import { CopyButton, formatValue } from "./Inspector";
import { projectBaseName } from "./ProjectPicker";
import { type RunEntry } from "./runData";
import { StatusPill } from "./RunMenu";
import { readStoredProjects } from "./useLaunchLocation";

function projectNameFor(path: string) {
  const match = readStoredProjects().find(
    (project) => project.path === path,
  );
  return match?.name || projectBaseName(path);
}

/**
 * Where the run executed: project name plus, for worktree runs, the
 * workspace branch. Only shell_root is persisted, so the project repo
 * behind a worktree path is recovered by asking git for its siblings.
 */
function RunLocation({ shellRoot }: { shellRoot: string }) {
  const [project, setProject] = useState(() =>
    projectNameFor(shellRoot),
  );
  const [branch, setBranch] = useState("");

  useEffect(() => {
    setProject(projectNameFor(shellRoot));
    setBranch("");
    const controller = new AbortController();
    fetchWorktrees(shellRoot, controller.signal)
      .then((response) => {
        const worktrees = response.worktrees ?? [];
        const main = worktrees.find((info) => info.is_main);
        if (!response.is_git || !main) {
          return;
        }
        setProject(projectNameFor(main.path));
        const current = worktrees.find(
          (info) => info.path === shellRoot,
        );
        if (current && !current.is_main) {
          setBranch(current.branch ?? "");
        }
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, [shellRoot]);

  return (
    <div className="builder-overview__location" title={shellRoot}>
      <span className="builder-overview__location-chip">
        <FolderIcon size={12} />
        <b>{project}</b>
      </span>
      {branch && (
        <span className="builder-overview__location-chip builder-overview__location-chip--branch">
          <BranchIcon size={12} />
          <span className="mono">{branch}</span>
        </span>
      )}
    </div>
  );
}

export function WorkflowOverview({
  workflow,
  runs,
  currentRun,
  onSelectRun,
  onSelectNode,
  resumedFrom,
  resumeNode,
  onOpenExecution,
  shellRoot,
  lastNodeOutput,
}: {
  workflow: WorkflowSummary | undefined;
  runs: RunEntry[];
  currentRun: RunEntry | null;
  onSelectRun: (fullId: string) => void;
  onSelectNode: (nodeId: string) => void;
  /** Set when this run resumed a prior one. */
  resumedFrom?: string;
  resumeNode?: string;
  onOpenExecution?: (executionId: string) => void;
  /** Directory the run executed in — shown as project · workspace. */
  shellRoot?: string;
  /** Fallback when the run has no workflow-level outputs yet. */
  lastNodeOutput?: { nodeId: string; outputs: Record<string, unknown> };
}) {
  const failures = currentRun
    ? Object.entries(currentRun.nodes).filter(
        ([, node]) => node.status === "err",
      )
    : [];
  const workflowOutputs = currentRun?.outputs
    ? Object.entries(currentRun.outputs)
    : [];
  // The finished run's declared outputs, or — while running or after a
  // failure — whatever the furthest node has produced so far.
  const usingFallback = workflowOutputs.length === 0 && !!lastNodeOutput;
  const outputs = usingFallback
    ? Object.entries(lastNodeOutput.outputs)
    : workflowOutputs;

  return (
    <div className="builder-inspector">
      <div className="builder-inspector__head">
        <div className="builder-inspector__title">
          <h3>{workflow?.name || workflow?.id || "Workflow"}</h3>
          <span className="mono">{workflow?.id ?? ""}</span>
        </div>
        {currentRun && (
          <StatusPill
            status={currentRun.status}
            runId={currentRun.id}
          />
        )}
      </div>

      <div className="builder-inspector__body scrollnice">
        {workflow?.description && (
          <p className="builder-overview__description">
            {workflow.description}
          </p>
        )}

        {currentRun ? (
          <>
            <div className="builder-inspector__run-meta">
              <span className="mono">{currentRun.id}</span>
              <span>·</span>
              <span>
                {currentRun.whenAbsolute} · {currentRun.when}
              </span>
              {currentRun.duration > 0 && (
                <>
                  <span>·</span>
                  <span>
                    {(currentRun.duration / 1000).toFixed(1)}s
                  </span>
                </>
              )}
            </div>

            {shellRoot && <RunLocation shellRoot={shellRoot} />}

            {resumedFrom && (
              <button
                type="button"
                className="builder-overview__resumed"
                title="Open the run this one resumed from"
                onClick={() => onOpenExecution?.(resumedFrom)}
              >
                ↺ resumed from{" "}
                <b className="mono">{resumedFrom.slice(0, 8)}</b>
                {resumeNode && <> at <b>{resumeNode}</b></>}
              </button>
            )}

            {failures.length > 0 && (
              <>
                <div className="builder-io-meta">
                  <b>Failed nodes</b>
                  <span>{failures.length}</span>
                </div>
                {failures.map(([nodeId, node]) => (
                  <button
                    type="button"
                    key={nodeId}
                    className="builder-overview__failure"
                    onClick={() => onSelectNode(nodeId)}
                  >
                    <b className="mono">{nodeId}</b>
                    {node.error && <span>{node.error}</span>}
                  </button>
                ))}
              </>
            )}

            <div className="builder-io-meta">
              <b>Outputs</b>
              <span>
                {currentRun.status === "run"
                  ? "running…"
                  : currentRun.status === "ok"
                    ? "completed"
                    : currentRun.status === "err"
                      ? "incomplete"
                      : ""}
              </span>
            </div>
            {usingFallback && (
              <button
                type="button"
                className="builder-overview__outputs-source"
                title="Show this node on the canvas"
                onClick={() => onSelectNode(lastNodeOutput.nodeId)}
              >
                latest from{" "}
                <b className="mono">{lastNodeOutput.nodeId}</b>
              </button>
            )}
            {outputs.length === 0 && (
              <div className="builder-io-card builder-io-card--output">
                —
              </div>
            )}
            {outputs.map(([name, value]) => {
              const text = formatValue(value);
              return (
                <div key={name} className="builder-io-field">
                  <div className="builder-io-field__head">
                    <b>{name}</b>
                    <CopyButton value={text} />
                  </div>
                  <div className="builder-io-card builder-io-card--output">
                    {text}
                  </div>
                </div>
              );
            })}
          </>
        ) : (
          <div className="builder-overview__empty">
            No runs yet — run the workflow to see what it produces
            here.
          </div>
        )}

        <div className="builder-io-meta">
          <b>Runs</b>
          <span>{runs.length}</span>
        </div>
        <div className="builder-inspector-runs">
          {runs.length === 0 && (
            <div className="builder-inspector-runs__empty">
              No runs yet.
            </div>
          )}
          {runs.map((run) => (
            <button
              type="button"
              key={run.fullId}
              className={
                run.fullId === currentRun?.fullId ? "active" : ""
              }
              onClick={() => onSelectRun(run.fullId)}
            >
              <StatusPill status={run.status} />
              <span className="builder-inspector-runs__identity">
                <b className="mono">{run.id}</b>
                <small>
                  {run.when} · {run.whenAbsolute}
                </small>
              </span>
              <span className="builder-inspector-runs__metrics mono">
                <b>
                  {run.duration
                    ? `${(run.duration / 1000).toFixed(1)}s`
                    : "—"}
                </b>
              </span>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
