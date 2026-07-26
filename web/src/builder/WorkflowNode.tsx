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

// The header sub already shows kind · model.
const REDUNDANT_FIELDS = new Set(["kind", "model"]);

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

  const fields = node.body.filter(
    (field) => !REDUNDANT_FIELDS.has(field.k),
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
        <div
          className="builder-node__icon"
          style={{
            background: visual.background,
            color: visual.foreground,
          }}
        >
          <KindIcon kind={node.kind} size={13} />
        </div>
        <div className="builder-node__title">
          <div className="builder-node__name">{node.name}</div>
          <div className="builder-node__sub">{node.sub}</div>
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

      {(fields.length > 0 ||
        preview !== null ||
        (node.isGroup && node.groupLabel)) && (
        <div className="builder-node__body">
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
