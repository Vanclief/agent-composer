import type {
  CanvasEdge,
  CanvasNode,
  ParsedWorkflow,
} from "../types/workflow";

export const NODE_WIDTH = 260;
const COLUMN_GAP = 380;
const ROW_GAP = 200;

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

  for (const [depth, column] of columns) {
    const x = depth * COLUMN_GAP + 80;
    const startY = Math.max(0, ((4 - column.length) * ROW_GAP) / 2);
    column.forEach((node, index) => {
      node.x = x;
      node.y = startY + index * ROW_GAP + 60;
    });
  }
}

export function layoutWorkflow(parsed: ParsedWorkflow) {
  const nodes = parsed.nodes.map((node) => ({ ...node }));
  const topLevel = nodes.filter((node) => !node.parentGroup);
  const topLevelIds = new Set(topLevel.map((node) => node.id));
  const topLevelEdges = parsed.edges.filter(
    (edge) => topLevelIds.has(edge.from) && topLevelIds.has(edge.to),
  );
  autoLayout(topLevel, topLevelEdges);

  for (const group of nodes.filter((node) => node.isGroup)) {
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
    autoLayout(children, childEdges);

    const minimumX = Math.min(...children.map((node) => node.x));
    const minimumY = Math.min(...children.map((node) => node.y));
    for (const child of children) {
      child.x += group.x - minimumX;
      child.y += group.y - minimumY + 200;
    }
  }

  return {
    nodes,
    edges: parsed.edges.map((edge) => ({ ...edge })),
    order: [...parsed.order],
  };
}
