import {
  type MouseEvent,
  useEffect,
  useState,
} from "react";
import type { CanvasNode } from "../types/workflow";
import { copyText } from "../utils/clipboard";
import { KIND_VISUAL } from "./constants";
import { KindIcon } from "./Icons";
import { type RunEntry } from "./runData";
import { RunMenuDropdown, StatusPill } from "./RunMenu";

type InspectorTab = "overview" | "config" | "runs";

export function formatValue(value: unknown) {
  if (value === null || value === undefined) {
    return "—";
  }
  if (typeof value === "string") {
    return value;
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export function CopyButton({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  if (!value || value === "—") {
    return null;
  }

  async function handleCopy(event: MouseEvent<HTMLButtonElement>) {
    event.stopPropagation();
    const success = await copyText(value);
    if (success) {
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1200);
    }
  }

  return (
    <button
      type="button"
      className={`builder-copy-button ${copied ? "copied" : ""}`}
      title="Copy to clipboard"
      onClick={(event) => void handleCopy(event)}
    >
      {copied ? "Copied" : "Copy"}
    </button>
  );
}

function LiveIO({
  node,
  currentRun,
}: {
  node: CanvasNode;
  currentRun: RunEntry | null;
}) {
  const snapshot = currentRun?.nodes[node.id];
  const status = snapshot?.status ?? node.last.status ?? "idle";
  const tokens = snapshot?.tokens ?? 0;
  const milliseconds = snapshot?.ms ?? node.last.ms ?? 0;
  const error = snapshot?.error ?? node.last.error;

  return (
    <div>
      <div className="builder-inspector__run-meta">
        <span className="mono">{currentRun?.id || "—"}</span>
        <span>·</span>
        <span>
          {currentRun?.whenAbsolute || "—"} · {currentRun?.when || "—"}
        </span>
      </div>

      {error && (
        <div className="builder-inspector__error">
          <strong>Error</strong>
          {error}
        </div>
      )}

      <div className="builder-stat-grid">
        <div className="builder-stat">
          <span>Tokens</span>
          <b>{tokens ? tokens.toLocaleString() : "—"}</b>
        </div>
        <div className="builder-stat">
          <span>Latency</span>
          <b>
            {milliseconds
              ? milliseconds >= 1000
                ? `${(milliseconds / 1000).toFixed(1)}s`
                : `${milliseconds}ms`
              : "—"}
          </b>
        </div>
        <div className="builder-stat">
          <span>Cost</span>
          <b>{tokens ? `$${(tokens * 0.00001).toFixed(3)}` : "—"}</b>
        </div>
      </div>

      {Boolean(node.config.instruction) && (
        <>
          <div className="builder-io-meta">
            <b>Prompt</b>
            <CopyButton value={String(node.config.instruction)} />
          </div>
          <div className="builder-io-card builder-io-card--prompt">
            {String(node.config.instruction)}
          </div>
        </>
      )}

      {node.inputs.length > 0 && (
        <>
          <div className="builder-io-meta">
            <b>Input</b>
            <span>{currentRun?.whenAbsolute || "—"}</span>
          </div>
          {node.inputs.map((port) => {
            const value = formatValue(snapshot?.inputSnapshot?.[port.id]);
            return (
              <div key={port.id} className="builder-io-field">
                <div className="builder-io-field__head">
                  <b>{port.label}</b>
                  <span className="mono">· {port.type}</span>
                  <CopyButton value={value} />
                </div>
                <div className="builder-io-card builder-io-card--input">
                  {value}
                </div>
              </div>
            );
          })}
        </>
      )}

      <div className="builder-io-meta">
        <b>Output</b>
        <span>
          {status === "run"
            ? "streaming…"
            : status === "ok"
              ? "completed"
              : status}
        </span>
      </div>
      {node.outputs.length === 0 && (
        <div className="builder-io-card builder-io-card--output">—</div>
      )}
      {node.outputs.map((port) => {
        const value = formatValue(snapshot?.outputSnapshot?.[port.id]);
        return (
          <div key={port.id} className="builder-io-field">
            <div className="builder-io-field__head">
              <b>{port.label}</b>
              <span className="mono">· {port.type}</span>
              <CopyButton value={value} />
            </div>
            <div className="builder-io-card builder-io-card--output">
              {status === "run" ? (
                <>
                  {value}
                  <span className="builder-stream-cursor" />
                </>
              ) : status === "idle" ? (
                "—"
              ) : (
                value
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

function Config({ node }: { node: CanvasNode }) {
  const config = node.config;
  if (node.kind === "llm") {
    return (
      <div>
        <div className="builder-field-row">
          <label>Model</label>
          <select
            className="builder-select"
            defaultValue={String(config.model || "")}
            disabled
            title="Configuration is read from workflow YAML"
          >
            <option>{String(config.model || "default")}</option>
            <option>gpt-5</option>
            <option>claude-sonnet-4.5</option>
            <option>claude-opus-4.5</option>
          </select>
        </div>
        <div className="builder-field-row">
          <label>Harness</label>
          <input
            className="builder-input mono"
            defaultValue={String(config.harnessId || "")}
            readOnly
            title="Configuration is read from workflow YAML"
          />
        </div>
        <div className="builder-field-row">
          <label>System prompt</label>
          <textarea
            className="builder-textarea"
            rows={8}
            defaultValue={String(config.instruction || "")}
            readOnly
            title="Configuration is read from workflow YAML"
          />
        </div>
        <div className="builder-field-row">
          <label>Tools</label>
          <div className="builder-segment">
            <button
              type="button"
              className="active"
              disabled
              title="Configuration is read from workflow YAML"
            >
              None
            </button>
            <button
              type="button"
              disabled
              title="Configuration is read from workflow YAML"
            >
              Web
            </button>
            <button
              type="button"
              disabled
              title="Configuration is read from workflow YAML"
            >
              Code
            </button>
            <button
              type="button"
              disabled
              title="Configuration is read from workflow YAML"
            >
              Custom
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="builder-field-row">
        <label>Kind</label>
        <input
          className="builder-input mono"
          defaultValue={String(config.kind || node.kind)}
          readOnly
          title="Configuration is read from workflow YAML"
        />
      </div>
      <div className="builder-field-row">
        <label>Operation</label>
        <input
          className="builder-input mono"
          defaultValue={String(config.operation || "")}
          readOnly
          title="Configuration is read from workflow YAML"
        />
      </div>
      {config.instruction && (
        <div className="builder-field-row">
          <label>Description</label>
          <textarea
            className="builder-textarea"
            rows={6}
            defaultValue={String(config.instruction)}
            readOnly
            title="Configuration is read from workflow YAML"
          />
        </div>
      )}
    </div>
  );
}

export function NodeConfigPanel({ node }: { node: CanvasNode | null }) {
  if (!node) {
    return (
      <div className="builder-inspector">
        <div className="builder-inspector__empty">
          <b>Nothing selected</b>
          <span>Click a node to inspect its YAML-backed config.</span>
        </div>
      </div>
    );
  }

  const visual = KIND_VISUAL[node.kind];
  return (
    <div className="builder-inspector">
      <div className="builder-inspector__head">
        <div
          className="builder-inspector__icon"
          style={{
            background: visual.background,
            color: visual.foreground,
          }}
        >
          <KindIcon kind={node.kind} size={15} />
        </div>
        <div className="builder-inspector__title">
          <h3>{node.name}</h3>
          <span className="mono">
            {node.kind} · {node.id}
          </span>
        </div>
      </div>
      <div className="builder-config-note">
        YAML-backed preview
      </div>
      <div className="builder-inspector__body scrollnice">
        <Config node={node} />
      </div>
    </div>
  );
}

function Runs({
  node,
  runs,
  currentRun,
  onSelectRun,
}: {
  node: CanvasNode;
  runs: RunEntry[];
  currentRun: RunEntry | null;
  onSelectRun: (fullId: string) => void;
}) {
  return (
    <div className="builder-inspector-runs">
      {runs.length === 0 && (
        <div className="builder-inspector-runs__empty">No runs yet.</div>
      )}
      {runs.map((run) => {
        const snapshot = run.nodes[node.id];
        const status = snapshot?.status ?? "idle";
        return (
          <button
            type="button"
            key={run.fullId}
            className={
              run.fullId === currentRun?.fullId ? "active" : ""
            }
            onClick={() => onSelectRun(run.fullId)}
          >
            <StatusPill status={status} />
            <span className="builder-inspector-runs__identity">
              <b className="mono">{run.id}</b>
              <small>
                {run.when} · {run.whenAbsolute}
              </small>
            </span>
            <span className="builder-inspector-runs__metrics mono">
              <b>
                {snapshot?.ms
                  ? snapshot.ms >= 1000
                    ? `${(snapshot.ms / 1000).toFixed(1)}s`
                    : `${snapshot.ms}ms`
                  : "—"}
              </b>
              <small>
                {snapshot?.tokens
                  ? `${snapshot.tokens.toLocaleString()} tok`
                  : "—"}
              </small>
            </span>
          </button>
        );
      })}
    </div>
  );
}

export function Inspector({
  node,
  currentRun,
  runs,
  onSelectRun,
}: {
  node: CanvasNode | null;
  currentRun: RunEntry | null;
  runs: RunEntry[];
  onSelectRun: (fullId: string) => void;
}) {
  const [tab, setTab] = useState<InspectorTab>("overview");

  useEffect(() => {
    setTab("overview");
  }, [node?.id]);

  if (!node) {
    return (
      <div className="builder-inspector">
        <div className="builder-inspector__empty">
          <svg
            width="40"
            height="40"
            viewBox="0 0 40 40"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.4"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <rect x="6" y="9" width="14" height="10" rx="2" />
            <rect x="20" y="21" width="14" height="10" rx="2" />
            <path d="M20 14 H30 M30 14 V19" />
          </svg>
          <b>Nothing selected</b>
          <span>
            Click a node to inspect its config
            <br />
            and live last-run I/O.
          </span>
        </div>
      </div>
    );
  }

  const visual = KIND_VISUAL[node.kind];
  const snapshot = currentRun?.nodes[node.id];
  const status = snapshot?.status ?? node.last.status ?? "idle";

  return (
    <div className="builder-inspector">
      <div className="builder-inspector__head">
        <div
          className="builder-inspector__icon"
          style={{
            background: visual.background,
            color: visual.foreground,
          }}
        >
          <KindIcon kind={node.kind} size={15} />
        </div>
        <div className="builder-inspector__title">
          <h3>{node.name}</h3>
          <span className="mono">
            {node.kind} · {node.id}
          </span>
        </div>
        {status === "run" ? (
          <StatusPill
            status={status}
            tokens={snapshot?.tokens}
            milliseconds={snapshot?.ms}
            runId={currentRun?.id}
          />
        ) : (
          <RunMenuDropdown
            trigger={
              <StatusPill
                status={status}
                tokens={snapshot?.tokens}
                milliseconds={snapshot?.ms}
                runId={currentRun?.id}
                interactive
              />
            }
            runs={runs}
            currentFullId={currentRun?.fullId}
            nodeId={node.id}
            onPick={onSelectRun}
          />
        )}
      </div>

      <div className="builder-tabs">
        <button
          type="button"
          className={tab === "overview" ? "active" : ""}
          onClick={() => setTab("overview")}
        >
          Overview
        </button>
        <button
          type="button"
          className={tab === "config" ? "active" : ""}
          onClick={() => setTab("config")}
        >
          Config
        </button>
        <button
          type="button"
          className={tab === "runs" ? "active" : ""}
          onClick={() => setTab("runs")}
        >
          Runs <span>{runs.length}</span>
        </button>
      </div>

      <div className="builder-inspector__body scrollnice">
        {tab === "overview" && (
          <LiveIO node={node} currentRun={currentRun} />
        )}
        {tab === "config" && <Config node={node} />}
        {tab === "runs" && (
          <Runs
            node={node}
            runs={runs}
            currentRun={currentRun}
            onSelectRun={onSelectRun}
          />
        )}
      </div>
    </div>
  );
}
