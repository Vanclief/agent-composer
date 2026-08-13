import type {
  CanvasEdge,
  CanvasNode,
  ParsedWorkflow,
} from "../types/workflow";

export const NODE_WIDTH = 240;
const COLUMN_GAP = 340;
const ROW_GAP = 48;

// Card sections, mirroring WorkflowNode's markup: header, body
// fields, then the connections zone (rows per side, side by side).
const HEAD_HEIGHT = 58;
const FIELD_HEIGHT = 26;
const PORT_HEIGHT = 32;
const GROUP_TOGGLE_HEIGHT = 30;

// A group renders as one container: the card on top (it carries the
// connectors), the children nested beneath it inside the dashed box.
export const GROUP_CHILD_INSET = 20;
export const GROUP_CHILD_GAP = 32;
const GROUP_BOTTOM_PADDING = 24;

/** Rough rendered height of a node card, so stacks never overlap. */
export function estimateNodeHeight(node: CanvasNode) {
  const body = node.body.length * FIELD_HEIGHT + 16;
  const toggle =
    node.isGroup && node.groupLabel ? GROUP_TOGGLE_HEIGHT : 0;
  const ports =
    Math.max(node.inputs.length, node.outputs.length) * PORT_HEIGHT +
    22;
  return HEAD_HEIGHT + body + toggle + ports;
}

export function autoLayout(
  nodes: CanvasNode[],
  edges: CanvasEdge[],
  heightOf: (node: CanvasNode) => number = estimateNodeHeight,
) {
  const dependencies = new Map(
    nodes.map((node) => [node.id, new Set<string>()]),
  );
  for (const edge of edges) {
    dependencies.get(edge.to)?.add(edge.from);
  }

  const depths = new Map<string, number>();
  const visiting = new Set<string>();

  function getDepth(id: string): number {
    const cached = depths.get(id);
    if (cached !== undefined) {
      return cached;
    }
    if (visiting.has(id)) {
      return 0;
    }

    visiting.add(id);
    const upstream = dependencies.get(id);
    const depth =
      upstream && upstream.size > 0
        ? 1 + Math.max(...[...upstream].map(getDepth))
        : 0;
    visiting.delete(id);
    depths.set(id, depth);
    return depth;
  }

  for (const node of nodes) {
    getDepth(node.id);
  }

  const columns = new Map<number, CanvasNode[]>();
  for (const node of nodes) {
    const depth = depths.get(node.id) ?? 0;
    const column = columns.get(depth) ?? [];
    column.push(node);
    columns.set(depth, column);
  }

  const columnHeights = new Map<number, number>();
  for (const [depth, column] of columns) {
    const total = column.reduce(
      (sum, node) => sum + heightOf(node) + ROW_GAP,
      -ROW_GAP,
    );
    columnHeights.set(depth, total);
  }
  const tallest = Math.max(0, ...columnHeights.values());

  for (const [depth, column] of columns) {
    const x = depth * COLUMN_GAP + 80;
    // Center each column against the tallest one.
    let y = 60 + (tallest - (columnHeights.get(depth) ?? 0)) / 2;
    for (const node of column) {
      node.x = x;
      node.y = y;
      y += heightOf(node) + ROW_GAP;
    }
  }
}

export function layoutWorkflow(parsed: ParsedWorkflow) {
  const nodes = parsed.nodes.map((node) => ({ ...node }));
  const groups = nodes.filter((node) => node.isGroup);

  // Pass 1 — each group's children in local coordinates, recording
  // the extent its open body needs. Reverse order so nested groups
  // are measured before the group that contains them.
  const bodyExtents = new Map<string, number>();
  const heightOf = (node: CanvasNode) => {
    const bodyExtent = node.defaultExpanded
      ? bodyExtents.get(node.id)
      : undefined;
    if (bodyExtent === undefined) {
      return estimateNodeHeight(node);
    }
    return (
      estimateNodeHeight(node) +
      GROUP_CHILD_GAP +
      bodyExtent +
      GROUP_BOTTOM_PADDING
    );
  };

  for (const group of [...groups].reverse()) {
    const children = nodes.filter(
      (node) => node.parentGroup === group.id,
    );
    if (children.length === 0) {
      continue;
    }
    const childIds = new Set(children.map((node) => node.id));
    const childEdges = parsed.edges.filter(
      (edge) => childIds.has(edge.from) && childIds.has(edge.to),
    );
    autoLayout(children, childEdges, heightOf);

    const minimumX = Math.min(...children.map((node) => node.x));
    const minimumY = Math.min(...children.map((node) => node.y));
    for (const child of children) {
      child.x -= minimumX;
      child.y -= minimumY;
    }
    bodyExtents.set(
      group.id,
      Math.max(...children.map((node) => node.y + heightOf(node))),
    );
  }

  // Pass 2 — the top level, with open groups sized as containers so
  // nothing lands on top of their bodies.
  const topLevel = nodes.filter((node) => !node.parentGroup);
  const topLevelIds = new Set(topLevel.map((node) => node.id));
  const topLevelEdges = parsed.edges.filter(
    (edge) => topLevelIds.has(edge.from) && topLevelIds.has(edge.to),
  );
  autoLayout(topLevel, topLevelEdges, heightOf);

  // Pass 3 — nest each group's children beneath its card. Parse
  // order puts containers before the groups they contain, so a
  // nested group is already placed when its own children move.
  for (const group of groups) {
    const children = nodes.filter(
      (node) => node.parentGroup === group.id,
    );
    for (const child of children) {
      child.x += group.x + GROUP_CHILD_INSET;
      child.y +=
        group.y + estimateNodeHeight(group) + GROUP_CHILD_GAP;
    }
  }

  return {
    nodes,
    edges: parsed.edges.map((edge) => ({ ...edge })),
    order: [...parsed.order],
  };
}
