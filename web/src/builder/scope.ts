import type {
  CanvasEdge,
  CanvasNode,
  CanvasPort,
  ParsedWorkflow,
} from "../types/workflow";

/**
 * Synthetic boundary node ids inside a drilled-open group —
 * hyphenated so they cannot collide with instance ids, which the
 * DSL keeps snake_case.
 */
export const GROUP_INPUTS_NODE_ID = "group-inputs";
export const GROUP_OUTPUTS_NODE_ID = "group-outputs";
/** Port pair carrying a loop's feedback wire between boundaries. */
export const RETURN_PORT_ID = "__return";

export interface ScopedWorkflow {
  view: ParsedWorkflow;
  /** The group the view is inside; null at the top level. */
  focus: CanvasNode | null;
  /** Groups enclosing the focus, outermost first. */
  trail: CanvasNode[];
}

function portRank(ports: CanvasPort[]) {
  return new Map(ports.map((port, index) => [port.id, index]));
}

function mirrorOrder(
  ports: CanvasPort[],
  rank: Map<string, number>,
) {
  return [...ports].sort(
    (a, b) =>
      (rank.get(a.id) ?? Number.MAX_SAFE_INTEGER) -
      (rank.get(b.id) ?? Number.MAX_SAFE_INTEGER),
  );
}

/**
 * The slice of a workflow one canvas shows: the top level, or —
 * drilled into a group — its body between two synthetic boundary
 * nodes carrying the group's ports, wired to same-named ports on
 * the body nodes. Loops also get a feedback wire from the exit
 * boundary back to the entry.
 */
export function scopeWorkflow(
  parsed: ParsedWorkflow,
  focusId: string | null,
): ScopedWorkflow {
  const focus = focusId
    ? (parsed.nodes.find(
        (node) => node.id === focusId && node.isGroup,
      ) ?? null)
    : null;

  if (!focus) {
    const nodes = parsed.nodes.filter((node) => !node.parentGroup);
    const ids = new Set(nodes.map((node) => node.id));
    return {
      view: {
        nodes,
        edges: parsed.edges.filter(
          (edge) => ids.has(edge.from) && ids.has(edge.to),
        ),
        order: nodes.map((node) => node.id),
      },
      focus: null,
      trail: [],
    };
  }

  // Body nodes with their ports mirroring the group's order, so the
  // boundary wires run parallel instead of crossing.
  const inputRank = portRank(focus.inputs);
  const outputRank = portRank(focus.outputs);
  const children = parsed.nodes
    .filter((node) => node.parentGroup === focus.id)
    .map((node) => ({
      ...node,
      inputs: mirrorOrder(node.inputs, inputRank),
      outputs: mirrorOrder(node.outputs, outputRank),
    }));
  const childIds = new Set(children.map((node) => node.id));
  const edges: CanvasEdge[] = parsed.edges.filter(
    (edge) => childIds.has(edge.from) && childIds.has(edge.to),
  );

  const kind =
    typeof focus.config.kind === "string" && focus.config.kind
      ? focus.config.kind
      : "group";
  const isLoop = kind === "loop";

  const entry: CanvasNode = {
    id: GROUP_INPUTS_NODE_ID,
    kind: "input",
    name: "Inputs",
    sub: `${kind} inputs`,
    x: 0,
    y: 0,
    config: {},
    inputs: isLoop
      ? [{ id: RETURN_PORT_ID, label: "↻ next iteration", type: "any" }]
      : [],
    outputs: focus.inputs.map((port) => ({ ...port })),
    body: focus.inputs.map((port) => ({
      k: port.label,
      v: port.type,
      mono: true,
    })),
    last: {},
  };
  const exit: CanvasNode = {
    id: GROUP_OUTPUTS_NODE_ID,
    kind: "output",
    name: "Outputs",
    sub: `${kind} outputs`,
    x: 0,
    y: 0,
    config: {},
    inputs: focus.outputs.map((port) => ({ ...port })),
    outputs: isLoop
      ? [{ id: RETURN_PORT_ID, label: "↻ repeat", type: "any" }]
      : [],
    body: focus.outputs.map((port) => ({
      k: port.label,
      v: port.type,
      mono: true,
    })),
    last: {},
  };

  // The group's ports feed same-named ports on its body nodes and
  // collect their results; explicitly wired inputs keep their edge.
  let syntheticIndex = 0;
  const wiredInputs = new Set(
    edges.map((edge) => `${edge.to}:${edge.toPort}`),
  );
  for (const child of children) {
    for (const port of focus.inputs) {
      if (
        child.inputs.some((candidate) => candidate.id === port.id) &&
        !wiredInputs.has(`${child.id}:${port.id}`)
      ) {
        edges.push({
          id: `scope-e${syntheticIndex++}`,
          from: GROUP_INPUTS_NODE_ID,
          fromPort: port.id,
          to: child.id,
          toPort: port.id,
        });
      }
    }
    for (const port of focus.outputs) {
      if (
        child.outputs.some((candidate) => candidate.id === port.id)
      ) {
        edges.push({
          id: `scope-e${syntheticIndex++}`,
          from: child.id,
          fromPort: port.id,
          to: GROUP_OUTPUTS_NODE_ID,
          toPort: port.id,
        });
      }
    }
  }
  if (isLoop) {
    edges.push({
      id: "scope-return",
      from: GROUP_OUTPUTS_NODE_ID,
      fromPort: RETURN_PORT_ID,
      to: GROUP_INPUTS_NODE_ID,
      toPort: RETURN_PORT_ID,
    });
  }

  const nodes = [entry, ...children, exit];

  const byId = new Map(parsed.nodes.map((node) => [node.id, node]));
  const trail: CanvasNode[] = [];
  let parentId = focus.parentGroup;
  const visited = new Set<string>();
  while (parentId && !visited.has(parentId)) {
    visited.add(parentId);
    const parent = byId.get(parentId);
    if (!parent) {
      break;
    }
    trail.unshift(parent);
    parentId = parent.parentGroup;
  }

  return {
    view: { nodes, edges, order: nodes.map((node) => node.id) },
    focus,
    trail,
  };
}
