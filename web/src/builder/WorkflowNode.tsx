import {
  Handle,
  type NodeProps,
  Position,
} from "@xyflow/react";
import { KIND_VISUAL } from "./constants";
import { useBuilderRuntime } from "./BuilderContext";
import type {
  GroupBoxFlowNode,
  WorkflowFlowNode,
} from "./flowTypes";
import { KindIcon } from "./Icons";
import { RunMenuDropdown, StatusPill } from "./RunMenu";

// kind is already the header subtitle; harness and model render as
// dedicated tag rows.
const SPECIAL_FIELDS = new Set(["kind", "model", "harness"]);

/** Chip tint by model maker (anthropic orange, openai teal, …). */
function vendorClass(value: string) {
  const v = value.toLowerCase();
  if (v.includes("claude") || v.includes("anthropic")) {
    return "anthropic";
  }
  if (
    v.includes("gpt") ||
    v.includes("codex") ||
    v.includes("openai") ||
    /^o\d/.test(v)
  ) {
    return "openai";
  }
  if (v.includes("gemini") || v.includes("google")) {
    return "google";
  }
  if (v.includes("llama") || v.includes("meta")) {
    return "meta";
  }
  if (v.includes("mistral")) {
    return "mistral";
  }
  if (v.includes("deepseek")) {
    return "deepseek";
  }
  return "neutral";
}

function outputPreview(value: unknown): string | null {
  if (typeof value === "string") {
    return value.trim() || null;
  }
  if (Array.isArray(value)) {
    if (value.length === 0) {
      return null;
    }
    return `${value.length} item${value.length === 1 ? "" : "s"}`;
  }
  if (value && typeof value === "object") {
    const keys = Object.keys(value);
    if (keys.length === 0) {
      return null;
    }
    return `{ ${keys.join(", ")} }`;
  }
  return null;
}

export function WorkflowNode({
  data,
  selected,
}: NodeProps<WorkflowFlowNode>) {
  const {
    currentRun,
    expandedGroups,
    onSelectRun,
    onToggleGroup,
    runs,
    showRunStatus,
  } = useBuilderRuntime();
  const node = data.canvas;
  const snapshot = currentRun?.nodes[node.id];
  const status = showRunStatus
    ? snapshot?.status ?? node.last.status ?? "idle"
    : "idle";
  const visual = KIND_VISUAL[node.kind];
  const preview = (() => {
    if (!showRunStatus) {
      return null;
    }
    if (status === "run") {
      return "⋯ streaming";
    }
    const output = snapshot?.outputSnapshot ?? node.last.output;
    if (!output) {
      return null;
    }
    const key = Object.keys(output)[0];
    if (!key) {
      return null;
    }
    return outputPreview(output[key]);
  })();

  const harnessField = node.body.find((field) => field.k === "harness");
  const modelField = node.body.find((field) => field.k === "model");
  const fields = node.body.filter(
    (field) => !SPECIAL_FIELDS.has(field.k),
  );

  const classes = [
    "builder-node",
    selected ? "selected" : "",
    status === "run" ? "running" : "",
    status === "ok" ? "succeeded" : "",
    status === "err" ? "failed" : "",
    node.isGroup ? "group" : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={classes}>
      <div className="builder-node__head">
        <div className="builder-node__head-row">
          <div
            className="builder-node__icon"
            style={{
              background: visual.background,
              color: visual.foreground,
            }}
          >
            <KindIcon kind={node.kind} size={13} />
          </div>
          <div className="builder-node__name" title={node.name}>
            {node.name}
          </div>
        {showRunStatus &&
          (status === "run" || runs.length < 2 ? (
            <StatusPill
              status={status}
              tokens={snapshot?.tokens}
              milliseconds={snapshot?.ms}
              runId={currentRun?.id}
            />
          ) : (
            <span
              onMouseDown={(event) => event.stopPropagation()}
              onClick={(event) => event.stopPropagation()}
            >
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
            </span>
          ))}
        </div>
        <div className="builder-node__sub" title={node.sub}>
          {node.sub}
        </div>
      </div>

      {(fields.length > 0 ||
        harnessField !== undefined ||
        modelField !== undefined ||
        preview !== null ||
        (node.isGroup && node.groupLabel)) && (
        <div className="builder-node__body">
          {harnessField && (
            <div className="builder-node__field">
              <span>harness</span>
              <b
                className={`builder-node__tag builder-node__tag--${vendorClass(harnessField.v)}`}
              >
                {harnessField.v}
              </b>
            </div>
          )}
          {modelField && (
            <div className="builder-node__field">
              <span>model</span>
              <b
                className={`builder-node__tag builder-node__tag--${vendorClass(modelField.v)}`}
              >
                {modelField.v}
              </b>
            </div>
          )}
          {fields.map((field, index) => (
            <div key={`${field.k}-${index}`} className="builder-node__field">
              <span>{field.k}</span>
              <span className={field.mono ? "mono" : ""}>{field.v}</span>
            </div>
          ))}
          {node.isGroup && node.groupLabel && (
            <button
              type="button"
              className="builder-group-toggle nodrag nopan"
              onClick={(event) => {
                event.stopPropagation();
                onToggleGroup(node.id);
              }}
            >
              <span
                className={
                  expandedGroups.has(node.id)
                    ? "builder-group-toggle__arrow expanded"
                    : "builder-group-toggle__arrow"
                }
              >
                ▶
              </span>
              {node.groupLabel}
              {node.childCount ? ` (${node.childCount} nodes)` : ""}
            </button>
          )}
          {preview !== null && (
            <div className="builder-node__preview">{preview}</div>
          )}
        </div>
      )}

      {(node.inputs.length > 0 || node.outputs.length > 0) && (
        <div className="builder-node__ports">
          <div className="builder-node__ports-col builder-node__ports-col--in">
            {node.inputs.map((port) => (
              <div
                key={port.id}
                className={`builder-port builder-port--in builder-port--${port.type}`}
              >
                <Handle
                  id={port.id}
                  type="target"
                  position={Position.Left}
                  isConnectable={false}
                  className="builder-port__handle"
                />
                <span
                  className="builder-port__label"
                  title={`${port.label} · ${port.type}`}
                >
                  {port.label}
                </span>
              </div>
            ))}
          </div>
          <div className="builder-node__ports-col builder-node__ports-col--out">
            {node.outputs.map((port) => (
              <div
                key={port.id}
                className={`builder-port builder-port--out builder-port--${port.type}`}
              >
                <span
                  className="builder-port__label"
                  title={`${port.label} · ${port.type}`}
                >
                  {port.label}
                </span>
                <Handle
                  id={port.id}
                  type="source"
                  position={Position.Right}
                  isConnectable={false}
                  className="builder-port__handle"
                />
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

export function GroupBoxNode({ data }: NodeProps<GroupBoxFlowNode>) {
  return (
    <div className="builder-group-box">
      <span>{data.label}</span>
    </div>
  );
}
