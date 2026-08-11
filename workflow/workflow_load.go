package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/vanclief/ez"
)

const (
	workflowHomeEnvVar  = "AGENT_COMPOSER_HOME"
	defaultWorkflowHome = ".agent_composer"
)

// ParseSpec decodes spec YAML. sourcePath is recorded so
// compilation can resolve embedded workflows from sibling files — pass
// "" for specs that did not come from disk.
func ParseSpec(raw []byte, sourcePath string) (*Spec, error) {
	var spec Spec
	err := yaml.Unmarshal(raw, &spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	nodeInputOrder, nodeOutputOrder, err := extractNodePortOrder(raw)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	spec.SourcePath = sourcePath
	spec.NodeInputOrder = nodeInputOrder
	spec.NodeOutputOrder = nodeOutputOrder

	return &spec, nil
}

func LoadSpecFile(path string) (*Spec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	spec, err := ParseSpec(raw, path)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return spec, nil
}

// Compile resolves embedded workflows only from files next to the
// spec's source. Registry.Compile also resolves installed
// workflows — use it whenever a database is available.
func Compile(spec *Spec) (*Snapshot, error) {
	return compileWithRegistry(context.Background(), spec, nil)
}

// Compile compiles a spec, resolving embedded workflows from the
// registry first and from files next to the spec's source second.
func (r *Registry) Compile(ctx context.Context, spec *Spec) (*Snapshot, error) {
	return compileWithRegistry(ctx, spec, r)
}

func compileWithRegistry(ctx context.Context, spec *Spec, registry *Registry) (*Snapshot, error) {
	resolver, err := newWorkflowResolver(ctx, spec, registry)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return compileSnapshot(spec, resolver, nil)
}

func compileSnapshot(spec *Spec, resolver *workflowResolver, stack []string) (*Snapshot, error) {
	if spec == nil {
		return nil, ez.New(ez.EINVALID, "workflow spec is nil", nil)
	}

	workflowID := strings.TrimSpace(spec.Workflow.Slug)
	if workflowID == "" {
		return nil, ez.New(ez.EINVALID, "workflow.slug is required", nil)
	}

	if strings.TrimSpace(spec.Workflow.Version) == "" {
		return nil, ez.New(ez.EINVALID, "workflow.version is required", nil)
	}

	snapshot := &Snapshot{
		WorkflowSlug:    workflowID,
		WorkflowID:      strings.TrimSpace(spec.Workflow.ID),
		WorkflowVersion: spec.Workflow.Version,
		Description:     strings.TrimSpace(spec.Workflow.Description),
		Inputs:          make(map[string]Port, len(spec.Workflow.Inputs)),
		Outputs:         make(map[string]OutputBinding, len(spec.Workflow.Outputs)),
	}

	for name, typeRef := range spec.Workflow.Inputs {
		schema, err := resolveTypeRef(spec, strings.TrimSpace(typeRef))
		if err != nil {
			return nil, ez.Wrap(fmt.Errorf("workflow input %q: %w", name, err))
		}

		snapshot.Inputs[name] = Port{
			Name:    name,
			TypeRef: typeRef,
			Schema:  schema,
		}
	}

	compiled, err := compileSpec(spec, "", resolver, nil, stack)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	snapshot.Nodes = compiled.Nodes

	order, err := topoSort(sortedKeys(snapshot.Nodes), compiled.Dependencies)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	snapshot.Order = order

	for outputName, outputSpec := range spec.Workflow.Outputs {
		schema, err := resolveTypeRef(spec, strings.TrimSpace(outputSpec.Schema))
		if err != nil {
			return nil, ez.Wrap(fmt.Errorf("workflow output %q: %w", outputName, err))
		}

		binding, found := compiled.Outputs[outputName]
		if !found {
			return nil, ez.New(ez.EINTERNAL, "compiled workflow output binding is missing: "+outputName, nil)
		}

		if binding.Kind != BindingKindInstance {
			return nil, ez.New(ez.EINVALID, "workflow outputs must bind from an instance output", nil)
		}

		node, found := snapshot.Nodes[binding.InstanceID]
		if !found {
			return nil, ez.New(ez.EINVALID, "workflow output references unknown instance: "+binding.InstanceID, nil)
		}

		_, found = node.Outputs[binding.OutputName]
		if !found {
			return nil, ez.New(ez.EINVALID, "workflow output references unknown node output: "+binding.InstanceID+"."+binding.OutputName, nil)
		}

		snapshot.Outputs[outputName] = OutputBinding{
			Name:   outputName,
			Schema: schema,
			From:   binding,
		}
	}

	return snapshot, nil
}

func pushWorkflowStack(stack []string, workflowID string) ([]string, error) {
	for _, activeWorkflowID := range stack {
		if activeWorkflowID == workflowID {
			cycle := append(append([]string{}, stack...), workflowID)
			return nil, fmt.Errorf("workflow composition cycle detected: %s", strings.Join(cycle, " -> "))
		}
	}

	next := append([]string{}, stack...)
	next = append(next, workflowID)

	return next, nil
}

func extractNodePortOrder(raw []byte) (map[string][]string, map[string][]string, error) {
	var root yaml.Node
	err := yaml.Unmarshal(raw, &root)
	if err != nil {
		return nil, nil, err
	}

	document := &root
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		document = root.Content[0]
	}

	nodesNode := findMappingValue(document, "nodes")
	if nodesNode == nil || nodesNode.Kind != yaml.MappingNode {
		return map[string][]string{}, map[string][]string{}, nil
	}

	inputOrder := map[string][]string{}
	outputOrder := map[string][]string{}

	for index := 0; index+1 < len(nodesNode.Content); index += 2 {
		nodeName := nodesNode.Content[index].Value
		nodeValue := nodesNode.Content[index+1]
		if nodeValue.Kind != yaml.MappingNode {
			continue
		}

		inputsNode := findMappingValue(nodeValue, "inputs")
		if inputsNode != nil && inputsNode.Kind == yaml.MappingNode {
			inputOrder[nodeName] = mappingKeyOrder(inputsNode)
		}

		outputsNode := findMappingValue(nodeValue, "outputs")
		if outputsNode != nil && outputsNode.Kind == yaml.MappingNode {
			outputOrder[nodeName] = mappingKeyOrder(outputsNode)
		}
	}

	return inputOrder, outputOrder, nil
}

func findMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1]
		}
	}

	return nil
}

func mappingKeyOrder(node *yaml.Node) []string {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	order := make([]string, 0, len(node.Content)/2)
	for index := 0; index+1 < len(node.Content); index += 2 {
		order = append(order, node.Content[index].Value)
	}

	return order
}

func newWorkflowResolver(ctx context.Context, spec *Spec, registry *Registry) (*workflowResolver, error) {
	if spec == nil {
		return nil, ez.New(ez.EINVALID, "workflow spec is nil", nil)
	}

	searchDirs := []string{}
	if strings.TrimSpace(spec.SourcePath) != "" {
		sourceDir := filepath.Dir(spec.SourcePath)
		if sourceDir != "" {
			searchDirs = append(searchDirs, sourceDir)
		}
	}

	return &workflowResolver{
		byID:       map[string]*Spec{},
		searchDirs: searchDirs,
		registry:   registry,
		ctx:        ctx,
	}, nil
}

func (r *workflowResolver) loadByWorkflowID(workflowID string) (*Spec, error) {
	spec, found := r.byID[workflowID]
	if found {
		return spec, nil
	}

	if r.registry != nil {
		record, err := r.registry.getInstalledBySlug(r.ctx, workflowID)
		if err == nil {
			spec, err := ParseSpec([]byte(record.Spec), "")
			if err != nil {
				return nil, ez.Wrap(err)
			}

			r.byID[workflowID] = spec
			return spec, nil
		}
		if ez.ErrorCode(err) != ez.ENOTFOUND {
			return nil, ez.Wrap(err)
		}
	}

	for _, dir := range r.searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, ez.Wrap(err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".yaml" && ext != ".yml" {
				continue
			}

			path := filepath.Join(dir, entry.Name())
			spec, err := LoadSpecFile(path)
			if err != nil {
				return nil, ez.Wrap(err)
			}

			if spec.Workflow.Slug == workflowID {
				r.byID[workflowID] = spec
				return spec, nil
			}
		}
	}

	return nil, ez.New(ez.ENOTFOUND, fmt.Sprintf("workflow_slug %q not found", workflowID), nil)
}

func workflowSummaryFromSpec(spec *Spec) (WorkflowSummary, error) {
	if spec == nil {
		return WorkflowSummary{}, ez.New(ez.EINVALID, "workflow spec is nil", nil)
	}

	workflowID := strings.TrimSpace(spec.Workflow.Slug)
	if workflowID == "" {
		return WorkflowSummary{}, ez.New(ez.EINVALID, "workflow.slug is required", nil)
	}

	inputs := make(map[string]string, len(spec.Workflow.Inputs))
	for inputName, typeRef := range spec.Workflow.Inputs {
		inputs[inputName] = strings.TrimSpace(typeRef)
	}

	outputs := make(map[string]string, len(spec.Workflow.Outputs))
	for outputName, outputSpec := range spec.Workflow.Outputs {
		outputs[outputName] = strings.TrimSpace(outputSpec.Schema)
	}

	name := strings.TrimSpace(spec.Workflow.Name)
	if name == "" {
		name = workflowID
	}

	return WorkflowSummary{
		Slug:        workflowID,
		ID:          strings.TrimSpace(spec.Workflow.ID),
		Name:        name,
		Version:     strings.TrimSpace(spec.Workflow.Version),
		Description: strings.TrimSpace(spec.Workflow.Description),
		Inputs:      inputs,
		Outputs:     outputs,
	}, nil
}

func writeFileAtomically(path string, raw []byte) error {
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return ez.Wrap(err)
	}

	tempPath := tempFile.Name()
	cleanup := true

	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	_, err = tempFile.Write(raw)
	if err != nil {
		_ = tempFile.Close()
		return ez.Wrap(err)
	}

	err = tempFile.Chmod(0644)
	if err != nil {
		_ = tempFile.Close()
		return ez.Wrap(err)
	}

	err = tempFile.Close()
	if err != nil {
		return ez.Wrap(err)
	}

	err = os.Rename(tempPath, path)
	if err != nil {
		return ez.Wrap(err)
	}

	cleanup = false

	return nil
}
