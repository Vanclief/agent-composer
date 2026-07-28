package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/vanclief/agent-composer/models/agent"
	executionmodels "github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/agent-composer/runtime/types"
	runtimetypes "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

// maxParallelNodes bounds how many dependency-independent nodes run
// concurrently within a single workflow scope, to avoid spawning an unbounded
// number of harness subprocesses at once.
const maxParallelNodes = 8

func (e *Executor) Run(ctx context.Context, snapshot *Snapshot, input map[string]any) (map[string]any, error) {
	output, _, err := e.RunWithHandle(ctx, snapshot, input)
	return output, err
}

func (e *Executor) Start(ctx context.Context, snapshot *Snapshot, input map[string]any) (*WorkflowExecutionHandle, error) {
	const op = "workflow.Executor.Start"

	err := e.validateRunnableSnapshot(snapshot)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	if e.Recorder == nil {
		return nil, ez.New(op, ez.EINTERNAL, "execution recorder is nil", nil)
	}

	handle, err := e.Recorder.StartWorkflow(ctx, snapshot, input, e.ShellRoot)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	workflowHandle := handle
	runInput := cloneMap(input)

	go e.runDetached(ctx, snapshot, runInput, workflowHandle)

	return &workflowHandle, nil
}

func (e *Executor) RunWithHandle(ctx context.Context, snapshot *Snapshot, input map[string]any) (map[string]any, *WorkflowExecutionHandle, error) {
	const op = "workflow.Executor.Run"

	err := e.validateRunnableSnapshot(snapshot)
	if err != nil {
		return nil, nil, ez.Wrap(op, err)
	}

	var workflowHandle *WorkflowExecutionHandle
	if e.Recorder != nil {
		handle, err := e.Recorder.StartWorkflow(ctx, snapshot, input, e.ShellRoot)
		if err != nil {
			return nil, nil, ez.Wrap(op, err)
		}

		workflowHandle = &handle
	}

	output, err := e.runSnapshot(ctx, snapshot, input, workflowHandle, NodeExecutionScope{})

	finishErr := e.finishWorkflow(ctx, workflowHandle, output, err)
	if finishErr != nil {
		if err != nil {
			return nil, workflowHandle, ez.New(op, ez.EINTERNAL, "workflow execution failed and finish workflow also failed", finishErr)
		}

		return nil, workflowHandle, ez.Wrap(op, finishErr)
	}

	if err != nil {
		return nil, workflowHandle, ez.Wrap(op, err)
	}

	return output, workflowHandle, nil
}

func (e *Executor) validateRunnableSnapshot(snapshot *Snapshot) error {
	const op = "workflow.Executor.validateRunnableSnapshot"

	if snapshot == nil {
		return ez.New(op, ez.EINVALID, "workflow snapshot is nil", nil)
	}

	if e.NewHarness == nil {
		return ez.New(op, ez.EINTERNAL, "harness factory is nil", nil)
	}

	return nil
}

func (e *Executor) runDetached(ctx context.Context, snapshot *Snapshot, input map[string]any, workflowHandle WorkflowExecutionHandle) {
	output, err := e.runSnapshot(ctx, snapshot, input, &workflowHandle, NodeExecutionScope{})
	_ = e.finishWorkflow(ctx, &workflowHandle, output, err)
}

func (e *Executor) finishWorkflow(ctx context.Context, workflowHandle *WorkflowExecutionHandle, output map[string]any, runErr error) error {
	const op = "workflow.Executor.finishWorkflow"

	if workflowHandle == nil || e.Recorder == nil {
		return nil
	}

	status := executionmodels.WorkflowExecutionStatusSucceeded
	if runErr != nil {
		status = executionmodels.WorkflowExecutionStatusFailed
	}

	err := e.Recorder.FinishWorkflow(ctx, *workflowHandle, output, status)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

func (e *Executor) runSnapshot(ctx context.Context, snapshot *Snapshot, input map[string]any, workflowHandle *WorkflowExecutionHandle, scope NodeExecutionScope) (map[string]any, error) {
	const op = "workflow.Executor.runSnapshot"

	instanceOutputs := make(map[string]map[string]any, len(snapshot.Nodes))
	completed := make(map[string]bool, len(snapshot.Order))

	// Seeds apply to the top-level graph only — nested scopes (loops,
	// conditionals, composed targets) always run in full.
	if scope == (NodeExecutionScope{}) {
		for instanceID, outputs := range e.SeedOutputs {
			if _, exists := snapshot.Nodes[instanceID]; exists {
				instanceOutputs[instanceID] = cloneMap(outputs)
				completed[instanceID] = true
			}
		}
	}

	dependencies := make(map[string]map[string]bool, len(snapshot.Order))
	for _, instanceID := range snapshot.Order {
		dependencies[instanceID] = nodeDependencies(snapshot.Nodes[instanceID])
	}

	// Run the graph in dependency waves: every node whose producers have all
	// finished is launched concurrently, so independent nodes (such as parallel
	// reviewers) execute at the same time instead of one after another.
	for len(completed) < len(snapshot.Order) {
		ready := make([]string, 0, len(snapshot.Order))
		for _, instanceID := range snapshot.Order {
			if completed[instanceID] {
				continue
			}
			if dependenciesSatisfied(dependencies[instanceID], completed) {
				ready = append(ready, instanceID)
			}
		}

		if len(ready) == 0 {
			return nil, ez.New(op, ez.EINTERNAL, "workflow has unsatisfiable node dependencies", nil)
		}

		type nodeResult struct {
			instanceID string
			outputs    map[string]any
		}
		results := make([]nodeResult, len(ready))

		group, groupCtx := errgroup.WithContext(ctx)
		group.SetLimit(maxParallelNodes)
		for index, instanceID := range ready {
			index := index
			instanceID := instanceID
			group.Go(func() error {
				nodeOutputs, err := e.runNode(groupCtx, snapshot, instanceID, input, instanceOutputs, workflowHandle, scope)
				if err != nil {
					return err
				}

				results[index] = nodeResult{instanceID: instanceID, outputs: nodeOutputs}

				return nil
			})
		}

		err := group.Wait()
		if err != nil {
			return nil, err
		}

		for _, result := range results {
			instanceOutputs[result.instanceID] = result.outputs
			completed[result.instanceID] = true
		}
	}

	outputs := make(map[string]any, len(snapshot.Outputs))
	for outputName, binding := range snapshot.Outputs {
		nodeValues, found := instanceOutputs[binding.From.InstanceID]
		if !found {
			return nil, ez.New(op, ez.EINTERNAL, "missing output for instance: "+binding.From.InstanceID, nil)
		}

		value, found := nodeValues[binding.From.OutputName]
		if !found {
			return nil, ez.New(op, ez.EINTERNAL, "missing output value: "+binding.From.InstanceID+"."+binding.From.OutputName, nil)
		}

		outputs[outputName] = value
	}

	return outputs, nil
}

// nodeDependencies returns the set of producer instance ids a node consumes
// through its input bindings. A node is ready to run once all of these have
// finished.
func nodeDependencies(node NodeSnapshot) map[string]bool {
	deps := make(map[string]bool, len(node.InputBindings))
	for _, binding := range node.InputBindings {
		if binding.Kind == BindingKindInstance && binding.InstanceID != "" {
			deps[binding.InstanceID] = true
		}
	}

	return deps
}

func dependenciesSatisfied(deps map[string]bool, completed map[string]bool) bool {
	for dep := range deps {
		if !completed[dep] {
			return false
		}
	}

	return true
}

func (e *Executor) runNode(ctx context.Context, snapshot *Snapshot, instanceID string, input map[string]any, instanceOutputs map[string]map[string]any, workflowHandle *WorkflowExecutionHandle, scope NodeExecutionScope) (map[string]any, error) {
	const op = "workflow.Executor.runNode"

	node := snapshot.Nodes[instanceID]
	values, err := resolveNodeInputs(node, input, instanceOutputs)
	if err != nil {
		return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("node %q: %v", instanceID, err), err)
	}

	var nodeHandle *NodeExecutionHandle
	if workflowHandle != nil && e.Recorder != nil {
		handle, startErr := e.Recorder.StartNode(ctx, *workflowHandle, node, values, scope)
		if startErr != nil {
			return nil, ez.Wrap(op, startErr)
		}

		nodeHandle = &handle
	}

	var (
		trace map[string]any
		value any
	)

	switch node.Kind {
	case "inference":
		value, err = e.runInference(ctx, node, values, nodeHandle)
	case "connector":
		value, err = runConnector(node, values)
	case "loop":
		value, trace, err = e.runLoop(ctx, node, values, workflowHandle, nodeHandle)
	case "conditional":
		value, trace, err = e.runConditional(ctx, node, values, workflowHandle, nodeHandle)
	default:
		err = ez.New(op, ez.EINVALID, "unsupported node kind: "+node.Kind, nil)
	}
	if err != nil {
		if nodeHandle != nil && e.Recorder != nil {
			_ = e.Recorder.FinishNode(ctx, *nodeHandle, nil, executionmodels.NodeExecutionStatusFailed, makeErrorTrace(err))
		}

		return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("node %q failed", instanceID), err)
	}

	nodeOutputs, err := materializeNodeOutputs(node, value)
	if err != nil {
		if nodeHandle != nil && e.Recorder != nil {
			_ = e.Recorder.FinishNode(ctx, *nodeHandle, nil, executionmodels.NodeExecutionStatusFailed, makeErrorTrace(err))
		}

		return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("node %q returned invalid outputs", instanceID), err)
	}

	if nodeHandle != nil && e.Recorder != nil {
		err = e.Recorder.FinishNode(ctx, *nodeHandle, nodeOutputs, executionmodels.NodeExecutionStatusSucceeded, trace)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	}

	return nodeOutputs, nil
}

func materializeNodeOutputs(node NodeSnapshot, value any) (map[string]any, error) {
	const op = "workflow.materializeNodeOutputs"

	if len(node.Outputs) == 1 {
		return map[string]any{
			node.OutputName: value,
		}, nil
	}

	outputs, ok := value.(map[string]any)
	if !ok {
		return nil, ez.New(op, ez.EINVALID, "node returned invalid multi-output value", nil)
	}

	for outputName := range node.Outputs {
		if _, found := outputs[outputName]; !found {
			return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("node output %q is missing", outputName), nil)
		}
	}

	return outputs, nil
}

func (e *Executor) runInference(ctx context.Context, node NodeSnapshot, input map[string]any, nodeHandle *NodeExecutionHandle) (any, error) {
	const op = "workflow.Executor.runInference"

	value, err := e.runStructuredNode(
		ctx,
		node.InstanceID,
		node.Instruction,
		node.Harness,
		node.Model,
		node.ReasoningEffort,
		node.HarnessConfig,
		node.StructuredOutputSchema,
		node.StructuredOutputSchemaRaw,
		input,
		nodeHandle,
	)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	if node.WrapStructuredOutput {
		wrappedValue, ok := value.(map[string]any)
		if !ok {
			return nil, ez.New(op, ez.EINVALID, "harness returned invalid wrapped structured output", nil)
		}

		unwrapped, found := wrappedValue["value"]
		if !found {
			return nil, ez.New(op, ez.EINVALID, "harness returned wrapped structured output without value", nil)
		}

		value = unwrapped
	}

	return value, nil
}

func (e *Executor) runStructuredNode(ctx context.Context, agentName string, instruction string, harnessID agent.Harness, model string, effort runtimetypes.ReasoningEffort, harnessConfig json.RawMessage, outputSchema map[string]any, outputSchemaRaw json.RawMessage, input map[string]any, nodeHandle *NodeExecutionHandle) (any, error) {
	const op = "workflow.Executor.runStructuredNode"

	harness, err := e.NewHarness(harnessID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = harness.Validate(ctx, model, harnessConfig)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	prompt, err := buildPrompt(input, outputSchema)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	conversation := &agent.Conversation{
		AgentName:              agentName,
		Harness:                harnessID,
		Model:                  model,
		HarnessConfig:          copyRawJSON(harnessConfig),
		ReasoningEffort:        effort,
		Instructions:           instruction,
		Status:                 agent.ConversationStatusRunning,
		CreatedAt:              time.Now().UTC(),
		ShellRoot:              e.ShellRoot,
		CompactAtPercent:       90,
		StructuredOutput:       true,
		StructuredOutputSchema: copyRawJSON(outputSchemaRaw),
		Messages: []types.Message{
			*runtimetypes.NewSystemMessage(instruction),
			*runtimetypes.NewUserMessage(prompt),
		},
	}

	if e.Recorder != nil && nodeHandle != nil {
		err = e.Recorder.StartConversation(ctx, *nodeHandle, conversation, input)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	}

	result, runErr := harness.Run(ctx, conversation, prompt)
	if result != nil {
		if strings.TrimSpace(result.LastAssistantMessage) != "" {
			conversation.Messages = append(conversation.Messages, *runtimetypes.NewAssistantMessage(result.LastAssistantMessage))
		}
		if strings.TrimSpace(result.SessionRef) != "" {
			conversation.HarnessSessionRef = result.SessionRef
		}
		conversation.HarnessState = result.State
		conversation.RawHarnessOutput = result.RawOutput
		conversation.HarnessExitCode = result.ExitCode
		conversation.HarnessError = strings.TrimSpace(result.HarnessError)
		conversation.InputTokens += result.InputTokens
		conversation.OutputTokens += result.OutputTokens
		conversation.CachedTokens += result.CachedTokens
	}

	raw := ""
	if result != nil {
		raw = strings.TrimSpace(result.LastAssistantMessage)
	}

	if runErr != nil {
		conversation.Status = agent.ConversationStatusFailed
		if conversation.HarnessError == "" {
			conversation.HarnessError = runErr.Error()
		}

		if e.Recorder != nil && nodeHandle != nil {
			_ = e.Recorder.FinishConversation(ctx, conversation, nil)
		}

		if result != nil {
			return nil, ez.New(op, ez.EINTERNAL, "harness run failed: "+strings.TrimSpace(conversation.HarnessError), runErr)
		}

		return nil, ez.Wrap(op, runErr)
	}

	if raw == "" {
		conversation.Status = agent.ConversationStatusFailed
		conversation.HarnessError = "harness returned an empty final message"
		if e.Recorder != nil && nodeHandle != nil {
			_ = e.Recorder.FinishConversation(ctx, conversation, nil)
		}

		return nil, ez.New(op, ez.EINTERNAL, "harness returned an empty final message", nil)
	}

	raw, err = extractStructuredJSON(raw)
	if err != nil {
		conversation.Status = agent.ConversationStatusFailed
		conversation.HarnessError = err.Error()
		if e.Recorder != nil && nodeHandle != nil {
			_ = e.Recorder.FinishConversation(ctx, conversation, nil)
		}

		return nil, ez.Wrap(op, err)
	}

	var value any
	err = json.Unmarshal([]byte(raw), &value)
	if err != nil {
		conversation.Status = agent.ConversationStatusFailed
		conversation.HarnessError = err.Error()
		if e.Recorder != nil && nodeHandle != nil {
			_ = e.Recorder.FinishConversation(ctx, conversation, nil)
		}

		return nil, ez.New(op, ez.EINVALID, "harness returned invalid JSON for structured output: "+raw, err)
	}

	conversation.Status = agent.ConversationStatusSucceeded
	if e.Recorder != nil && nodeHandle != nil {
		err = e.Recorder.FinishConversation(ctx, conversation, value)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	}

	return value, nil
}

func buildPrompt(input map[string]any, outputSchema map[string]any) (string, error) {
	const op = "workflow.buildPrompt"

	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	schemaJSON, err := json.MarshalIndent(outputSchema, "", "  ")
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	var builder strings.Builder

	builder.WriteString("Use the following workflow inputs to perform the task.\n\n")
	builder.WriteString("Inputs:\n")
	builder.WriteString(string(payload))
	builder.WriteString("\n\n")
	builder.WriteString("Return exactly one JSON value that matches this JSON Schema:\n")
	builder.WriteString(string(schemaJSON))
	builder.WriteString("\n\n")
	builder.WriteString("Rules:\n")
	builder.WriteString("- Return only JSON.\n")
	builder.WriteString("- Do not wrap the JSON in markdown fences.\n")
	builder.WriteString("- Do not add commentary before or after the JSON.\n")

	return builder.String(), nil
}

func resolveNodeInputs(node NodeSnapshot, workflowInput map[string]any, instanceOutputs map[string]map[string]any) (map[string]any, error) {
	const op = "workflow.resolveNodeInputs"

	values := make(map[string]any, len(node.InputBindings))

	for inputName, binding := range node.InputBindings {
		switch binding.Kind {
		case BindingKindWorkflowInput:
			value, found := workflowInput[binding.WorkflowInput]
			if !found {
				return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("missing workflow input %q", binding.WorkflowInput), nil)
			}
			values[inputName] = value
		case BindingKindInstance:
			nodeValues, found := instanceOutputs[binding.InstanceID]
			if !found {
				return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("missing upstream instance output %q", binding.InstanceID), nil)
			}
			value, found := nodeValues[binding.OutputName]
			if !found {
				return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("missing upstream output %q.%q", binding.InstanceID, binding.OutputName), nil)
			}
			values[inputName] = value
		default:
			return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("unsupported binding kind %q", binding.Kind), nil)
		}
	}

	return values, nil
}

func runConnector(node NodeSnapshot, input map[string]any) (any, error) {
	const op = "workflow.runConnector"

	switch node.Operation {
	case "collect":
		values := make([]any, 0, len(node.InputBindings))
		for _, inputName := range orderedPortNames(node.InputOrder, node.InputBindings) {
			value, found := input[inputName]
			if !found {
				return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("missing connector input %q", inputName), nil)
			}
			values = append(values, value)
		}
		return values, nil
	case "concat":
		values := make([]any, 0, len(node.InputBindings))
		for _, inputName := range orderedPortNames(node.InputOrder, node.InputBindings) {
			value, found := input[inputName]
			if !found {
				return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("missing connector input %q", inputName), nil)
			}

			items, err := toAnySlice(value)
			if err != nil {
				return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("connector input %q is not an array", inputName), err)
			}

			values = append(values, items...)
		}
		return values, nil
	case "pack":
		if len(node.Outputs) != 1 {
			return nil, ez.New(op, ez.EINVALID, "pack connector requires exactly one output", nil)
		}

		values := make(map[string]any, len(node.InputBindings))
		for _, inputName := range orderedPortNames(node.InputOrder, node.InputBindings) {
			value, found := input[inputName]
			if !found {
				return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("missing connector input %q", inputName), nil)
			}

			values[inputName] = value
		}

		return values, nil
	case "unpack":
		if len(node.InputBindings) != 1 {
			return nil, ez.New(op, ez.EINVALID, "unpack connector requires exactly one input", nil)
		}

		sourceName := sortedKeys(node.InputBindings)[0]
		sourceValue, found := input[sourceName]
		if !found {
			return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("missing connector input %q", sourceName), nil)
		}

		sourceObject, ok := sourceValue.(map[string]any)
		if !ok {
			return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("connector input %q is not an object", sourceName), nil)
		}

		values := make(map[string]any, len(node.Outputs))
		for outputName := range node.Outputs {
			value, found := sourceObject[outputName]
			if !found {
				return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("connector unpack output %q is missing in source object", outputName), nil)
			}

			values[outputName] = value
		}

		return values, nil
	default:
		return nil, ez.New(op, ez.EINVALID, fmt.Sprintf("unsupported connector operation %q", node.Operation), nil)
	}
}

func (e *Executor) runLoop(ctx context.Context, node NodeSnapshot, input map[string]any, workflowHandle *WorkflowExecutionHandle, nodeHandle *NodeExecutionHandle) (any, map[string]any, error) {
	const op = "workflow.Executor.runLoop"

	switch node.Operation {
	case "foreach":
		return e.runForeachLoop(ctx, node, input, workflowHandle, nodeHandle)
	case "while":
		return e.runWhileLoop(ctx, node, input, workflowHandle, nodeHandle)
	default:
		return nil, nil, ez.New(op, ez.EINVALID, fmt.Sprintf("unsupported loop operation %q", node.Operation), nil)
	}
}

func (e *Executor) runConditional(ctx context.Context, node NodeSnapshot, input map[string]any, workflowHandle *WorkflowExecutionHandle, nodeHandle *NodeExecutionHandle) (any, map[string]any, error) {
	const op = "workflow.Executor.runConditional"

	if strings.TrimSpace(node.Operation) != "if" {
		return nil, nil, ez.New(op, ez.EINVALID, fmt.Sprintf("unsupported conditional operation %q", node.Operation), nil)
	}

	rawDecision, found := input[node.RoutesOn]
	if !found {
		return nil, nil, ez.New(op, ez.EINVALID, fmt.Sprintf("conditional is missing input %q", node.RoutesOn), nil)
	}

	decision, ok := rawDecision.(bool)
	if !ok {
		return nil, nil, ez.New(op, ez.EINVALID, fmt.Sprintf("conditional input %q must be a boolean", node.RoutesOn), nil)
	}

	target := node.FalseTarget
	selectedBranch := "when_false"
	if decision {
		target = node.TrueTarget
		selectedBranch = "when_true"
	}

	if target == nil {
		return nil, nil, ez.New(op, ez.EINVALID, "conditional target is missing", nil)
	}

	targetInput, err := selectTargetInput(*target, input)
	if err != nil {
		return nil, nil, ez.Wrap(op, err)
	}

	scope := NodeExecutionScope{
		BranchName: selectedBranch,
	}
	if nodeHandle != nil {
		scope.ParentNodeExecutionID = nodeHandle.ID
	}

	value, err := e.runTarget(ctx, *target, targetInput, workflowHandle, scope)
	if err != nil {
		return nil, nil, ez.Wrap(op, err)
	}

	trace := map[string]any{
		"operation":       "if",
		"selected_branch": selectedBranch,
	}

	return value, trace, nil
}

func (e *Executor) runForeachLoop(ctx context.Context, node NodeSnapshot, input map[string]any, workflowHandle *WorkflowExecutionHandle, nodeHandle *NodeExecutionHandle) (any, map[string]any, error) {
	const op = "workflow.Executor.runForeachLoop"

	if node.LoopTarget == nil {
		return nil, nil, ez.New(op, ez.EINVALID, "foreach loop is missing target", nil)
	}

	items, found := input[node.Over]
	if !found {
		return nil, nil, ez.New(op, ez.EINVALID, fmt.Sprintf("foreach loop is missing input %q", node.Over), nil)
	}

	iterationItems, err := toAnySlice(items)
	if err != nil {
		return nil, nil, ez.New(op, ez.EINVALID, fmt.Sprintf("foreach loop input %q is invalid", node.Over), err)
	}

	results := make([]any, 0, len(iterationItems))
	for index, item := range iterationItems {
		iterationValues := make(map[string]any, len(input))
		for key, value := range input {
			iterationValues[key] = value
		}

		iterationValues[node.Over] = item

		iterationInput, err := selectTargetInput(*node.LoopTarget, iterationValues)
		if err != nil {
			return nil, nil, ez.Wrap(op, err)
		}

		iterationIndex := index
		scope := NodeExecutionScope{
			IterationIndex: &iterationIndex,
		}
		if nodeHandle != nil {
			scope.ParentNodeExecutionID = nodeHandle.ID
		}

		value, err := e.runTarget(ctx, *node.LoopTarget, iterationInput, workflowHandle, scope)
		if err != nil {
			return nil, nil, ez.Wrap(op, err)
		}

		results = append(results, value)
	}

	trace := map[string]any{
		"operation":  "foreach",
		"iterations": len(iterationItems),
	}

	return results, trace, nil
}

func (e *Executor) runTarget(ctx context.Context, target NodeSnapshot, input map[string]any, workflowHandle *WorkflowExecutionHandle, scope NodeExecutionScope) (any, error) {
	const op = "workflow.Executor.runTarget"

	if target.Workflow != nil {
		outputs, err := e.runSnapshot(ctx, target.Workflow, input, workflowHandle, scope)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		if len(target.Outputs) == 1 {
			value, found := outputs[target.OutputName]
			if !found {
				return nil, ez.New(op, ez.EINTERNAL, fmt.Sprintf("workflow target output %q is missing", target.OutputName), nil)
			}

			return value, nil
		}

		return outputs, nil
	}

	var nodeHandle *NodeExecutionHandle
	if workflowHandle != nil && e.Recorder != nil {
		handle, err := e.Recorder.StartNode(ctx, *workflowHandle, target, input, scope)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		nodeHandle = &handle
	}

	value, err := e.runInference(ctx, target, input, nodeHandle)
	if err != nil {
		if nodeHandle != nil && e.Recorder != nil {
			_ = e.Recorder.FinishNode(ctx, *nodeHandle, nil, executionmodels.NodeExecutionStatusFailed, makeErrorTrace(err))
		}

		return nil, err
	}

	nodeOutputs, err := materializeNodeOutputs(target, value)
	if err != nil {
		if nodeHandle != nil && e.Recorder != nil {
			_ = e.Recorder.FinishNode(ctx, *nodeHandle, nil, executionmodels.NodeExecutionStatusFailed, makeErrorTrace(err))
		}

		return nil, err
	}

	if nodeHandle != nil && e.Recorder != nil {
		err = e.Recorder.FinishNode(ctx, *nodeHandle, nodeOutputs, executionmodels.NodeExecutionStatusSucceeded, nil)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	}

	return value, nil
}

func (e *Executor) runWhileLoop(ctx context.Context, node NodeSnapshot, input map[string]any, workflowHandle *WorkflowExecutionHandle, nodeHandle *NodeExecutionHandle) (any, map[string]any, error) {
	const op = "workflow.Executor.runWhileLoop"

	if node.WhileTarget == nil {
		return nil, nil, ez.New(op, ez.EINVALID, "while loop is missing target", nil)
	}

	if node.MaxIterations <= 0 {
		return nil, nil, ez.New(op, ez.EINVALID, "while loop max_iterations must be greater than zero", nil)
	}

	currentInput := make(map[string]any, len(input))
	for key, value := range input {
		currentInput[key] = value
	}

	var lastState any

	for iteration := 0; iteration < node.MaxIterations; iteration++ {
		iterationInput, err := selectWhileTargetInput(*node.WhileTarget, currentInput)
		if err != nil {
			return nil, nil, ez.Wrap(op, err)
		}

		iterationIndex := iteration
		scope := NodeExecutionScope{
			IterationIndex: &iterationIndex,
		}
		if nodeHandle != nil {
			scope.ParentNodeExecutionID = nodeHandle.ID
		}

		nextState, shouldStop, err := e.runWhileTarget(ctx, *node.WhileTarget, iterationInput, workflowHandle, scope)
		if err != nil {
			return nil, nil, ez.Wrap(op, err)
		}

		currentInput[node.Updates] = nextState
		lastState = nextState
		if shouldStop {
			trace := map[string]any{
				"operation":  "while",
				"iterations": iteration + 1,
			}

			return nextState, trace, nil
		}
	}

	trace := map[string]any{
		"operation":               "while",
		"iterations":              node.MaxIterations,
		"max_iterations_exceeded": true,
		"stopped_gracefully":      true,
	}

	return lastState, trace, nil
}

func (e *Executor) runWhileTarget(ctx context.Context, target WhileTargetSnapshot, input map[string]any, workflowHandle *WorkflowExecutionHandle, scope NodeExecutionScope) (any, bool, error) {
	const op = "workflow.Executor.runWhileTarget"

	if target.Workflow != nil {
		outputs, err := e.runSnapshot(ctx, target.Workflow, input, workflowHandle, scope)
		if err != nil {
			return nil, false, ez.Wrap(op, err)
		}

		nextState, found := outputs[target.UpdateOutputName]
		if !found {
			return nil, false, ez.New(op, ez.EINTERNAL, fmt.Sprintf("while target output %q is missing", target.UpdateOutputName), nil)
		}

		rawShouldStop, found := outputs[target.BreakOutputName]
		if !found {
			return nil, false, ez.New(op, ez.EINTERNAL, fmt.Sprintf("while target output %q is missing", target.BreakOutputName), nil)
		}

		shouldStop, ok := rawShouldStop.(bool)
		if !ok {
			return nil, false, ez.New(op, ez.EINVALID, fmt.Sprintf("while target output %q must be a boolean", target.BreakOutputName), nil)
		}

		return nextState, shouldStop, nil
	}

	var nodeHandle *NodeExecutionHandle
	if workflowHandle != nil && e.Recorder != nil {
		persistedTarget := whileTargetSnapshotNode(target)
		handle, err := e.Recorder.StartNode(ctx, *workflowHandle, persistedTarget, input, scope)
		if err != nil {
			return nil, false, ez.Wrap(op, err)
		}

		nodeHandle = &handle
	}

	value, err := e.runStructuredNode(
		ctx,
		target.InstanceID,
		target.Instruction,
		target.Harness,
		target.Model,
		target.ReasoningEffort,
		target.HarnessConfig,
		target.StructuredOutputSchema,
		target.StructuredOutputSchemaRaw,
		input,
		nodeHandle,
	)
	if err != nil {
		if nodeHandle != nil && e.Recorder != nil {
			_ = e.Recorder.FinishNode(ctx, *nodeHandle, nil, executionmodels.NodeExecutionStatusFailed, makeErrorTrace(err))
		}

		return nil, false, err
	}

	outputs, ok := value.(map[string]any)
	if !ok {
		err = ez.New(op, ez.EINVALID, "while target returned invalid structured output", nil)
		if nodeHandle != nil && e.Recorder != nil {
			_ = e.Recorder.FinishNode(ctx, *nodeHandle, nil, executionmodels.NodeExecutionStatusFailed, makeErrorTrace(err))
		}

		return nil, false, err
	}

	nextState, found := outputs[target.UpdateOutputName]
	if !found {
		err = ez.New(op, ez.EINTERNAL, fmt.Sprintf("while target output %q is missing", target.UpdateOutputName), nil)
		if nodeHandle != nil && e.Recorder != nil {
			_ = e.Recorder.FinishNode(ctx, *nodeHandle, nil, executionmodels.NodeExecutionStatusFailed, makeErrorTrace(err))
		}

		return nil, false, err
	}

	rawShouldStop, found := outputs[target.BreakOutputName]
	if !found {
		err = ez.New(op, ez.EINTERNAL, fmt.Sprintf("while target output %q is missing", target.BreakOutputName), nil)
		if nodeHandle != nil && e.Recorder != nil {
			_ = e.Recorder.FinishNode(ctx, *nodeHandle, nil, executionmodels.NodeExecutionStatusFailed, makeErrorTrace(err))
		}

		return nil, false, err
	}

	shouldStop, ok := rawShouldStop.(bool)
	if !ok {
		err = ez.New(op, ez.EINVALID, fmt.Sprintf("while target output %q must be a boolean", target.BreakOutputName), nil)
		if nodeHandle != nil && e.Recorder != nil {
			_ = e.Recorder.FinishNode(ctx, *nodeHandle, nil, executionmodels.NodeExecutionStatusFailed, makeErrorTrace(err))
		}

		return nil, false, err
	}

	if nodeHandle != nil && e.Recorder != nil {
		err = e.Recorder.FinishNode(ctx, *nodeHandle, outputs, executionmodels.NodeExecutionStatusSucceeded, nil)
		if err != nil {
			return nil, false, ez.Wrap(op, err)
		}
	}

	return nextState, shouldStop, nil
}
