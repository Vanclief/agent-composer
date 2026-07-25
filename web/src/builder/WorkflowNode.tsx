import { useState } from "react";
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
import { RunMenu, StatusPill } from "./RunMenu";

function NodePorts({
  ports,
  side,
}: {
  ports: WorkflowFlowNode["data"]["canvas"]["inputs"];
  side: "in" | "out";
}) {
  return (
    <div className={`builder-ports builder-ports--${side}`}>
      {ports.map((port) => (
        <div
          key={port.id}
          className={`builder-port builder-port--${side} builder-port--${port.type}`}
        >
          <Handle
            id={port.id}
            type={side === "in" ? "target" : "source"}
            position={side === "in" ? Position.Left : Position.Right}
            isConnectable={false}
            className="builder-port__handle"
          />
          <span className="builder-port__label">{port.label}</span>
          <span className="builder-port__type">{port.type}</span>
        </div>
      ))}
    </div>
  );
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
    onViewRuns,
    runs,
  } = useBuilderRuntime();
  const [menuOpen, setMenuOpen] = useState(false);
  const node = data.canvas;
  const snapshot = currentRun?.nodes[node.id];
  const status = snapshot?.status ?? node.last.status ?? "idle";
  const visual = KIND_VISUAL[node.kind];
  const preview = (() => {
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
    const value = output[key];
    if (typeof value === "string") {
      return value;
    }
    if (Array.isArray(value)) {
      return `[${value.length}] ${String(value[0] ?? "")}`;
    }
    return value == null ? String(value) : JSON.stringify(value);
  })();

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
        <span
          className="builder-runmenu-anchor"
          onMouseDown={(event) => event.stopPropagation()}
        >
          <StatusPill
            status={status}
            tokens={snapshot?.tokens}
            milliseconds={snapshot?.ms}
            runId={currentRun?.id}
            onClick={
              status === "run"
                ? undefined
                : (event) => {
                    event.stopPropagation();
                    setMenuOpen((value) => !value);
                  }
            }
          />
          {menuOpen && (
            <RunMenu
              runs={runs}
              currentFullId={currentRun?.fullId}
              nodeId={node.id}
              onPick={(fullId) => {
                setMenuOpen(false);
                onSelectRun(fullId);
              }}
              onClose={() => setMenuOpen(false)}
              onViewAll={onViewRuns}
              align="right"
            />
          )}
        </span>
      </div>

      <div className="builder-node__body">
        {node.body.map((field, index) => (
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

      {node.inputs.length > 0 && (
        <NodePorts ports={node.inputs} side="in" />
      )}
      {node.outputs.length > 0 && (
        <NodePorts ports={node.outputs} side="out" />
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
