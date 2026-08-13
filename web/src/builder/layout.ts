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
const GROUP_ENTER_HEIGHT = 30;

/** Rough rendered height of a node card, so stacks never overlap. */
export function estimateNodeHeight(node: CanvasNode) {
  // "kind" renders as the header subtitle, not a body row.
  const bodyRows = node.body.filter(
    (field) => field.k !== "kind",
  ).length;
  const enterRow =
    node.isGroup && node.groupLabel ? GROUP_ENTER_HEIGHT : 0;
  const body =
    bodyRows > 0 || enterRow > 0 ? bodyRows * FIELD_HEIGHT + 16 : 0;
  const ports =
    Math.max(node.inputs.length, node.outputs.length) * PORT_HEIGHT +
    22;
  return HEAD_HEIGHT + body + enterRow + ports;
}

export function autoLayout(nodes: CanvasNode[], edges: CanvasEdge[]) {
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
      (sum, node) => sum + estimateNodeHeight(node) + ROW_GAP,
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
      y += estimateNodeHeight(node) + ROW_GAP;
    }
  }
}

export function layoutWorkflow(parsed: ParsedWorkflow) {
  // Every canvas view is a single level — groups render as cards
  // here and open as their own view — so one pass places it all.
  const nodes = parsed.nodes.map((node) => ({ ...node }));
  autoLayout(nodes, parsed.edges);
  return {
    nodes,
    edges: parsed.edges.map((edge) => ({ ...edge })),
    order: [...parsed.order],
  };
}
