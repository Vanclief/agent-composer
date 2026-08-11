package workflow

import (
	"fmt"
	"strings"

	"github.com/vanclief/ez"
)

// The canvas reserves these ids for its synthetic Inputs/Outputs
// nodes (they live in node-selection URLs), so no flow instance may
// claim them. Keep in sync with web/src/api/blueprints.ts.
var reservedInstanceIDs = map[string]bool{
	"workflow-inputs":  true,
	"workflow-outputs": true,
}

func compileBlueprint(blueprint *Blueprint, namespace string, resolver *workflowResolver, inputResolver func(string) (Binding, error), stack []string) (*compiledWorkflow, error) {
	workflowID := strings.TrimSpace(blueprint.Workflow.ID)
	stack, err := pushWorkflowStack(stack, workflowID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	compiled := &compiledWorkflow{
		Nodes:        make(map[string]NodeSnapshot),
		Dependencies: make(map[string][]string),
		Outputs:      make(map[string]Binding),
	}

	workflowAliases := make(map[string]map[string]Binding)

	instanceIDs := sortedKeys(blueprint.Flow.Instances)

	// The canvas adds synthetic Inputs/Outputs nodes under these ids —
	// a real instance shadowing them would corrupt the graph and its
	// URLs.
	for _, instanceID := range instanceIDs {
		if reservedInstanceIDs[instanceID] {
			return nil, ez.New(ez.EINVALID, "instance id "+instanceID+" is reserved for the workflow inputs/outputs nodes", nil)
		}
	}

	err = compileWorkflowInstances(blueprint, instanceIDs, namespace, resolver, inputResolver, compiled, workflowAliases, stack)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = compileConcreteInstances(blueprint, instanceIDs, namespace, resolver, inputResolver, compiled, workflowAliases, stack)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	for outputName, outputSpec := range blueprint.Workflow.Outputs {
		binding, err := parseBinding(outputSpec.From)
		if err != nil {
			return nil, ez.Wrap(fmt.Errorf("workflow output %q: %w", outputName, err))
		}

		binding, err = rewriteBinding(binding, namespace, workflowAliases, inputResolver)
		if err != nil {
			return nil, ez.Wrap(fmt.Errorf("workflow output %q: %w", outputName, err))
		}

		compiled.Outputs[outputName] = binding
	}

	return compiled, nil
}

func compileWorkflowInstances(blueprint *Blueprint, instanceIDs []string, namespace string, resolver *workflowResolver, inputResolver func(string) (Binding, error), compiled *compiledWorkflow, workflowAliases map[string]map[string]Binding, stack []string) error {
	for _, instanceID := range instanceIDs {
		instance := blueprint.Flow.Instances[instanceID]

		nodeSpec, err := lookupInstanceNodeSpec(blueprint, instance)
		if err != nil {
			return err
		}

		if nodeSpec.Kind != "workflow" {
			continue
		}

		workflowID := strings.TrimSpace(nodeSpec.WorkflowID)
		if workflowID == "" {
			return ez.New(ez.EINVALID, "workflow node is missing workflow_id", nil)
		}

		childBlueprint, err := resolver.loadByWorkflowID(workflowID)
		if err != nil {
			return err
		}

		childInputResolver := func(name string) (Binding, error) {
			rawBinding, found := instance.Inputs[name]
			if !found {
				return Binding{}, fmt.Errorf("workflow instance %q is missing input binding %q", instanceID, name)
			}

			binding, err := parseBinding(rawBinding)
			if err != nil {
				return Binding{}, err
			}

			return rewriteBinding(binding, namespace, workflowAliases, inputResolver)
		}

		childCompiled, err := compileBlueprint(childBlueprint, namespace+instanceID+"__", resolver, childInputResolver, stack)
		if err != nil {
			return err
		}

		for nodeID, node := range childCompiled.Nodes {
			compiled.Nodes[nodeID] = node
		}

		for nodeID, deps := range childCompiled.Dependencies {
			compiled.Dependencies[nodeID] = append(compiled.Dependencies[nodeID], deps...)
		}

		workflowAliases[instanceID] = childCompiled.Outputs
	}

	return nil
}

func compileConcreteInstances(blueprint *Blueprint, instanceIDs []string, namespace string, resolver *workflowResolver, inputResolver func(string) (Binding, error), compiled *compiledWorkflow, workflowAliases map[string]map[string]Binding, stack []string) error {
	for _, instanceID := range instanceIDs {
		instance := blueprint.Flow.Instances[instanceID]

		nodeSpec, err := lookupInstanceNodeSpec(blueprint, instance)
		if err != nil {
			return err
		}

		if nodeSpec.Kind == "workflow" {
			continue
		}

		node, dependencies, err := compileConcreteNode(blueprint, instanceID, instance, nodeSpec, namespace, resolver, inputResolver, workflowAliases, stack)
		if err != nil {
			return err
		}

		compiled.Dependencies[node.InstanceID] = append(compiled.Dependencies[node.InstanceID], dependencies...)
		compiled.Nodes[node.InstanceID] = node
	}

	return nil
}

func lookupInstanceNodeSpec(blueprint *Blueprint, instance InstanceSpec) (NodeSpec, error) {
	nodeSpec, found := blueprint.Nodes[instance.Node]
	if !found {
		return NodeSpec{}, ez.New(ez.EINVALID, "flow instance references unknown node: "+instance.Node, nil)
	}

	return nodeSpec, nil
}

func compileConcreteNode(blueprint *Blueprint, instanceID string, instance InstanceSpec, nodeSpec NodeSpec, namespace string, resolver *workflowResolver, inputResolver func(string) (Binding, error), workflowAliases map[string]map[string]Binding, stack []string) (NodeSnapshot, []string, error) {
	err := validateConcreteNodeSpec(nodeSpec)
	if err != nil {
		return NodeSnapshot{}, nil, err
	}

	shape, err := buildNodeShape(blueprint, instance.Node, nodeSpec)
	if err != nil {
		return NodeSnapshot{}, nil, err
	}

	inputBindings, dependencies, err := compileInputBindings(nodeSpec, instance, instanceID, namespace, workflowAliases, inputResolver)
	if err != nil {
		return NodeSnapshot{}, nil, err
	}

	node := buildConcreteNodeSnapshot(namespace, instanceID, instance.Node, nodeSpec, shape, inputBindings)

	compositeDependencies, err := attachCompositeTargets(blueprint, instance.Node, nodeSpec, resolver, &node, stack)
	if err != nil {
		return NodeSnapshot{}, nil, err
	}

	dependencies = append(dependencies, compositeDependencies...)

	return node, dependencies, nil
}

func validateConcreteNodeSpec(nodeSpec NodeSpec) error {
	switch nodeSpec.Kind {
	case "inference", "loop", "conditional":
		return nil
	case "connector":
		operation := strings.TrimSpace(nodeSpec.Operation)
		if operation == "collect" || operation == "concat" || operation == "pack" || operation == "unpack" {
			return nil
		}

		return ez.New(ez.EINVALID, "only connector operations collect, concat, pack, and unpack are supported in the current workflow runtime", nil)
	default:
		return ez.New(ez.EINVALID, "only inference nodes, connector nodes, loop nodes, conditional nodes, and workflow nodes are supported in the current workflow runtime", nil)
	}
}

func buildConcreteNodeSnapshot(namespace string, instanceID string, nodeName string, nodeSpec NodeSpec, shape compiledNodeShape, inputBindings map[string]Binding) NodeSnapshot {
	return NodeSnapshot{
		InstanceID:                namespace + instanceID,
		NodeName:                  nodeName,
		Kind:                      nodeSpec.Kind,
		Operation:                 strings.TrimSpace(nodeSpec.Operation),
		Executes:                  strings.TrimSpace(nodeSpec.Executes),
		Over:                      strings.TrimSpace(nodeSpec.Over),
		Updates:                   strings.TrimSpace(nodeSpec.Updates),
		BreaksOn:                  strings.TrimSpace(nodeSpec.BreaksOn),
		MaxIterations:             nodeSpec.MaxIterations,
		RoutesOn:                  strings.TrimSpace(nodeSpec.RoutesOn),
		WhenTrue:                  strings.TrimSpace(nodeSpec.WhenTrue),
		WhenFalse:                 strings.TrimSpace(nodeSpec.WhenFalse),
		Instruction:               shape.Instruction,
		Harness:                   shape.Harness,
		Model:                     shape.Model,
		ReasoningEffort:           shape.ReasoningEffort,
		HarnessConfig:             shape.HarnessConfig,
		Inputs:                    shape.Inputs,
		InputOrder:                shape.InputOrder,
		InputBindings:             inputBindings,
		Outputs:                   shape.Outputs,
		OutputName:                shape.OutputName,
		OutputSchema:              shape.OutputSchema,
		StructuredOutputSchema:    shape.StructuredOutputSchema,
		StructuredOutputSchemaRaw: shape.StructuredOutputSchemaRaw,
		WrapStructuredOutput:      shape.WrapStructuredOutput,
	}
}

func attachCompositeTargets(blueprint *Blueprint, nodeName string, nodeSpec NodeSpec, resolver *workflowResolver, node *NodeSnapshot, stack []string) ([]string, error) {
	switch nodeSpec.Kind {
	case "loop":
		return attachLoopTargets(blueprint, nodeName, nodeSpec, resolver, node, stack)
	case "conditional":
		return attachConditionalTargets(blueprint, nodeName, nodeSpec, resolver, node, stack)
	default:
		return nil, nil
	}
}

func attachLoopTargets(blueprint *Blueprint, nodeName string, nodeSpec NodeSpec, resolver *workflowResolver, node *NodeSnapshot, stack []string) ([]string, error) {
	switch strings.TrimSpace(nodeSpec.Operation) {
	case "foreach":
		targetNode, targetDependencies, err := buildForeachLoopTarget(blueprint, nodeSpec, nodeName, resolver, stack)
		if err != nil {
			return nil, err
		}

		node.LoopTarget = targetNode

		return targetDependencies, nil
	case "while":
		targetNode, targetDependencies, err := buildWhileLoopTarget(blueprint, nodeSpec, nodeName, resolver, stack)
		if err != nil {
			return nil, err
		}

		node.WhileTarget = targetNode

		return targetDependencies, nil
	default:
		return nil, ez.New(ez.EINVALID, "only loop operations foreach and while are supported in the current workflow runtime", nil)
	}
}

func attachConditionalTargets(blueprint *Blueprint, nodeName string, nodeSpec NodeSpec, resolver *workflowResolver, node *NodeSnapshot, stack []string) ([]string, error) {
	trueTarget, falseTarget, branchDependencies, err := buildConditionalTargets(blueprint, nodeSpec, nodeName, resolver, stack)
	if err != nil {
		return nil, err
	}

	node.TrueTarget = trueTarget
	node.FalseTarget = falseTarget

	return branchDependencies, nil
}

func rewriteBinding(binding Binding, namespace string, workflowAliases map[string]map[string]Binding, inputResolver func(string) (Binding, error)) (Binding, error) {
	switch binding.Kind {
	case BindingKindWorkflowInput:
		if inputResolver == nil {
			return binding, nil
		}

		resolved, err := inputResolver(binding.WorkflowInput)
		if err != nil {
			return Binding{}, err
		}

		return resolved, nil
	case BindingKindInstance:
		if aliasOutputs, found := workflowAliases[binding.InstanceID]; found {
			aliasBinding, found := aliasOutputs[binding.OutputName]
			if !found {
				return Binding{}, fmt.Errorf("workflow instance %q does not expose output %q", binding.InstanceID, binding.OutputName)
			}

			return aliasBinding, nil
		}

		rewritten := binding
		rewritten.InstanceID = namespace + binding.InstanceID
		return rewritten, nil
	default:
		return Binding{}, fmt.Errorf("unsupported binding kind %q", binding.Kind)
	}
}
