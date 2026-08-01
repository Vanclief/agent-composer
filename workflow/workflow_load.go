package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/vanclief/ez"
)

const (
	workflowHomeEnvVar    = "AGENT_COMPOSER_HOME"
	defaultWorkflowHome   = ".agent_composer"
	defaultWorkflowSubdir = "workflows"
)

func LoadBlueprintFile(path string) (*Blueprint, error) {
	const op = "workflow.LoadBlueprintFile"

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	var blueprint Blueprint
	err = yaml.Unmarshal(raw, &blueprint)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	nodeInputOrder, nodeOutputOrder, err := extractNodePortOrder(raw)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	blueprint.SourcePath = path
	blueprint.NodeInputOrder = nodeInputOrder
	blueprint.NodeOutputOrder = nodeOutputOrder

	return &blueprint, nil
}

func LoadBlueprintByWorkflowID(workflowID string) (*Blueprint, error) {
	const op = "workflow.LoadBlueprintByWorkflowID"

	entry, err := loadRegistryBlueprintEntryByWorkflowID(workflowID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return entry.Blueprint, nil
}

func ListBlueprints() ([]WorkflowSummary, error) {
	const op = "workflow.ListBlueprints"

	workflowDir, err := ResolveWorkflowDir()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []WorkflowSummary{}, nil
		}

		return nil, ez.Wrap(op, err)
	}

	summaries := make([]WorkflowSummary, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(workflowDir, entry.Name())
		blueprint, err := LoadBlueprintFile(path)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		summary, err := workflowSummaryFromBlueprint(blueprint)
		if err != nil {
			return nil, ez.Wrap(op, fmt.Errorf("workflow %q: %w", path, err))
		}

		summaries = append(summaries, summary)
	}

	sort.Slice(summaries, func(i int, j int) bool {
		return summaries[i].ID < summaries[j].ID
	})

	return summaries, nil
}

func ImportBlueprintFile(sourcePath string, overwrite bool) (WorkflowSummary, error) {
	const op = "workflow.ImportBlueprintFile"

	trimmedSourcePath := strings.TrimSpace(sourcePath)
	if trimmedSourcePath == "" {
		return WorkflowSummary{}, ez.New(op, ez.EINVALID, "source path is required", nil)
	}

	raw, err := os.ReadFile(trimmedSourcePath)
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(op, err)
	}

	blueprint, err := LoadBlueprintFile(trimmedSourcePath)
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(op, err)
	}

	_, err = Compile(blueprint)
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(op, err)
	}

	summary, err := workflowSummaryFromBlueprint(blueprint)
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(op, err)
	}

	workflowDir, err := ResolveWorkflowDir()
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(op, err)
	}

	err = os.MkdirAll(workflowDir, 0755)
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(op, err)
	}

	targetPath := filepath.Join(workflowDir, summary.ID+".yaml")

	err = validateRegistryTargetPath(targetPath, summary.ID)
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(op, err)
	}

	entries, err := listRegistryBlueprintEntriesByWorkflowID(summary.ID)
	if err != nil && ez.ErrorCode(err) != ez.ENOTFOUND {
		return WorkflowSummary{}, ez.Wrap(op, err)
	}

	if ez.ErrorCode(err) == ez.ENOTFOUND {
		entries = nil
	}

	if len(entries) > 0 {
		if !overwrite {
			return WorkflowSummary{}, ez.New(op, ez.EINVALID, "workflow_id already exists in registry: "+summary.ID, nil)
		}

		canonicalExists := false
		for _, entry := range entries {
			if entry.Path == targetPath {
				canonicalExists = true
				break
			}
		}

		if !canonicalExists {
			err = os.Rename(entries[0].Path, targetPath)
			if err != nil {
				return WorkflowSummary{}, ez.Wrap(op, err)
			}
		}
	}

	err = writeFileAtomically(targetPath, raw)
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(op, err)
	}

	if overwrite {
		for _, entry := range entries {
			if entry.Path == targetPath {
				continue
			}

			err = os.Remove(entry.Path)
			if err != nil && !os.IsNotExist(err) {
				return WorkflowSummary{}, ez.Wrap(op, err)
			}
		}
	}

	return summary, nil
}

func DeleteBlueprintByWorkflowID(workflowID string) error {
	const op = "workflow.DeleteBlueprintByWorkflowID"

	entries, err := listRegistryBlueprintEntriesByWorkflowID(workflowID)
	if err != nil {
		return ez.Wrap(op, err)
	}

	for _, entry := range entries {
		err = os.Remove(entry.Path)
		if err != nil {
			return ez.Wrap(op, err)
		}
	}

	return nil
}

func ExportBlueprintByWorkflowID(workflowID string, targetPath string, overwrite bool) error {
	const op = "workflow.ExportBlueprintByWorkflowID"

	trimmedTargetPath := strings.TrimSpace(targetPath)
	if trimmedTargetPath == "" {
		return ez.New(op, ez.EINVALID, "target path is required", nil)
	}

	_, err := os.Stat(trimmedTargetPath)
	if err == nil && !overwrite {
		return ez.New(op, ez.EINVALID, "target file already exists: "+trimmedTargetPath, nil)
	}

	if err != nil && !os.IsNotExist(err) {
		return ez.Wrap(op, err)
	}

	raw, err := ReadBlueprintBytesByWorkflowID(workflowID)
	if err != nil {
		return ez.Wrap(op, err)
	}

	err = writeFileAtomically(trimmedTargetPath, raw)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

func ReadBlueprintBytesByWorkflowID(workflowID string) ([]byte, error) {
	const op = "workflow.ReadBlueprintBytesByWorkflowID"

	entry, err := loadRegistryBlueprintEntryByWorkflowID(workflowID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	raw, err := os.ReadFile(entry.Path)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return raw, nil
}

func Compile(blueprint *Blueprint) (*Snapshot, error) {
	resolver, err := newWorkflowResolver(blueprint)
	if err != nil {
		return nil, ez.Wrap("workflow.Compile", err)
	}

	return compileSnapshot(blueprint, resolver, nil)
}

func compileSnapshot(blueprint *Blueprint, resolver *workflowResolver, stack []string) (*Snapshot, error) {
	const op = "workflow.compileSnapshot"

	if blueprint == nil {
		return nil, ez.New(op, ez.EINVALID, "workflow blueprint is nil", nil)
	}

	workflowID := strings.TrimSpace(blueprint.Workflow.ID)
	if workflowID == "" {
		return nil, ez.New(op, ez.EINVALID, "workflow.id is required", nil)
	}

	if strings.TrimSpace(blueprint.Workflow.Version) == "" {
		return nil, ez.New(op, ez.EINVALID, "workflow.version is required", nil)
	}

	snapshot := &Snapshot{
		WorkflowID:      workflowID,
		WorkflowUUID:    strings.TrimSpace(blueprint.Workflow.UUID),
		WorkflowVersion: blueprint.Workflow.Version,
		Description:     strings.TrimSpace(blueprint.Workflow.Description),
		Inputs:          make(map[string]Port, len(blueprint.Workflow.Inputs)),
		Outputs:         make(map[string]OutputBinding, len(blueprint.Workflow.Outputs)),
	}

	for name, typeRef := range blueprint.Workflow.Inputs {
		schema, err := resolveTypeRef(blueprint, strings.TrimSpace(typeRef))
		if err != nil {
			return nil, ez.Wrap(op, fmt.Errorf("workflow input %q: %w", name, err))
		}

		snapshot.Inputs[name] = Port{
			Name:    name,
			TypeRef: typeRef,
			Schema:  schema,
		}
	}

	compiled, err := compileBlueprint(blueprint, "", resolver, nil, stack)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	snapshot.Nodes = compiled.Nodes

	order, err := topoSort(sortedKeys(snapshot.Nodes), compiled.Dependencies)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}
	snapshot.Order = order

	for outputName, outputSpec := range blueprint.Workflow.Outputs {
		schema, err := resolveTypeRef(blueprint, strings.TrimSpace(outputSpec.Schema))
		if err != nil {
			return nil, ez.Wrap(op, fmt.Errorf("workflow output %q: %w", outputName, err))
		}

		binding, found := compiled.Outputs[outputName]
		if !found {
			return nil, ez.New(op, ez.EINTERNAL, "compiled workflow output binding is missing: "+outputName, nil)
		}

		if binding.Kind != BindingKindInstance {
			return nil, ez.New(op, ez.EINVALID, "workflow outputs must bind from an instance output", nil)
		}

		node, found := snapshot.Nodes[binding.InstanceID]
		if !found {
			return nil, ez.New(op, ez.EINVALID, "workflow output references unknown instance: "+binding.InstanceID, nil)
		}

		_, found = node.Outputs[binding.OutputName]
		if !found {
			return nil, ez.New(op, ez.EINVALID, "workflow output references unknown node output: "+binding.InstanceID+"."+binding.OutputName, nil)
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

func newWorkflowResolver(blueprint *Blueprint) (*workflowResolver, error) {
	const op = "workflow.newWorkflowResolver"

	if blueprint == nil {
		return nil, ez.New(op, ez.EINVALID, "workflow blueprint is nil", nil)
	}

	searchDirs := []string{}
	workflowDir, err := ResolveWorkflowDir()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	if workflowDir != "" {
		searchDirs = append(searchDirs, workflowDir)
	}

	if strings.TrimSpace(blueprint.SourcePath) != "" {
		sourceDir := filepath.Dir(blueprint.SourcePath)
		if sourceDir != "" && sourceDir != workflowDir {
			searchDirs = append(searchDirs, sourceDir)
		}
	}

	return &workflowResolver{
		byID:       map[string]*Blueprint{},
		searchDirs: searchDirs,
	}, nil
}

func (r *workflowResolver) loadByWorkflowID(workflowID string) (*Blueprint, error) {
	const op = "workflow.workflowResolver.loadByWorkflowID"

	blueprint, found := r.byID[workflowID]
	if found {
		return blueprint, nil
	}

	for _, dir := range r.searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, ez.Wrap(op, err)
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
			blueprint, err := LoadBlueprintFile(path)
			if err != nil {
				return nil, ez.Wrap(op, err)
			}

			if blueprint.Workflow.ID == workflowID {
				r.byID[workflowID] = blueprint
				return blueprint, nil
			}
		}
	}

	return nil, ez.New(op, ez.ENOTFOUND, fmt.Sprintf("workflow_id %q not found", workflowID), nil)
}

type registryBlueprintEntry struct {
	Path      string
	Blueprint *Blueprint
}

func loadRegistryBlueprintEntryByWorkflowID(workflowID string) (*registryBlueprintEntry, error) {
	const op = "workflow.loadRegistryBlueprintEntryByWorkflowID"

	entries, err := listRegistryBlueprintEntriesByWorkflowID(workflowID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &entries[0], nil
}

func listRegistryBlueprintEntriesByWorkflowID(workflowID string) ([]registryBlueprintEntry, error) {
	const op = "workflow.listRegistryBlueprintEntriesByWorkflowID"

	trimmedWorkflowID := strings.TrimSpace(workflowID)
	if trimmedWorkflowID == "" {
		return nil, ez.New(op, ez.EINVALID, "workflow_id is required", nil)
	}

	workflowDir, err := ResolveWorkflowDir()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	resolver := &workflowResolver{
		byID:       map[string]*Blueprint{},
		searchDirs: []string{workflowDir},
	}

	blueprint, err := resolver.loadByWorkflowID(trimmedWorkflowID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	if strings.TrimSpace(blueprint.SourcePath) == "" {
		return nil, ez.New(op, ez.EINTERNAL, "registry blueprint source path is missing", nil)
	}

	entries := []registryBlueprintEntry{
		{
			Path:      blueprint.SourcePath,
			Blueprint: blueprint,
		},
	}

	entriesInDir, err := os.ReadDir(workflowDir)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	for _, entry := range entriesInDir {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(workflowDir, entry.Name())
		if path == blueprint.SourcePath {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		candidate, err := LoadBlueprintFile(path)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		if strings.TrimSpace(candidate.Workflow.ID) != trimmedWorkflowID {
			continue
		}

		if strings.TrimSpace(candidate.SourcePath) == "" {
			return nil, ez.New(op, ez.EINTERNAL, "registry blueprint source path is missing", nil)
		}

		entries = append(entries, registryBlueprintEntry{
			Path:      candidate.SourcePath,
			Blueprint: candidate,
		})
	}

	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	return entries, nil
}

func workflowSummaryFromBlueprint(blueprint *Blueprint) (WorkflowSummary, error) {
	const op = "workflow.workflowSummaryFromBlueprint"

	if blueprint == nil {
		return WorkflowSummary{}, ez.New(op, ez.EINVALID, "workflow blueprint is nil", nil)
	}

	workflowID := strings.TrimSpace(blueprint.Workflow.ID)
	if workflowID == "" {
		return WorkflowSummary{}, ez.New(op, ez.EINVALID, "workflow.id is required", nil)
	}

	inputs := make(map[string]string, len(blueprint.Workflow.Inputs))
	for inputName, typeRef := range blueprint.Workflow.Inputs {
		inputs[inputName] = strings.TrimSpace(typeRef)
	}

	outputs := make(map[string]string, len(blueprint.Workflow.Outputs))
	for outputName, outputSpec := range blueprint.Workflow.Outputs {
		outputs[outputName] = strings.TrimSpace(outputSpec.Schema)
	}

	name := strings.TrimSpace(blueprint.Workflow.Name)
	if name == "" {
		name = workflowID
	}

	return WorkflowSummary{
		ID:          workflowID,
		UUID:        strings.TrimSpace(blueprint.Workflow.UUID),
		Name:        name,
		Description: strings.TrimSpace(blueprint.Workflow.Description),
		Inputs:      inputs,
		Outputs:     outputs,
	}, nil
}

func validateRegistryTargetPath(path string, workflowID string) error {
	const op = "workflow.validateRegistryTargetPath"

	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return ez.Wrap(op, err)
	}

	blueprint, err := LoadBlueprintFile(path)
	if err != nil {
		return ez.Wrap(op, err)
	}

	existingWorkflowID := strings.TrimSpace(blueprint.Workflow.ID)
	if existingWorkflowID == "" {
		return ez.New(op, ez.EINVALID, "registry file is missing workflow.id: "+path, nil)
	}

	if existingWorkflowID != workflowID {
		return ez.New(op, ez.EINVALID, fmt.Sprintf("registry file %q already stores workflow_id %q", path, existingWorkflowID), nil)
	}

	return nil
}

func writeFileAtomically(path string, raw []byte) error {
	const op = "workflow.writeFileAtomically"

	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return ez.Wrap(op, err)
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
		return ez.Wrap(op, err)
	}

	err = tempFile.Chmod(0644)
	if err != nil {
		_ = tempFile.Close()
		return ez.Wrap(op, err)
	}

	err = tempFile.Close()
	if err != nil {
		return ez.Wrap(op, err)
	}

	err = os.Rename(tempPath, path)
	if err != nil {
		return ez.Wrap(op, err)
	}

	cleanup = false

	return nil
}

func ResolveWorkflowDir() (string, error) {
	const op = "workflow.ResolveWorkflowDir"

	configRoot := strings.TrimSpace(os.Getenv(workflowHomeEnvVar))
	if configRoot == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", ez.Wrap(op, err)
		}

		configRoot = filepath.Join(userHome, defaultWorkflowHome)
	}

	return filepath.Join(configRoot, defaultWorkflowSubdir), nil
}
