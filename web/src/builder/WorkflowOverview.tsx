import type { WorkflowSummary } from "../types/api";
import { CopyButton, formatValue } from "./Inspector";
import { type RunEntry } from "./runData";
import { StatusPill } from "./RunMenu";

export function WorkflowOverview({
  workflow,
  runs,
  currentRun,
  onSelectRun,
  onSelectNode,
}: {
  workflow: WorkflowSummary | undefined;
  runs: RunEntry[];
  currentRun: RunEntry | null;
  onSelectRun: (fullId: string) => void;
  onSelectNode: (nodeId: string) => void;
}) {
  const failures = currentRun
    ? Object.entries(currentRun.nodes).filter(
        ([, node]) => node.status === "err",
      )
    : [];
  const outputs = currentRun?.outputs
    ? Object.entries(currentRun.outputs)
    : [];

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
