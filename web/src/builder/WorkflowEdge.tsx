import {
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  getStraightPath,
  type EdgeProps,
} from "@xyflow/react";
import type { WorkflowFlowEdge } from "./flowTypes";

export function WorkflowEdge(props: EdgeProps<WorkflowFlowEdge>) {
  const {
    data,
    interactionWidth,
    markerEnd,
    markerStart,
    sourcePosition,
    sourceX,
    sourceY,
    targetPosition,
    targetX,
    targetY,
  } = props;

  let path: string;
  let labelX = (sourceX + targetX) / 2;
  let labelY = (sourceY + targetY) / 2;
  if (data?.wireStyle === "straight") {
    [path] = getStraightPath({
      sourceX,
      sourceY,
      targetX,
      targetY,
    });
  } else if (data?.wireStyle === "orthogonal") {
    const middleX = (sourceX + targetX) / 2;
    path = `M ${sourceX} ${sourceY} L ${middleX} ${sourceY} L ${middleX} ${targetY} L ${targetX} ${targetY}`;
  } else if (data?.wireStyle === "return") {
    // A loop's feedback wire: out the right of the exit boundary,
    // under the whole graph, back up into the entry boundary.
    const drop =
      typeof data?.dropY === "number"
        ? data.dropY
        : Math.max(sourceY, targetY) + 64;
    const rightX = sourceX + 28;
    const leftX = targetX - 28;
    path =
      `M ${sourceX} ${sourceY} L ${rightX} ${sourceY} ` +
      `L ${rightX} ${drop} L ${leftX} ${drop} ` +
      `L ${leftX} ${targetY} L ${targetX} ${targetY}`;
    labelY = drop;
  } else {
    [path, labelX, labelY] = getBezierPath({
      sourceX,
      sourceY,
      sourcePosition,
      targetX,
      targetY,
      targetPosition,
      curvature: 0.5,
    });
  }

  return (
    <>
      <BaseEdge
        path={path}
        markerStart={markerStart}
        markerEnd={markerEnd}
        interactionWidth={interactionWidth ?? 14}
        className={
          data?.active
            ? "builder-wire builder-wire--active"
            : "builder-wire"
        }
        style={{
          stroke: data?.color ?? "var(--t-any)",
          strokeWidth: data?.implicit ? 1.4 : 1.6,
          opacity: data?.active ? 0.95 : data?.implicit ? 0.4 : 0.55,
          // Implicit plumbing reads as dashed until a run animates
          // it (the active dash pattern takes over then).
          ...(data?.implicit && !data?.active
            ? { strokeDasharray: "4 5" }
            : {}),
        }}
      />
      {typeof data?.label === "string" && data.label && (
        <EdgeLabelRenderer>
          <div
            className="builder-wire-label"
            style={{
              transform:
                `translate(-50%, -50%) ` +
                `translate(${labelX}px, ${labelY}px)`,
            }}
          >
            {data.label}
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  );
}
