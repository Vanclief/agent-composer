import {
  type CSSProperties,
  type FormEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  fetchNodeExecutions,
  fetchWorkflowExecution,
  fetchWorkflowExecutions,
} from "../api";
import type {
  NodeExecution,
  WorkflowExecution,
} from "../types/api";
import { copyText } from "../utils/clipboard";

const RUNNING_STATUSES = new Set(["running", "queued"]);

function statusClass(status?: string) {
  return String(status ?? "").toLowerCase();
}

function StatusPill({ status }: { status?: string }) {
  const value = status || "unknown";
  return (
    <span className={`runs-pill ${statusClass(value)}`}>{value}</span>
  );
}

function formatTime(value?: string) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function formatDuration(start?: string, finish?: string) {
  if (!start) {
    return "";
  }
  const startDate = new Date(start);
  const endDate = finish ? new Date(finish) : new Date();
  if (
    Number.isNaN(startDate.getTime()) ||
    Number.isNaN(endDate.getTime())
  ) {
    return "";
  }

  const milliseconds = Math.max(
    0,
    endDate.getTime() - startDate.getTime(),
  );
  if (milliseconds < 1000) {
    return `${milliseconds} ms`;
  }
  if (milliseconds < 60000) {
    return `${(milliseconds / 1000).toFixed(1)} s`;
  }
  return `${Math.floor(milliseconds / 60000)}m ${Math.round(
    (milliseconds % 60000) / 1000,
  )}s`;
}

function pretty(value: unknown) {
  if (value === null || value === undefined) {
    return "";
  }
  return JSON.stringify(value, null, 2);
}

function compactID(value?: string) {
  const raw = String(value ?? "");
  if (raw.length <= 18) {
    return raw;
  }
  return `${raw.slice(0, 8)}...${raw.slice(-6)}`;
}

function sortByTime(items: NodeExecution[]) {
  return [...items].sort((left, right) => {
    const leftTime =
      Date.parse(left.started_at || left.created_at || "") || 0;
    const rightTime =
      Date.parse(right.started_at || right.created_at || "") || 0;
    if (leftTime !== rightTime) {
      return leftTime - rightTime;
    }
    return String(left.id || "").localeCompare(String(right.id || ""));
  });
}

function buildDepths(nodes: NodeExecution[]) {
  const byID = new Map(nodes.map((node) => [node.id, node]));
  const depths = new Map<string, number>();
  const visiting = new Set<string>();

  function depth(node?: NodeExecution): number {
    if (!node?.parent_node_execution_id) {
      return 0;
    }
    const cached = depths.get(node.id);
    if (cached !== undefined) {
      return cached;
    }
    if (visiting.has(node.id)) {
      return 0;
    }

    visiting.add(node.id);
    const parent = byID.get(node.parent_node_execution_id);
    const value = parent ? depth(parent) + 1 : 0;
    visiting.delete(node.id);
    depths.set(node.id, value);
    return value;
  }

  for (const node of nodes) {
    depths.set(node.id, depth(node));
  }
  return depths;
}

function Metric({
  label,
  children,
}: {
  label: string;
  children?: ReactNode;
}) {
  return (
    <div className="runs-metric">
      <div className="runs-metric__label">{label}</div>
      <div className="runs-metric__value">{children}</div>
    </div>
  );
}

function JsonBox({
  title,
  value,
  onCopyError,
}: {
  title: string;
  value: unknown;
  onCopyError: (error: Error) => void;
}) {
  const text = pretty(value);
  return (
    <div className="runs-json-box">
      <header>
        <h3>{title}</h3>
        <button
          type="button"
          onClick={() => {
            copyText(text).catch((error: unknown) => {
              onCopyError(
                error instanceof Error ? error : new Error(String(error)),
              );
            });
          }}
        >
          Copy
        </button>
      </header>
      <pre className="mono">{text}</pre>
    </div>
  );
}

function NodeCard({
  node,
  index,
  depth,
  onCopyError,
}: {
  node: NodeExecution;
  index: number;
  depth: number;
  onCopyError: (error: Error) => void;
}) {
  const meta = [
    node.kind,
    node.branch_name ? `branch ${node.branch_name}` : "",
    node.iteration_index !== undefined
      ? `iteration ${node.iteration_index}`
      : "",
    formatDuration(node.started_at, node.finished_at),
  ].filter(Boolean);
  const style = { "--depth": depth } as CSSProperties;
  const [open, setOpen] = useState(index < 3);

  return (
    <details
      className="runs-node"
      open={open}
      onToggle={(event) => setOpen(event.currentTarget.open)}
      style={style}
    >
      <summary>
        <div>
          <div className="runs-node-title">
            <strong className="runs-truncate">
              {node.node_id || node.id}
            </strong>
            <StatusPill status={node.status} />
          </div>
          <div className="runs-node-meta runs-small">
            {meta.map((item) => (
              <span key={item}>{item}</span>
            ))}
          </div>
        </div>
        <span className="runs-small mono">{compactID(node.id)}</span>
      </summary>
      <div className="runs-node-body">
        <div className="runs-json-grid">
          <JsonBox
            title="Input"
            value={node.input_snapshot}
            onCopyError={onCopyError}
          />
          <JsonBox
            title="Output"
            value={node.output_snapshot}
            onCopyError={onCopyError}
          />
        </div>
        {node.trace && (
          <>
            <div className="runs-section-head">
              <h3>Trace</h3>
            </div>
            <div className="runs-json-box">
              <pre className="mono">{pretty(node.trace)}</pre>
            </div>
          </>
        )}
      </div>
    </details>
  );
}

export function RunsPage() {
  const navigate = useNavigate();
  const { executionId } = useParams();
  const [lookup, setLookup] = useState(executionId ?? "");
  const [workflowFilter, setWorkflowFilter] = useState("");
  const [appliedWorkflowFilter, setAppliedWorkflowFilter] = useState("");
  const [recent, setRecent] = useState<WorkflowExecution[]>([]);
  const [recentLoading, setRecentLoading] = useState(true);
  const [execution, setExecution] = useState<WorkflowExecution | null>(
    null,
  );
  const [nodes, setNodes] = useState<NodeExecution[]>([]);
  const [detailLoading, setDetailLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    document.title = "Workflow Runs";
  }, []);

  useEffect(() => {
    setLookup(executionId ?? "");
    setExecution(null);
    setNodes([]);
  }, [executionId]);

  const loadRecent = useCallback(async () => {
    setError("");
    setRecentLoading(true);
    try {
      const items = await fetchWorkflowExecutions(
        appliedWorkflowFilter || undefined,
        20,
      );
      setRecent(items);
    } catch (caught) {
      setRecent([]);
      setError(
        caught instanceof Error ? caught.message : String(caught),
      );
    } finally {
      setRecentLoading(false);
    }
  }, [appliedWorkflowFilter]);

  useEffect(() => {
    void loadRecent();
  }, [loadRecent]);

  const loadExecution = useCallback(
    async (silent = false) => {
      if (!executionId) {
        return;
      }
      setError("");
      if (!silent) {
        setDetailLoading(true);
      }

      try {
        const [nextExecution, nextNodes] = await Promise.all([
          fetchWorkflowExecution(executionId),
          fetchNodeExecutions(executionId, 200),
        ]);
        setExecution(nextExecution);
        setNodes(sortByTime(nextNodes));
        setRecent((items) =>
          items.map((item) =>
            item.id === nextExecution.id ? nextExecution : item,
          ),
        );
      } catch (caught) {
        setError(
          caught instanceof Error ? caught.message : String(caught),
        );
      } finally {
        if (!silent) {
          setDetailLoading(false);
        }
      }
    },
    [executionId],
  );

  useEffect(() => {
    void loadExecution();
  }, [loadExecution]);

  useEffect(() => {
    if (
      !autoRefresh ||
      !executionId ||
      !RUNNING_STATUSES.has(statusClass(execution?.status))
    ) {
      return;
    }
    const timer = window.setInterval(() => {
      void loadExecution(true);
    }, 2500);
    return () => window.clearInterval(timer);
  }, [autoRefresh, execution?.status, executionId, loadExecution]);

  const depths = useMemo(() => buildDepths(nodes), [nodes]);

  function handleLookup(event: FormEvent) {
    event.preventDefault();
    const id = lookup.trim();
    if (id) {
      navigate(`/runs/${encodeURIComponent(id)}`);
    }
  }

  function showCopyError(copyError: Error) {
    setError(copyError.message || "Could not copy to clipboard");
  }

  return (
    <div className="runs-page">
      <div className="runs-shell">
        <aside className="runs-sidebar">
          <div className="runs-topbar">
            <h1>Workflow Runs</h1>
            <button
              className="runs-icon"
              type="button"
              title="Reload recent executions"
              aria-label="Reload recent executions"
              onClick={() => void loadRecent()}
            >
              R
            </button>
          </div>

          <form className="runs-load-form" onSubmit={handleLookup}>
            <input
              className="mono"
              name="execution_id"
              autoComplete="off"
              placeholder="execution id"
              aria-label="Execution ID"
              value={lookup}
              onChange={(event) => setLookup(event.target.value)}
            />
            <button className="runs-primary" type="submit">
              Load
            </button>
          </form>

          <input
            className="runs-filter"
            autoComplete="off"
            placeholder="workflow id filter"
            aria-label="Workflow ID filter"
            value={workflowFilter}
            onChange={(event) => setWorkflowFilter(event.target.value)}
            onBlur={() =>
              setAppliedWorkflowFilter(workflowFilter.trim())
            }
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                setAppliedWorkflowFilter(workflowFilter.trim());
              }
            }}
          />

          <div className="runs-row">
            <h2>Recent Runs</h2>
            <span className="runs-small">{recent.length}</span>
          </div>
          <div className="runs-recent-list">
            {recentLoading ? (
              <div className="runs-empty">Loading executions.</div>
            ) : recent.length === 0 ? (
              <div className="runs-empty">No executions found.</div>
            ) : (
              recent.map((item) => (
                <button
                  key={item.id}
                  className={`runs-recent-item ${
                    item.id === executionId ? "active" : ""
                  }`}
                  type="button"
                  onClick={() =>
                    navigate(`/runs/${encodeURIComponent(item.id)}`)
                  }
                >
                  <span className="runs-row">
                    <span className="runs-truncate">
                      {item.workflow_id}
                    </span>
                    <StatusPill status={item.status} />
                  </span>
                  <span className="runs-small mono runs-truncate">
                    {compactID(item.id)}
                  </span>
                  <span className="runs-small">
                    {formatTime(item.started_at || item.created_at)}
                  </span>
                </button>
              ))
            )}
          </div>
        </aside>

        <main className="runs-main">
          {error && <div className="runs-error">{error}</div>}

          <div className="runs-topbar">
            <div>
              <h2>{execution?.workflow_id || "Execution"}</h2>
              <div className="runs-small mono">
                {execution?.id || executionId || ""}
              </div>
            </div>
            <div className="runs-toolbar">
              <label className="runs-check">
                <input
                  type="checkbox"
                  checked={autoRefresh}
                  onChange={(event) =>
                    setAutoRefresh(event.target.checked)
                  }
                />
                Auto refresh
              </label>
              <button
                type="button"
                disabled={!executionId}
                onClick={() => void loadExecution()}
              >
                Refresh
              </button>
            </div>
          </div>

          {!executionId ? (
            <div className="runs-empty">Select a workflow execution.</div>
          ) : detailLoading && !execution ? (
            <div className="runs-empty">Loading execution.</div>
          ) : execution ? (
            <section>
              <div className="runs-summary-grid">
                <Metric label="Status">
                  <StatusPill status={execution.status} />
                </Metric>
                <Metric label="Workflow">{execution.workflow_id}</Metric>
                <Metric label="Version">
                  {execution.workflow_version}
                </Metric>
                <Metric label="Duration">
                  {formatDuration(
                    execution.started_at,
                    execution.finished_at,
                  )}
                </Metric>
                <Metric label="Started">
                  {formatTime(execution.started_at)}
                </Metric>
                <Metric label="Finished">
                  {formatTime(execution.finished_at)}
                </Metric>
                <Metric label="Execution ID">
                  <span className="mono">{execution.id}</span>
                </Metric>
                <Metric label="Nodes">{nodes.length}</Metric>
              </div>

              <div className="runs-section-head">
                <h2>Workflow Snapshots</h2>
              </div>
              <div className="runs-json-grid">
                <JsonBox
                  title="Input"
                  value={execution.input_snapshot}
                  onCopyError={showCopyError}
                />
                <JsonBox
                  title="Output"
                  value={execution.output_snapshot}
                  onCopyError={showCopyError}
                />
              </div>

              <div className="runs-section-head">
                <h2>Node Executions</h2>
                <span className="runs-small">{nodes.length} nodes</span>
              </div>
              <div className="runs-node-list">
                {nodes.length === 0 ? (
                  <div className="runs-empty">
                    No node executions found.
                  </div>
                ) : (
                  nodes.map((node, index) => (
                    <NodeCard
                      key={node.id}
                      node={node}
                      index={index}
                      depth={depths.get(node.id) ?? 0}
                      onCopyError={showCopyError}
                    />
                  ))
                )}
              </div>
            </section>
          ) : (
            <div className="runs-empty">
              The execution could not be loaded.
            </div>
          )}
        </main>
      </div>
    </div>
  );
}
