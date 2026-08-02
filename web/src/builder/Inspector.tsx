import {
  type MouseEvent,
  useEffect,
  useState,
} from "react";
import { fetchHarnesses } from "../api";
import type { HarnessInfo } from "../types/api";
import type { CanvasNode } from "../types/workflow";
import { copyText } from "../utils/clipboard";
import { KIND_VISUAL } from "./constants";
import { KindIcon } from "./Icons";
import { ModelPicker } from "./ModelPicker";
import { type RunEntry } from "./runData";
import { RunMenuDropdown, StatusPill } from "./RunMenu";

type InspectorTab = "overview" | "config" | "runs";

/** Saves one node's editable config; resolves when the YAML is written. */
export type NodeConfigSave = (
  nodeName: string,
  update: { model?: string; harness?: string; instruction?: string },
) => Promise<void>;

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

/**
 * Editable config for an inference node. Saves through the node-update
 * endpoint, which edits the YAML surgically and re-compiles before
 * persisting — a bad value never lands.
 */
function EditableLLMConfig({
  node,
  onSave,
}: {
  node: CanvasNode;
  onSave: NodeConfigSave;
}) {
  const config = node.config;
  const [model, setModel] = useState(String(config.model || ""));
  const [harness, setHarness] = useState(String(config.harnessId || ""));
  const [instruction, setInstruction] = useState(
    String(config.instruction || ""),
  );
  const [harnesses, setHarnesses] = useState<HarnessInfo[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  // Re-sync when the selection moves or the YAML changes underneath.
  useEffect(() => {
    setModel(String(node.config.model || ""));
    setHarness(String(node.config.harnessId || ""));
    setInstruction(String(node.config.instruction || ""));
    setError("");
  }, [node.id, node.config.model, node.config.harnessId, node.config.instruction]);

  useEffect(() => {
    const controller = new AbortController();
    fetchHarnesses(controller.signal)
      .then((response) => setHarnesses(response?.harnesses ?? []))
      .catch(() => undefined);
    return () => controller.abort();
  }, []);

  const dirty =
    model !== String(config.model || "") ||
    harness !== String(config.harnessId || "") ||
    instruction !== String(config.instruction || "");
  const knownModels =
    harnesses.find((info) => info.id === harness)?.models ?? [];

  async function save() {
    if (!dirty || saving) {
      return;
    }
    setSaving(true);
    setError("");
    try {
      await onSave(node.name, {
        model: model !== String(config.model || "") ? model : undefined,
        harness:
          harness !== String(config.harnessId || "")
            ? harness
            : undefined,
        instruction:
          instruction !== String(config.instruction || "")
            ? instruction
            : undefined,
      });
    } catch (caught) {
      setError(
        caught instanceof Error ? caught.message : String(caught),
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <div>
      <div className="builder-field-row">
        <label>Harness</label>
        {harnesses.length > 0 ? (
          <select
            className="builder-select mono"
            value={harness}
            onChange={(event) => {
              const next = event.target.value;
              setHarness(next);
              // A model from another harness makes no sense here —
              // snap to the new harness's lead model.
              const nextModels =
                harnesses.find((info) => info.id === next)?.models ??
                [];
              if (
                nextModels.length > 0 &&
                !nextModels.includes(model)
              ) {
                setModel(nextModels[0] ?? "");
              }
            }}
          >
            {!harnesses.some((info) => info.id === harness) && (
              <option value={harness}>{harness}</option>
            )}
            {harnesses.map((info) => (
              <option key={info.id} value={info.id}>
                {info.id}
                {info.available ? "" : " (not installed)"}
              </option>
            ))}
          </select>
        ) : (
          <input
            className="builder-input mono"
            value={harness}
            onChange={(event) => setHarness(event.target.value)}
          />
        )}
      </div>
      <div className="builder-field-row">
        <label>Model</label>
        <ModelPicker
          value={model}
          models={knownModels}
          onChange={setModel}
        />
      </div>
      <div className="builder-field-row">
        <label>System prompt</label>
        <textarea
          className="builder-textarea"
          rows={14}
          value={instruction}
          onChange={(event) => setInstruction(event.target.value)}
        />
      </div>
      {error && (
        <div className="builder-inspector__error">
          <strong>Save failed</strong>
          {error}
        </div>
      )}
      <div className="builder-config-actions">
        <button
          type="button"
          className="builder-run-button"
          disabled={!dirty || saving}
          onClick={() => void save()}
        >
          {saving ? "Saving…" : "Save changes"}
        </button>
        {dirty && !saving && (
          <span className="builder-config-actions__hint">
            Unsaved changes
          </span>
        )}
      </div>
    </div>
  );
}

function Config({
  node,
  onSave,
}: {
  node: CanvasNode;
  onSave?: NodeConfigSave;
}) {
  const config = node.config;
  if (node.kind === "llm") {
    // Nodes inside a composed sub-workflow belong to that workflow's
    // YAML; editing them here would target the wrong file. Loop and
    // conditional targets live in this file, so they stay editable.
    if (onSave && !node.foreign) {
      return <EditableLLMConfig node={node} onSave={onSave} />;
    }
    return (
      <div>
        {onSave && node.foreign && (
          <div className="builder-config-note">
            Defined in a composed workflow — open that workflow to
            edit it.
          </div>
        )}
        <div className="builder-field-row">
          <label>Model</label>
          <input
            className="builder-input mono"
            value={String(config.model || "")}
            readOnly
            title="Configuration is read from workflow YAML"
          />
        </div>
        <div className="builder-field-row">
          <label>Harness</label>
          <input
            className="builder-input mono"
            value={String(config.harnessId || "")}
            readOnly
            title="Configuration is read from workflow YAML"
          />
        </div>
        <div className="builder-field-row">
          <label>System prompt</label>
          <textarea
            className="builder-textarea"
            rows={10}
            value={String(config.instruction || "")}
            readOnly
            title="Configuration is read from workflow YAML"
          />
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

export function NodeConfigPanel({
  node,
  onSave,
}: {
  node: CanvasNode | null;
  /** When set, inference-node config becomes editable. */
  onSave?: NodeConfigSave;
}) {
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
      {!onSave && (
        <div className="builder-config-note">YAML-backed preview</div>
      )}
      <div className="builder-inspector__body scrollnice">
        <Config node={node} onSave={onSave} />
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
  onRerunFrom,
}: {
  node: CanvasNode | null;
  currentRun: RunEntry | null;
  runs: RunEntry[];
  onSelectRun: (fullId: string) => void;
  /** "Re-run from here": this node plus everything downstream. */
  onRerunFrom?: (nodeId: string) => void;
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
        {onRerunFrom &&
          node.kind !== "trigger" &&
          node.kind !== "input" &&
          node.kind !== "output" &&
          status !== "run" && (
          <button
            type="button"
            className="builder-ghost-button builder-inspector__rerun"
            title="Re-run this node and everything downstream of it"
            onClick={() => onRerunFrom(node.id)}
          >
            ↺ Re-run
          </button>
        )}
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
