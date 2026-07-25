import {
  BaseEdge,
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
  } else {
    [path] = getBezierPath({
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
        strokeWidth: 1.6,
        opacity: data?.active ? 0.95 : 0.55,
      }}
    />
  );
}
