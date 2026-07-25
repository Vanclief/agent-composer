import { load } from "js-yaml";
import type {
  SnapshotNode,
  WorkflowExecution,
  WorkflowSnapshot,
} from "../types/api";
import type {
  BlueprintDocument,
  BlueprintNode,
  CanvasEdge,
  CanvasField,
  CanvasNode,
  CanvasPort,
  CanvasNodeKind,
  ParsedWorkflow,
  PortType,
} from "../types/workflow";

const emptyWorkflow = (): ParsedWorkflow => ({
  nodes: [],
  edges: [],
  order: [],
});

function readBlueprint(spec: string): BlueprintDocument | null {
  try {
    const value = load(spec);
    if (!value || typeof value !== "object") {
      return null;
    }
    return value as BlueprintDocument;
  } catch {
    return null;
  }
}

export function mapKind(kind?: string): CanvasNodeKind {
  if (kind === "inference") {
    return "llm";
  }
  return "transform";
}

export function mapPortType(typeRef?: string): PortType {
  if (!typeRef) {
    return "any";
  }

  const type = typeRef.toLowerCase();
  if (type === "string") {
    return "text";
  }
  if (type === "file") {
    return "file";
  }
  return "json";
}

function blueprintPorts(definitions?: Record<string, unknown>): CanvasPort[] {
  return Object.entries(definitions ?? {}).map(([name, definition]) => {
    const typeRef =
      typeof definition === "string" ? definition : "any";
    return {
      id: name,
      label: name,
      type: mapPortType(typeRef),
    };
  });
}

function preferPorts(
  declared: Record<string, unknown> | undefined,
  referenced: Record<string, unknown> | undefined,
) {
  if (declared && Object.keys(declared).length > 0) {
    return declared;
  }
  return referenced;
}

function blueprintBody(node: BlueprintNode): CanvasField[] {
  const fields: CanvasField[] = [];
  if (node.kind) {
    fields.push({ k: "kind", v: node.kind, mono: true });
  }
  if (node.config?.harness?.model) {
    fields.push({
      k: "model",
      v: node.config.harness.model,
      mono: true,
    });
  }
  if (node.config?.harness?.id) {
    fields.push({
      k: "harness",
      v: node.config.harness.id,
      mono: true,
    });
  }
  if (node.operation) {
    fields.push({ k: "operation", v: node.operation, mono: true });
  }
  if (node.workflow_id) {
    fields.push({ k: "workflow", v: node.workflow_id, mono: true });
  }
  return fields;
}

function referencedWorkflow(
  node: BlueprintNode,
  nodeSpecs: Record<string, BlueprintNode>,
) {
  if (node.kind === "loop" || node.kind === "conditional") {
    const target = node.executes ? nodeSpecs[node.executes] : undefined;
    if (target?.kind === "workflow" && target.workflow_id) {
      return target.workflow_id;
    }
    return null;
  }

  if (node.kind === "workflow" && node.workflow_id) {
    return node.workflow_id;
  }

  return null;
}

export function parseBlueprintYAML(
  yamlSpec: string,
  allSpecs: Record<string, string> = {},
): ParsedWorkflow {
  const blueprint = readBlueprint(yamlSpec);
  const rootInstances = blueprint?.flow?.instances;
  const rootNodeSpecs = blueprint?.nodes;
  if (!rootInstances || !rootNodeSpecs) {
    return emptyWorkflow();
  }

  const nodes: CanvasNode[] = [];
  const edges: CanvasEdge[] = [];
  let edgeIndex = 0;

  function addInputEdges(
    inputs: Record<string, unknown> | undefined,
    prefix: string,
    targetId: string,
  ) {
    for (const [inputName, binding] of Object.entries(inputs ?? {})) {
      if (typeof binding !== "string") {
        continue;
      }
      const parts = binding.split(".");
      if (parts[0] !== "instance" || parts.length < 3) {
        continue;
      }
      const instanceId = parts[1];
      if (!instanceId) {
        continue;
      }
      edges.push({
        id: `e${edgeIndex++}`,
        from: prefix ? `${prefix}.${instanceId}` : instanceId,
        fromPort: parts.slice(2).join("."),
        to: targetId,
        toPort: inputName,
      });
    }
  }

  function addInstanceNodes(
    instances: NonNullable<BlueprintDocument["flow"]>["instances"],
    nodeSpecs: Record<string, BlueprintNode>,
    prefix: string,
    parentGroupId: string | null,
  ) {
    for (const instanceId of Object.keys(instances ?? {})) {
      const instance = instances?.[instanceId];
      if (!instance?.node) {
        continue;
      }
      const nodeSpec = nodeSpecs[instance.node];
      if (!nodeSpec) {
        continue;
      }

      const fullId = prefix ? `${prefix}.${instanceId}` : instanceId;
      const workflowId = referencedWorkflow(nodeSpec, nodeSpecs);
      const subBlueprint = workflowId
        ? readBlueprint(allSpecs[workflowId] ?? "")
        : null;
      const subInstances = subBlueprint?.flow?.instances;
      const subNodeSpecs = subBlueprint?.nodes;

      if (workflowId && subInstances && subNodeSpecs) {
        const body: CanvasField[] = [];
        if (nodeSpec.kind) {
          body.push({ k: "kind", v: nodeSpec.kind, mono: true });
        }
        if (nodeSpec.operation) {
          body.push({
            k: "operation",
            v: nodeSpec.operation,
            mono: true,
          });
        }
        if (nodeSpec.max_iterations) {
          body.push({
            k: "max_iter",
            v: String(nodeSpec.max_iterations),
            mono: true,
          });
        }
        body.push({ k: "workflow", v: workflowId, mono: true });

        const description = subBlueprint.workflow?.description ?? "";
        const sub = [nodeSpec.kind, nodeSpec.operation, workflowId]
          .filter(Boolean)
          .join(" · ");

        nodes.push({
          id: fullId,
          kind: mapKind(nodeSpec.kind),
          name: instance.node,
          sub: sub || mapKind(nodeSpec.kind),
          x: 0,
          y: 0,
          config: {
            kind: nodeSpec.kind,
            operation: nodeSpec.operation ?? "",
            instruction: description,
          },
          inputs: blueprintPorts(
            preferPorts(
              nodeSpec.inputs,
              subBlueprint.workflow?.inputs,
            ),
          ),
          outputs: blueprintPorts(
            preferPorts(
              nodeSpec.outputs,
              subBlueprint.workflow?.outputs,
            ),
          ),
          body,
          last: {},
          parentGroup: parentGroupId,
          isGroup: true,
          groupLabel: workflowId,
        });

        addInstanceNodes(
          subInstances,
          subNodeSpecs,
          fullId,
          fullId,
        );
        addInputEdges(instance.inputs, prefix, fullId);
        continue;
      }

      const kind = mapKind(nodeSpec.kind);
      const sub = [nodeSpec.kind, nodeSpec.config?.harness?.model]
        .filter(Boolean)
        .join(" · ");

      nodes.push({
        id: fullId,
        kind,
        name: instance.node,
        sub: sub || kind,
        x: 0,
        y: 0,
        config: {
          model: nodeSpec.config?.harness?.model ?? "",
          instruction: nodeSpec.config?.instruction ?? "",
          harnessId: nodeSpec.config?.harness?.id ?? "",
          kind: nodeSpec.kind ?? "",
          operation: nodeSpec.operation ?? "",
        },
        inputs: blueprintPorts(nodeSpec.inputs),
        outputs: blueprintPorts(nodeSpec.outputs),
        body: blueprintBody(nodeSpec),
        last: {},
        parentGroup: parentGroupId,
      });

      addInputEdges(instance.inputs, prefix, fullId);
    }
  }

  addInstanceNodes(rootInstances, rootNodeSpecs, "", null);

  for (const node of nodes) {
    if (node.isGroup) {
      node.childCount = nodes.filter(
        (candidate) => candidate.parentGroup === node.id,
      ).length;
    }
  }

  return {
    nodes,
    edges,
    order: nodes.map((node) => node.id),
  };
}

function snapshotPorts(
  definitions: SnapshotNode["Inputs"],
  order?: string[],
) {
  const names = order ?? Object.keys(definitions ?? {});
  return names.map((name) => {
    const port = definitions?.[name];
    return {
      id: name,
      label: port?.Name ?? name,
      type: mapPortType(port?.TypeRef),
    };
  });
}

export function parseSnapshot(
  workflowExecution: WorkflowExecution,
): ParsedWorkflow {
  let snapshot: WorkflowSnapshot;
  try {
    snapshot =
      typeof workflowExecution.workflow_snapshot === "string"
        ? (JSON.parse(workflowExecution.workflow_snapshot) as WorkflowSnapshot)
        : workflowExecution.workflow_snapshot;
  } catch {
    throw new Error("The workflow snapshot is not valid JSON.");
  }

  if (!snapshot?.Nodes) {
    return emptyWorkflow();
  }

  const order = snapshot.Order ?? Object.keys(snapshot.Nodes);
  const nodes: CanvasNode[] = [];
  const edges: CanvasEdge[] = [];
  let edgeIndex = 0;

  for (const instanceId of order) {
    const nodeSpec = snapshot.Nodes[instanceId];
    if (!nodeSpec) {
      continue;
    }

    const kind = mapKind(nodeSpec.Kind);
    const body: CanvasField[] = [];
    if (nodeSpec.Kind) {
      body.push({ k: "kind", v: nodeSpec.Kind, mono: true });
    }
    if (nodeSpec.Model) {
      body.push({ k: "model", v: nodeSpec.Model, mono: true });
    }
    if (nodeSpec.Harness) {
      body.push({ k: "harness", v: nodeSpec.Harness, mono: true });
    }
    const sub = [nodeSpec.Kind, nodeSpec.Model].filter(Boolean).join(" · ");

    nodes.push({
      id: instanceId,
      kind,
      name: nodeSpec.NodeName ?? instanceId,
      sub: sub || kind,
      x: 0,
      y: 0,
      config: {
        model: nodeSpec.Model ?? "",
        instruction: nodeSpec.Instruction ?? "",
        harnessId: nodeSpec.Harness ?? "",
        kind: nodeSpec.Kind ?? "",
        operation: nodeSpec.Operation ?? "",
      },
      inputs: snapshotPorts(nodeSpec.Inputs, nodeSpec.InputOrder),
      outputs: snapshotPorts(nodeSpec.Outputs),
      body,
      last: {},
    });

    for (const [inputName, binding] of Object.entries(
      nodeSpec.InputBindings ?? {},
    )) {
      if (
        binding.Kind === "instance" &&
        binding.InstanceID &&
        binding.OutputName
      ) {
        edges.push({
          id: `e${edgeIndex++}`,
          from: binding.InstanceID,
          fromPort: binding.OutputName,
          to: instanceId,
          toPort: inputName,
        });
      }
    }
  }

  return { nodes, edges, order };
}
