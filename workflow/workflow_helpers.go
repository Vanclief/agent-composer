package workflow

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/vanclief/agent-composer/models/agent"
	runtimetypes "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

func buildStructuredOutputSchema(outputSchema map[string]any) (map[string]any, bool) {
	if isObjectSchema(outputSchema) {
		return outputSchema, false
	}

	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": outputSchema,
		},
		"required":             []string{"value"},
		"additionalProperties": false,
	}, true
}

func isObjectSchema(schema map[string]any) bool {
	if schema == nil {
		return false
	}

	rawType, found := schema["type"]
	if !found {
		return false
	}

	switch typed := rawType.(type) {
	case string:
		return typed == "object"
	case []any:
		for _, value := range typed {
			text, ok := value.(string)
			if ok && text == "object" {
				return true
			}
		}
	}

	return false
}

func schemaTypeIncludes(schema map[string]any, expected string) bool {
	if schema == nil {
		return false
	}

	rawType, found := schema["type"]
	if !found {
		return false
	}

	switch typed := rawType.(type) {
	case string:
		return typed == expected
	case []any:
		for _, value := range typed {
			text, ok := value.(string)
			if ok && text == expected {
				return true
			}
		}
	}

	return false
}

func validateStrictStructuredOutputSchema(schema map[string]any, path string) error {
	if schema == nil {
		return nil
	}

	if schemaTypeIncludes(schema, "array") {
		rawItems, found := schema["items"]
		if !found {
			return fmt.Errorf("array schema at %s is missing items", path)
		}

		items, ok := rawItems.(map[string]any)
		if !ok {
			return fmt.Errorf("array schema at %s has invalid items", path)
		}

		return validateStrictStructuredOutputSchema(items, path+".items")
	}

	if !isObjectSchema(schema) {
		return nil
	}

	rawProperties, found := schema["properties"]
	if !found {
		return fmt.Errorf("object schema at %s is missing properties", path)
	}

	properties, ok := rawProperties.(map[string]any)
	if !ok {
		return fmt.Errorf("object schema at %s has invalid properties", path)
	}

	rawRequired, found := schema["required"]
	if !found {
		return fmt.Errorf("object schema at %s must declare required for every property; optional object fields are not supported in structured outputs", path)
	}

	required := map[string]struct{}{}
	switch typed := rawRequired.(type) {
	case []string:
		for _, name := range typed {
			required[name] = struct{}{}
		}
	case []any:
		for _, value := range typed {
			name, ok := value.(string)
			if !ok {
				return fmt.Errorf("object schema at %s has a non-string required entry", path)
			}
			required[name] = struct{}{}
		}
	default:
		return fmt.Errorf("object schema at %s has invalid required", path)
	}

	propertyNames := make([]string, 0, len(properties))
	for propertyName := range properties {
		propertyNames = append(propertyNames, propertyName)
	}
	sort.Strings(propertyNames)

	for _, propertyName := range propertyNames {
		if _, found := required[propertyName]; !found {
			return fmt.Errorf("object schema at %s marks property %q as optional; optional object fields are not supported in structured outputs", path, propertyName)
		}

		childSchema, ok := properties[propertyName].(map[string]any)
		if !ok {
			continue
		}

		err := validateStrictStructuredOutputSchema(childSchema, path+"."+propertyName)
		if err != nil {
			return err
		}
	}

	return nil
}

func buildNodeShape(spec *Spec, nodeName string, nodeSpec NodeSpec) (compiledNodeShape, error) {
	inputOrder := orderedPortNames(spec.NodeInputOrder[nodeName], nodeSpec.Inputs)
	inputPorts, err := buildInputPorts(spec, nodeName, nodeSpec.Inputs, inputOrder)
	if err != nil {
		return compiledNodeShape{}, err
	}

	outputOrder := orderedPortNames(spec.NodeOutputOrder[nodeName], nodeSpec.Outputs)
	outputs, outputName, outputSchema, structuredOutputSchema, structuredOutputSchemaRaw, wrapStructuredOutput, err := buildNodeOutputs(spec, nodeName, nodeSpec.Outputs, outputOrder)
	if err != nil {
		return compiledNodeShape{}, err
	}

	shape := compiledNodeShape{
		Inputs:                    inputPorts,
		InputOrder:                inputOrder,
		Outputs:                   outputs,
		OutputName:                outputName,
		OutputSchema:              outputSchema,
		StructuredOutputSchema:    structuredOutputSchema,
		StructuredOutputSchemaRaw: structuredOutputSchemaRaw,
		WrapStructuredOutput:      wrapStructuredOutput,
		Instruction:               strings.TrimSpace(nodeSpec.Config.Instruction),
	}

	if nodeSpec.Kind != "inference" {
		return shape, nil
	}

	harnessID, model, effort, harnessConfig, err := parseHarnessConfig(nodeSpec.Config.Harness)
	if err != nil {
		return compiledNodeShape{}, fmt.Errorf("node %q harness config: %w", nodeName, err)
	}

	shape.Harness = harnessID
	shape.Model = model
	shape.ReasoningEffort = effort
	shape.HarnessConfig = harnessConfig

	err = validateStrictStructuredOutputSchema(structuredOutputSchema, "response")
	if err != nil {
		return compiledNodeShape{}, fmt.Errorf("node %q structured output schema: %w", nodeName, err)
	}

	return shape, nil
}

func buildInputPorts(spec *Spec, nodeName string, declaredInputs map[string]string, inputNames []string) (map[string]Port, error) {
	inputPorts := make(map[string]Port, len(declaredInputs))

	for _, inputName := range inputNames {
		typeRef := declaredInputs[inputName]
		schema, err := resolveTypeRef(spec, strings.TrimSpace(typeRef))
		if err != nil {
			return nil, fmt.Errorf("node %q input %q: %w", nodeName, inputName, err)
		}

		inputPorts[inputName] = Port{
			Name:    inputName,
			TypeRef: typeRef,
			Schema:  schema,
		}
	}

	return inputPorts, nil
}

func buildNodeOutputs(spec *Spec, nodeName string, declaredOutputs map[string]string, outputNames []string) (map[string]Port, string, map[string]any, map[string]any, json.RawMessage, bool, error) {
	if len(outputNames) == 0 {
		return nil, "", nil, nil, nil, false, fmt.Errorf("nodes must declare at least one output")
	}

	outputs := make(map[string]Port, len(outputNames))
	for _, outputName := range outputNames {
		outputTypeRef := strings.TrimSpace(declaredOutputs[outputName])
		outputSchema, err := resolveTypeRef(spec, outputTypeRef)
		if err != nil {
			return nil, "", nil, nil, nil, false, fmt.Errorf("node %q output %q: %w", nodeName, outputName, err)
		}

		outputs[outputName] = Port{
			Name:    outputName,
			TypeRef: outputTypeRef,
			Schema:  outputSchema,
		}
	}

	if len(outputNames) == 1 {
		outputName := outputNames[0]
		outputSchema := outputs[outputName].Schema
		structuredOutputSchema, wrapStructuredOutput := buildStructuredOutputSchema(outputSchema)

		outputSchemaRaw, err := json.Marshal(structuredOutputSchema)
		if err != nil {
			return nil, "", nil, nil, nil, false, err
		}

		return outputs, outputName, outputSchema, structuredOutputSchema, outputSchemaRaw, wrapStructuredOutput, nil
	}

	structuredOutputSchema := map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"required":             outputNames,
		"additionalProperties": false,
	}

	properties := structuredOutputSchema["properties"].(map[string]any)
	for _, outputName := range outputNames {
		properties[outputName] = outputs[outputName].Schema
	}

	outputSchemaRaw, err := json.Marshal(structuredOutputSchema)
	if err != nil {
		return nil, "", nil, nil, nil, false, err
	}

	return outputs, "", nil, structuredOutputSchema, outputSchemaRaw, false, nil
}

func selectTargetInput(target NodeSnapshot, available map[string]any) (map[string]any, error) {
	input := make(map[string]any, len(target.Inputs))

	for inputName := range target.Inputs {
		value, found := available[inputName]
		if !found {
			return nil, fmt.Errorf("missing target input %q", inputName)
		}

		input[inputName] = value
	}

	return input, nil
}

func selectWhileTargetInput(target WhileTargetSnapshot, available map[string]any) (map[string]any, error) {
	input := make(map[string]any, len(target.Inputs))

	for inputName := range target.Inputs {
		value, found := available[inputName]
		if !found {
			return nil, fmt.Errorf("missing target input %q", inputName)
		}

		input[inputName] = value
	}

	return input, nil
}

func compileInputBindings(nodeSpec NodeSpec, instance InstanceSpec, instanceID string, namespace string, workflowAliases map[string]map[string]Binding, inputResolver func(string) (Binding, error)) (map[string]Binding, []string, error) {
	inputBindings := make(map[string]Binding, len(nodeSpec.Inputs))
	dependencies := []string{}

	for inputName := range nodeSpec.Inputs {
		rawBinding, found := instance.Inputs[inputName]
		if !found {
			return nil, nil, ez.New(ez.EINVALID, "missing binding for node input: "+instanceID+"."+inputName, nil)
		}

		binding, err := parseBinding(rawBinding)
		if err != nil {
			return nil, nil, fmt.Errorf("node %q input %q: %w", instanceID, inputName, err)
		}

		binding, err = rewriteBinding(binding, namespace, workflowAliases, inputResolver)
		if err != nil {
			return nil, nil, fmt.Errorf("node %q input %q: %w", instanceID, inputName, err)
		}

		if binding.Kind == BindingKindInstance {
			dependencies = append(dependencies, binding.InstanceID)
		}

		inputBindings[inputName] = binding
	}

	return inputBindings, dependencies, nil
}

func buildForeachLoopTarget(spec *Spec, loopSpec NodeSpec, loopNodeName string, resolver *workflowResolver, stack []string) (*NodeSnapshot, []string, error) {
	if strings.TrimSpace(loopSpec.Operation) != "foreach" {
		return nil, nil, fmt.Errorf("only loop operation foreach is supported in the current workflow runtime")
	}

	targetName := strings.TrimSpace(loopSpec.Executes)
	if targetName == "" {
		return nil, nil, fmt.Errorf("loop node %q is missing executes", loopNodeName)
	}

	targetSpec, found := spec.Nodes[targetName]
	if !found {
		return nil, nil, fmt.Errorf("loop node %q executes unknown node %q", loopNodeName, targetName)
	}

	over := strings.TrimSpace(loopSpec.Over)
	if over == "" {
		return nil, nil, fmt.Errorf("loop node %q is missing over", loopNodeName)
	}

	loopShape, err := buildNodeShape(spec, loopNodeName, loopSpec)
	if err != nil {
		return nil, nil, err
	}

	if targetSpec.Kind == "workflow" {
		if resolver == nil {
			return nil, nil, fmt.Errorf("workflow resolver is required for workflow foreach loop targets")
		}

		workflowID := strings.TrimSpace(targetSpec.WorkflowSlug)
		if workflowID == "" {
			return nil, nil, fmt.Errorf("workflow loop target %q is missing workflow_slug", targetName)
		}

		childSpec, err := resolver.loadByWorkflowID(workflowID)
		if err != nil {
			return nil, nil, err
		}

		childSnapshot, err := compileSnapshot(childSpec, resolver, stack)
		if err != nil {
			return nil, nil, err
		}

		err = validateForeachWorkflowTarget(loopNodeName, over, loopShape, targetName, childSnapshot)
		if err != nil {
			return nil, nil, err
		}

		outputs, outputName, outputSchema, err := snapshotOutputsToNodeOutputs(childSnapshot)
		if err != nil {
			return nil, nil, err
		}

		return &NodeSnapshot{
			InstanceID:   loopNodeName + "__" + targetName,
			NodeName:     targetName,
			Kind:         targetSpec.Kind,
			Inputs:       childSnapshot.Inputs,
			Outputs:      outputs,
			Workflow:     childSnapshot,
			OutputName:   outputName,
			OutputSchema: outputSchema,
		}, nil, nil
	}

	if targetSpec.Kind != "inference" {
		return nil, nil, fmt.Errorf("only inference and workflow loop targets are supported in the current workflow runtime")
	}

	targetShape, err := buildNodeShape(spec, targetName, targetSpec)
	if err != nil {
		return nil, nil, err
	}

	err = validateForeachLoopShape(loopNodeName, over, loopShape, targetName, targetShape)
	if err != nil {
		return nil, nil, err
	}

	targetNode := &NodeSnapshot{
		InstanceID:                loopNodeName + "__" + targetName,
		NodeName:                  targetName,
		Kind:                      targetSpec.Kind,
		Operation:                 strings.TrimSpace(targetSpec.Operation),
		Instruction:               targetShape.Instruction,
		Harness:                   targetShape.Harness,
		Model:                     targetShape.Model,
		ReasoningEffort:           targetShape.ReasoningEffort,
		HarnessConfig:             targetShape.HarnessConfig,
		Inputs:                    targetShape.Inputs,
		Outputs:                   targetShape.Outputs,
		OutputName:                targetShape.OutputName,
		OutputSchema:              targetShape.OutputSchema,
		StructuredOutputSchema:    targetShape.StructuredOutputSchema,
		StructuredOutputSchemaRaw: targetShape.StructuredOutputSchemaRaw,
		WrapStructuredOutput:      targetShape.WrapStructuredOutput,
	}

	return targetNode, nil, nil
}

func buildWhileLoopTarget(spec *Spec, loopSpec NodeSpec, loopNodeName string, resolver *workflowResolver, stack []string) (*WhileTargetSnapshot, []string, error) {
	if strings.TrimSpace(loopSpec.Operation) != "while" {
		return nil, nil, fmt.Errorf("only loop operation while is supported in the current workflow runtime")
	}

	targetName := strings.TrimSpace(loopSpec.Executes)
	if targetName == "" {
		return nil, nil, fmt.Errorf("loop node %q is missing executes", loopNodeName)
	}

	targetSpec, found := spec.Nodes[targetName]
	if !found {
		return nil, nil, fmt.Errorf("loop node %q executes unknown node %q", loopNodeName, targetName)
	}

	if targetSpec.Kind != "inference" {
		if targetSpec.Kind != "workflow" {
			return nil, nil, fmt.Errorf("only inference and workflow while loop targets are supported in the current workflow runtime")
		}
	}

	updates := strings.TrimSpace(loopSpec.Updates)
	if updates == "" {
		return nil, nil, fmt.Errorf("loop node %q is missing updates", loopNodeName)
	}

	breaksOn := strings.TrimSpace(loopSpec.BreaksOn)
	if breaksOn == "" {
		return nil, nil, fmt.Errorf("loop node %q is missing breaks_on", loopNodeName)
	}

	if loopSpec.MaxIterations <= 0 {
		return nil, nil, fmt.Errorf("loop node %q max_iterations must be greater than zero", loopNodeName)
	}

	loopShape, err := buildNodeShape(spec, loopNodeName, loopSpec)
	if err != nil {
		return nil, nil, err
	}

	if targetSpec.Kind == "workflow" {
		if resolver == nil {
			return nil, nil, fmt.Errorf("workflow resolver is required for workflow while loop targets")
		}

		workflowID := strings.TrimSpace(targetSpec.WorkflowSlug)
		if workflowID == "" {
			return nil, nil, fmt.Errorf("workflow loop target %q is missing workflow_slug", targetName)
		}

		childSpec, err := resolver.loadByWorkflowID(workflowID)
		if err != nil {
			return nil, nil, err
		}

		childSnapshot, err := compileSnapshot(childSpec, resolver, stack)
		if err != nil {
			return nil, nil, err
		}

		updateOutput, found := childSnapshot.Outputs[updates]
		if !found {
			return nil, nil, fmt.Errorf("while workflow target %q is missing output %q", targetName, updates)
		}

		breakOutput, found := childSnapshot.Outputs[breaksOn]
		if !found {
			return nil, nil, fmt.Errorf("while workflow target %q is missing output %q", targetName, breaksOn)
		}

		err = validateWhileWorkflowTarget(loopNodeName, updates, loopShape, targetName, childSnapshot, updateOutput.Schema, breakOutput.Schema)
		if err != nil {
			return nil, nil, err
		}

		return &WhileTargetSnapshot{
			InstanceID:         loopNodeName + "__" + targetName,
			NodeName:           targetName,
			Inputs:             childSnapshot.Inputs,
			Workflow:           childSnapshot,
			UpdateOutputName:   updates,
			UpdateOutputSchema: updateOutput.Schema,
			BreakOutputName:    breaksOn,
		}, nil, nil
	}

	targetInputs, err := buildInputPorts(spec, targetName, targetSpec.Inputs, orderedPortNames(spec.NodeInputOrder[targetName], targetSpec.Inputs))
	if err != nil {
		return nil, nil, err
	}

	updateSchema, breakSchema, structuredOutputSchema, structuredOutputSchemaRaw, err := buildWhileTargetOutputs(spec, targetName, targetSpec.Outputs, updates, breaksOn)
	if err != nil {
		return nil, nil, err
	}

	err = validateWhileLoopShape(loopNodeName, updates, loopShape, targetName, targetInputs, updateSchema, breakSchema)
	if err != nil {
		return nil, nil, err
	}

	err = validateStrictStructuredOutputSchema(structuredOutputSchema, "response")
	if err != nil {
		return nil, nil, fmt.Errorf("while target %q structured output schema: %w", targetName, err)
	}

	harnessID, model, effort, harnessConfig, err := parseHarnessConfig(targetSpec.Config.Harness)
	if err != nil {
		return nil, nil, fmt.Errorf("node %q harness config: %w", targetName, err)
	}

	return &WhileTargetSnapshot{
		InstanceID:                loopNodeName + "__" + targetName,
		NodeName:                  targetName,
		Instruction:               strings.TrimSpace(targetSpec.Config.Instruction),
		Harness:                   harnessID,
		Model:                     model,
		ReasoningEffort:           effort,
		HarnessConfig:             harnessConfig,
		Inputs:                    targetInputs,
		UpdateOutputName:          updates,
		UpdateOutputSchema:        updateSchema,
		BreakOutputName:           breaksOn,
		StructuredOutputSchema:    structuredOutputSchema,
		StructuredOutputSchemaRaw: structuredOutputSchemaRaw,
	}, nil, nil
}

func buildWhileTargetOutputs(spec *Spec, nodeName string, declaredOutputs map[string]string, updates string, breaksOn string) (map[string]any, map[string]any, map[string]any, json.RawMessage, error) {
	if len(declaredOutputs) != 2 {
		return nil, nil, nil, nil, fmt.Errorf("while target %q must declare exactly two outputs", nodeName)
	}

	updateTypeRef, found := declaredOutputs[updates]
	if !found {
		return nil, nil, nil, nil, fmt.Errorf("while target %q is missing update output %q", nodeName, updates)
	}

	breakTypeRef, found := declaredOutputs[breaksOn]
	if !found {
		return nil, nil, nil, nil, fmt.Errorf("while target %q is missing break output %q", nodeName, breaksOn)
	}

	updateSchema, err := resolveTypeRef(spec, strings.TrimSpace(updateTypeRef))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("node %q output %q: %w", nodeName, updates, err)
	}

	breakSchema, err := resolveTypeRef(spec, strings.TrimSpace(breakTypeRef))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("node %q output %q: %w", nodeName, breaksOn, err)
	}

	structuredOutputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			updates:  updateSchema,
			breaksOn: breakSchema,
		},
		"required":             []string{updates, breaksOn},
		"additionalProperties": false,
	}

	structuredOutputSchemaRaw, err := json.Marshal(structuredOutputSchema)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return updateSchema, breakSchema, structuredOutputSchema, structuredOutputSchemaRaw, nil
}

func buildConditionalTargets(spec *Spec, conditionalSpec NodeSpec, conditionalNodeName string, resolver *workflowResolver, stack []string) (*NodeSnapshot, *NodeSnapshot, []string, error) {
	if strings.TrimSpace(conditionalSpec.Operation) != "if" {
		return nil, nil, nil, fmt.Errorf("only conditional operation if is supported in the current workflow runtime")
	}

	routesOn := strings.TrimSpace(conditionalSpec.RoutesOn)
	if routesOn == "" {
		return nil, nil, nil, fmt.Errorf("conditional node %q is missing routes_on", conditionalNodeName)
	}

	conditionalShape, err := buildNodeShape(spec, conditionalNodeName, conditionalSpec)
	if err != nil {
		return nil, nil, nil, err
	}

	routesOnPort, found := conditionalShape.Inputs[routesOn]
	if !found {
		return nil, nil, nil, fmt.Errorf("conditional node %q routes_on input %q is not declared", conditionalNodeName, routesOn)
	}

	if !schemasEqual(routesOnPort.Schema, map[string]any{"type": "boolean"}) {
		return nil, nil, nil, fmt.Errorf("conditional node %q routes_on input %q must be boolean", conditionalNodeName, routesOn)
	}

	trueTarget, err := buildConditionalTarget(spec, conditionalSpec.WhenTrue, conditionalNodeName, conditionalShape, resolver, stack)
	if err != nil {
		return nil, nil, nil, err
	}

	falseTarget, err := buildConditionalTarget(spec, conditionalSpec.WhenFalse, conditionalNodeName, conditionalShape, resolver, stack)
	if err != nil {
		return nil, nil, nil, err
	}

	err = validateConditionalTargets(conditionalNodeName, conditionalShape, trueTarget, falseTarget)
	if err != nil {
		return nil, nil, nil, err
	}

	return trueTarget, falseTarget, nil, nil
}

func buildConditionalTarget(spec *Spec, targetName string, conditionalNodeName string, conditionalShape compiledNodeShape, resolver *workflowResolver, stack []string) (*NodeSnapshot, error) {
	trimmedTargetName := strings.TrimSpace(targetName)
	if trimmedTargetName == "" {
		return nil, fmt.Errorf("conditional node %q is missing a branch target", conditionalNodeName)
	}

	targetSpec, found := spec.Nodes[trimmedTargetName]
	if !found {
		return nil, fmt.Errorf("conditional node %q references unknown branch target %q", conditionalNodeName, trimmedTargetName)
	}

	if targetSpec.Kind == "workflow" {
		if resolver == nil {
			return nil, fmt.Errorf("workflow resolver is required for workflow conditional targets")
		}

		workflowID := strings.TrimSpace(targetSpec.WorkflowSlug)
		if workflowID == "" {
			return nil, fmt.Errorf("workflow conditional target %q is missing workflow_slug", trimmedTargetName)
		}

		childSpec, err := resolver.loadByWorkflowID(workflowID)
		if err != nil {
			return nil, err
		}

		childSnapshot, err := compileSnapshot(childSpec, resolver, stack)
		if err != nil {
			return nil, err
		}

		for inputName, targetPort := range childSnapshot.Inputs {
			conditionalPort, found := conditionalShape.Inputs[inputName]
			if !found {
				return nil, fmt.Errorf("conditional branch target %q is missing matching conditional input %q", trimmedTargetName, inputName)
			}

			if !schemasEqual(conditionalPort.Schema, targetPort.Schema) {
				return nil, fmt.Errorf("conditional branch target %q input %q does not match the conditional input schema", trimmedTargetName, inputName)
			}
		}

		outputs, outputName, outputSchema, err := snapshotOutputsToNodeOutputs(childSnapshot)
		if err != nil {
			return nil, err
		}

		return &NodeSnapshot{
			InstanceID:   conditionalNodeName + "__" + trimmedTargetName,
			NodeName:     trimmedTargetName,
			Kind:         targetSpec.Kind,
			Inputs:       childSnapshot.Inputs,
			Outputs:      outputs,
			Workflow:     childSnapshot,
			OutputName:   outputName,
			OutputSchema: outputSchema,
		}, nil
	}

	if targetSpec.Kind != "inference" {
		return nil, fmt.Errorf("only inference and workflow conditional targets are supported in the current workflow runtime")
	}

	targetShape, err := buildNodeShape(spec, trimmedTargetName, targetSpec)
	if err != nil {
		return nil, err
	}

	for inputName, targetPort := range targetShape.Inputs {
		conditionalPort, found := conditionalShape.Inputs[inputName]
		if !found {
			return nil, fmt.Errorf("conditional branch target %q is missing matching conditional input %q", trimmedTargetName, inputName)
		}

		if !schemasEqual(conditionalPort.Schema, targetPort.Schema) {
			return nil, fmt.Errorf("conditional branch target %q input %q does not match the conditional input schema", trimmedTargetName, inputName)
		}
	}

	return &NodeSnapshot{
		InstanceID:                conditionalNodeName + "__" + trimmedTargetName,
		NodeName:                  trimmedTargetName,
		Kind:                      targetSpec.Kind,
		Operation:                 strings.TrimSpace(targetSpec.Operation),
		Instruction:               targetShape.Instruction,
		Harness:                   targetShape.Harness,
		Model:                     targetShape.Model,
		ReasoningEffort:           targetShape.ReasoningEffort,
		HarnessConfig:             targetShape.HarnessConfig,
		Inputs:                    targetShape.Inputs,
		Outputs:                   targetShape.Outputs,
		OutputName:                targetShape.OutputName,
		OutputSchema:              targetShape.OutputSchema,
		StructuredOutputSchema:    targetShape.StructuredOutputSchema,
		StructuredOutputSchemaRaw: targetShape.StructuredOutputSchemaRaw,
		WrapStructuredOutput:      targetShape.WrapStructuredOutput,
	}, nil
}

func validateConditionalTargets(conditionalNodeName string, conditionalShape compiledNodeShape, trueTarget *NodeSnapshot, falseTarget *NodeSnapshot) error {
	if trueTarget == nil || falseTarget == nil {
		return fmt.Errorf("conditional node %q is missing a branch target", conditionalNodeName)
	}

	if len(trueTarget.Outputs) != len(conditionalShape.Outputs) || len(falseTarget.Outputs) != len(conditionalShape.Outputs) {
		return fmt.Errorf("conditional node %q outputs must match both branch targets", conditionalNodeName)
	}

	for outputName, conditionalOutput := range conditionalShape.Outputs {
		trueOutput, found := trueTarget.Outputs[outputName]
		if !found {
			return fmt.Errorf("conditional true branch is missing output %q", outputName)
		}

		falseOutput, found := falseTarget.Outputs[outputName]
		if !found {
			return fmt.Errorf("conditional false branch is missing output %q", outputName)
		}

		if !schemasEqual(trueOutput.Schema, conditionalOutput.Schema) {
			return fmt.Errorf("conditional true branch output %q does not match conditional output", outputName)
		}

		if !schemasEqual(falseOutput.Schema, conditionalOutput.Schema) {
			return fmt.Errorf("conditional false branch output %q does not match conditional output", outputName)
		}

		if !schemasEqual(trueOutput.Schema, falseOutput.Schema) {
			return fmt.Errorf("conditional branch outputs %q do not match each other", outputName)
		}
	}

	return nil
}

func validateWhileLoopShape(loopNodeName string, updates string, loopShape compiledNodeShape, targetName string, targetInputs map[string]Port, updateSchema map[string]any, breakSchema map[string]any) error {
	updateInput, found := loopShape.Inputs[updates]
	if !found {
		return fmt.Errorf("loop node %q updates input %q is not declared", loopNodeName, updates)
	}

	targetUpdateInput, found := targetInputs[updates]
	if !found {
		return fmt.Errorf("while target %q is missing input %q", targetName, updates)
	}

	if !schemasEqual(updateInput.Schema, targetUpdateInput.Schema) {
		return fmt.Errorf("while loop input %q does not match while target %q input schema", updates, targetName)
	}

	for inputName, loopPort := range loopShape.Inputs {
		targetPort, found := targetInputs[inputName]
		if !found {
			return fmt.Errorf("while target %q is missing input %q", targetName, inputName)
		}

		if !schemasEqual(loopPort.Schema, targetPort.Schema) {
			return fmt.Errorf("while loop input %q does not match while target %q input schema", inputName, targetName)
		}
	}

	if len(loopShape.Inputs) != len(targetInputs) {
		return fmt.Errorf("while loop node %q inputs must match while target %q inputs", loopNodeName, targetName)
	}

	if !schemasEqual(loopShape.OutputSchema, updateSchema) {
		return fmt.Errorf("while loop output does not match while target %q update output schema", targetName)
	}

	if !schemasEqual(breakSchema, map[string]any{"type": "boolean"}) {
		return fmt.Errorf("while target %q break output must be boolean", targetName)
	}

	return nil
}

func validateWhileWorkflowTarget(loopNodeName string, updates string, loopShape compiledNodeShape, targetName string, childSnapshot *Snapshot, updateSchema map[string]any, breakSchema map[string]any) error {
	if childSnapshot == nil {
		return fmt.Errorf("while workflow target %q snapshot is nil", targetName)
	}

	for inputName, loopPort := range loopShape.Inputs {
		targetPort, found := childSnapshot.Inputs[inputName]
		if !found {
			return fmt.Errorf("while workflow target %q is missing input %q", targetName, inputName)
		}

		if !schemasEqual(loopPort.Schema, targetPort.Schema) {
			return fmt.Errorf("while loop input %q does not match while workflow target %q input schema", inputName, targetName)
		}
	}

	if len(loopShape.Inputs) != len(childSnapshot.Inputs) {
		return fmt.Errorf("while loop node %q inputs must match while workflow target %q inputs", loopNodeName, targetName)
	}

	if !schemasEqual(loopShape.OutputSchema, updateSchema) {
		return fmt.Errorf("while loop output does not match while workflow target %q update output schema", targetName)
	}

	if !schemasEqual(breakSchema, map[string]any{"type": "boolean"}) {
		return fmt.Errorf("while workflow target %q break output must be boolean", targetName)
	}

	if _, found := childSnapshot.Outputs[updates]; !found {
		return fmt.Errorf("while workflow target %q is missing output %q", targetName, updates)
	}

	return nil
}

func validateForeachLoopShape(loopNodeName string, over string, loopShape compiledNodeShape, targetName string, targetShape compiledNodeShape) error {
	loopOverPort, found := loopShape.Inputs[over]
	if !found {
		return fmt.Errorf("loop node %q over input %q is not declared", loopNodeName, over)
	}

	targetOverPort, found := targetShape.Inputs[over]
	if !found {
		return fmt.Errorf("loop target %q is missing iterated input %q", targetName, over)
	}

	itemsSchema, err := arrayItemsSchema(loopOverPort.Schema)
	if err != nil {
		return fmt.Errorf("loop node %q over input %q: %w", loopNodeName, over, err)
	}

	if !schemasEqual(itemsSchema, targetOverPort.Schema) {
		return fmt.Errorf("loop node %q over input %q item schema does not match loop target %q input schema", loopNodeName, over, targetName)
	}

	for inputName, loopPort := range loopShape.Inputs {
		targetPort, found := targetShape.Inputs[inputName]
		if !found {
			return fmt.Errorf("loop target %q is missing input %q", targetName, inputName)
		}

		if inputName == over {
			continue
		}

		if !schemasEqual(loopPort.Schema, targetPort.Schema) {
			return fmt.Errorf("loop input %q does not match loop target %q input %q", inputName, targetName, inputName)
		}
	}

	if len(loopShape.Inputs) != len(targetShape.Inputs) {
		return fmt.Errorf("loop node %q inputs must match loop target %q inputs", loopNodeName, targetName)
	}

	loopItemsSchema, err := arrayItemsSchema(loopShape.OutputSchema)
	if err != nil {
		return fmt.Errorf("loop node %q output: %w", loopNodeName, err)
	}

	if !schemasEqual(loopItemsSchema, targetShape.OutputSchema) {
		return fmt.Errorf("loop output schema does not match loop target %q output schema", targetName)
	}

	return nil
}

func validateForeachWorkflowTarget(loopNodeName string, over string, loopShape compiledNodeShape, targetName string, childSnapshot *Snapshot) error {
	if childSnapshot == nil {
		return fmt.Errorf("workflow loop target %q snapshot is nil", targetName)
	}

	loopOverPort, found := loopShape.Inputs[over]
	if !found {
		return fmt.Errorf("loop node %q over input %q is not declared", loopNodeName, over)
	}

	targetOverPort, found := childSnapshot.Inputs[over]
	if !found {
		return fmt.Errorf("loop target %q is missing iterated input %q", targetName, over)
	}

	itemsSchema, err := arrayItemsSchema(loopOverPort.Schema)
	if err != nil {
		return fmt.Errorf("loop node %q over input %q: %w", loopNodeName, over, err)
	}

	if !schemasEqual(itemsSchema, targetOverPort.Schema) {
		return fmt.Errorf("loop node %q over input %q item schema does not match loop target %q input schema", loopNodeName, over, targetName)
	}

	for inputName, loopPort := range loopShape.Inputs {
		targetPort, found := childSnapshot.Inputs[inputName]
		if !found {
			return fmt.Errorf("loop target %q is missing input %q", targetName, inputName)
		}

		if inputName == over {
			continue
		}

		if !schemasEqual(loopPort.Schema, targetPort.Schema) {
			return fmt.Errorf("loop input %q does not match loop target %q input %q", inputName, targetName, inputName)
		}
	}

	if len(loopShape.Inputs) != len(childSnapshot.Inputs) {
		return fmt.Errorf("loop node %q inputs must match loop target %q inputs", loopNodeName, targetName)
	}

	loopItemsSchema, err := arrayItemsSchema(loopShape.OutputSchema)
	if err != nil {
		return fmt.Errorf("loop node %q output: %w", loopNodeName, err)
	}

	targetOutput, found := childSnapshot.Outputs["out"]
	if !found {
		return fmt.Errorf("workflow loop target %q is missing output %q", targetName, "out")
	}

	if !schemasEqual(loopItemsSchema, targetOutput.Schema) {
		return fmt.Errorf("loop output schema does not match loop target %q output schema", targetName)
	}

	return nil
}

func snapshotOutputsToNodeOutputs(snapshot *Snapshot) (map[string]Port, string, map[string]any, error) {
	if snapshot == nil {
		return nil, "", nil, fmt.Errorf("workflow snapshot is nil")
	}

	outputNames := sortedKeys(snapshot.Outputs)
	if len(outputNames) == 0 {
		return nil, "", nil, fmt.Errorf("workflow snapshot has no outputs")
	}

	outputs := make(map[string]Port, len(outputNames))
	for _, outputName := range outputNames {
		output := snapshot.Outputs[outputName]
		outputs[outputName] = Port{
			Name:   outputName,
			Schema: output.Schema,
		}
	}

	if len(outputNames) == 1 {
		outputName := outputNames[0]
		return outputs, outputName, outputs[outputName].Schema, nil
	}

	return outputs, "", nil, nil
}

func arrayItemsSchema(schema map[string]any) (map[string]any, error) {
	rawType, found := schema["type"]
	if !found {
		return nil, fmt.Errorf("schema is missing type")
	}

	typeName, ok := rawType.(string)
	if !ok || typeName != "array" {
		return nil, fmt.Errorf("schema must be an array")
	}

	items, found := schema["items"]
	if !found {
		return nil, fmt.Errorf("array schema is missing items")
	}

	itemsSchema, ok := items.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("array items schema is invalid")
	}

	return itemsSchema, nil
}

func schemasEqual(left map[string]any, right map[string]any) bool {
	leftBytes, err := json.Marshal(left)
	if err != nil {
		return false
	}

	rightBytes, err := json.Marshal(right)
	if err != nil {
		return false
	}

	return string(leftBytes) == string(rightBytes)
}

func toAnySlice(value any) ([]any, error) {
	typed, ok := value.([]any)
	if ok {
		return typed, nil
	}

	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return nil, fmt.Errorf("value is nil")
	}

	if reflected.Kind() != reflect.Slice && reflected.Kind() != reflect.Array {
		return nil, fmt.Errorf("value is not a slice")
	}

	items := make([]any, 0, reflected.Len())
	for index := 0; index < reflected.Len(); index++ {
		items = append(items, reflected.Index(index).Interface())
	}

	return items, nil
}

func parseBinding(raw string) (Binding, error) {
	trimmed := strings.TrimSpace(raw)

	if strings.HasPrefix(trimmed, "workflow_input.") {
		name := strings.TrimPrefix(trimmed, "workflow_input.")
		if name == "" {
			return Binding{}, fmt.Errorf("invalid workflow input binding %q", raw)
		}

		return Binding{
			Kind:          BindingKindWorkflowInput,
			WorkflowInput: name,
		}, nil
	}

	if strings.HasPrefix(trimmed, "instance.") {
		parts := strings.Split(trimmed, ".")
		if len(parts) != 3 {
			return Binding{}, fmt.Errorf("invalid instance binding %q", raw)
		}

		if parts[1] == "" || parts[2] == "" {
			return Binding{}, fmt.Errorf("invalid instance binding %q", raw)
		}

		return Binding{
			Kind:       BindingKindInstance,
			InstanceID: parts[1],
			OutputName: parts[2],
		}, nil
	}

	return Binding{}, fmt.Errorf("unsupported binding %q", raw)
}

func resolveTypeRef(spec *Spec, typeRef string) (map[string]any, error) {
	switch typeRef {
	case "string", "boolean", "integer", "number":
		return map[string]any{"type": typeRef}, nil
	}

	schema, found := spec.Schemas[typeRef]
	if !found {
		return nil, fmt.Errorf("unknown schema %q", typeRef)
	}

	return buildSchema(spec, schema)
}

func buildSchema(spec *Spec, schemaSpec SchemaSpec) (map[string]any, error) {
	if strings.TrimSpace(schemaSpec.SchemaRef) != "" {
		return resolveTypeRef(spec, strings.TrimSpace(schemaSpec.SchemaRef))
	}

	schema := map[string]any{}

	switch schemaSpec.Type {
	case "string", "boolean", "integer", "number":
		schema["type"] = schemaSpec.Type
	case "array":
		if schemaSpec.Items == nil {
			return nil, fmt.Errorf("array schema is missing items")
		}
		items, err := buildSchema(spec, *schemaSpec.Items)
		if err != nil {
			return nil, err
		}
		schema["type"] = "array"
		schema["items"] = items
	case "object":
		properties := make(map[string]any, len(schemaSpec.Properties))
		required := make([]string, 0, len(schemaSpec.Properties))
		propertyNames := sortedKeys(schemaSpec.Properties)
		for _, propertyName := range propertyNames {
			propertySpec := schemaSpec.Properties[propertyName]
			propertySchema, err := buildSchema(spec, propertySpec)
			if err != nil {
				return nil, err
			}
			properties[propertyName] = propertySchema
			if !propertySpec.Optional {
				required = append(required, propertyName)
			}
		}
		schema["type"] = "object"
		schema["properties"] = properties
		schema["additionalProperties"] = false
		if len(required) > 0 {
			schema["required"] = required
		}
	default:
		return nil, fmt.Errorf("unsupported schema type %q", schemaSpec.Type)
	}

	if len(schemaSpec.Enum) > 0 {
		schema["enum"] = schemaSpec.Enum
	}

	if strings.TrimSpace(schemaSpec.Description) != "" {
		schema["description"] = strings.TrimSpace(schemaSpec.Description)
	}

	if schemaSpec.Nullable {
		rawType, found := schema["type"]
		if found {
			switch typed := rawType.(type) {
			case string:
				schema["type"] = []any{typed, "null"}
			case []any:
				schema["type"] = append(typed, "null")
			}
		}
	}

	return schema, nil
}

func parseHarnessConfig(raw map[string]any) (agent.Harness, string, runtimetypes.ReasoningEffort, json.RawMessage, error) {
	harnessIDValue, found := raw["id"]
	if !found {
		return "", "", "", nil, fmt.Errorf("config.harness.id is required")
	}

	harnessIDText, ok := harnessIDValue.(string)
	if !ok {
		return "", "", "", nil, fmt.Errorf("config.harness.id must be a string")
	}

	harnessID := agent.Harness(strings.TrimSpace(harnessIDText))
	err := harnessID.Validate()
	if err != nil {
		return "", "", "", nil, err
	}

	modelValue, found := raw["model"]
	if !found {
		return "", "", "", nil, fmt.Errorf("config.harness.model is required")
	}

	model, ok := modelValue.(string)
	if !ok || strings.TrimSpace(model) == "" {
		return "", "", "", nil, fmt.Errorf("config.harness.model must be a non-empty string")
	}

	effort := runtimetypes.ReasoningEffortMedium
	if value, found := raw["reasoning_effort"]; found {
		text, ok := value.(string)
		if !ok {
			return "", "", "", nil, fmt.Errorf("config.harness.reasoning_effort must be a string")
		}
		effort = runtimetypes.ReasoningEffort(strings.TrimSpace(text))
		err = effort.Validate()
		if err != nil {
			return "", "", "", nil, err
		}
	}

	extra := make(map[string]any, len(raw))
	for key, value := range raw {
		switch key {
		case "id", "model", "reasoning_effort":
			continue
		default:
			extra[key] = value
		}
	}

	if len(extra) == 0 {
		return harnessID, strings.TrimSpace(model), effort, nil, nil
	}

	payload, err := json.Marshal(extra)
	if err != nil {
		return "", "", "", nil, err
	}

	return harnessID, strings.TrimSpace(model), effort, payload, nil
}

func topoSort(nodes []string, dependencies map[string][]string) ([]string, error) {
	inDegree := make(map[string]int, len(nodes))
	reverse := make(map[string][]string, len(nodes))

	for _, node := range nodes {
		inDegree[node] = 0
	}

	for node, deps := range dependencies {
		seen := make(map[string]bool, len(deps))
		for _, dep := range deps {
			if dep == "" || seen[dep] {
				continue
			}
			seen[dep] = true
			inDegree[node]++
			reverse[dep] = append(reverse[dep], node)
		}
	}

	queue := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}
	sort.Strings(queue)

	order := make([]string, 0, len(nodes))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		nextNodes := reverse[current]
		sort.Strings(nextNodes)
		for _, next := range nextNodes {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
				sort.Strings(queue)
			}
		}
	}

	if len(order) != len(nodes) {
		return nil, ez.New(ez.EINVALID, "workflow graph contains a cycle", nil)
	}

	return order, nil
}

func orderedPortNames[T any](declaredOrder []string, items map[string]T) []string {
	if len(items) == 0 {
		return nil
	}

	if len(declaredOrder) == len(items) {
		names := make([]string, 0, len(declaredOrder))
		seen := make(map[string]bool, len(declaredOrder))
		for _, name := range declaredOrder {
			if _, found := items[name]; !found {
				return sortedKeys(items)
			}

			if seen[name] {
				return sortedKeys(items)
			}

			seen[name] = true
			names = append(names, name)
		}

		return names
	}

	return sortedKeys(items)
}

func sortedKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func copyRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}

	clone := make(json.RawMessage, len(raw))
	copy(clone, raw)
	return clone
}

func extractStructuredJSON(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ez.New(ez.EINVALID, "assistant output is empty", nil)
	}

	if json.Valid([]byte(trimmed)) {
		return trimmed, nil
	}

	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	if json.Valid([]byte(trimmed)) {
		return trimmed, nil
	}

	objectStart := strings.Index(trimmed, "{")
	objectEnd := strings.LastIndex(trimmed, "}")
	if objectStart >= 0 && objectEnd > objectStart {
		candidate := strings.TrimSpace(trimmed[objectStart : objectEnd+1])
		if json.Valid([]byte(candidate)) {
			return candidate, nil
		}
	}

	arrayStart := strings.Index(trimmed, "[")
	arrayEnd := strings.LastIndex(trimmed, "]")
	if arrayStart >= 0 && arrayEnd > arrayStart {
		candidate := strings.TrimSpace(trimmed[arrayStart : arrayEnd+1])
		if json.Valid([]byte(candidate)) {
			return candidate, nil
		}
	}

	return "", ez.New(ez.EINVALID, "assistant output does not contain valid JSON", nil)
}
