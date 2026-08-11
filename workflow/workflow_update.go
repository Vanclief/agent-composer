package workflow

import (
	"context"
	"strings"

	"github.com/vanclief/ez"
	yaml "gopkg.in/yaml.v3"
)

// NodeConfigUpdate carries the editable fields of a node's config.
// A nil field is left unchanged.
type NodeConfigUpdate struct {
	Model       *string
	Harness     *string
	Instruction *string
	// "" removes the field — the harness default takes over.
	ReasoningEffort *string
}

// UpdateNodeConfig edits one node's config in the workflow's YAML.
// The edit is surgical (yaml.v3 node tree), so comments and
// formatting survive, and the result lands as a new compiled version
// — an edit that breaks the spec never installs.
func (r *Registry) UpdateNodeConfig(ctx context.Context, workflowID, nodeName string, update NodeConfigUpdate) error {
	record, err := r.getInstalledBySlug(ctx, workflowID)
	if err != nil {
		return ez.Wrap(err)
	}

	var doc yaml.Node
	err = yaml.Unmarshal([]byte(record.Spec), &doc)
	if err != nil {
		return ez.Wrap(err)
	}
	if len(doc.Content) == 0 {
		return ez.New(ez.EINVALID, "The workflow spec is empty", nil)
	}
	root := doc.Content[0]

	nodesMap := findMapValue(root, "nodes")
	if nodesMap == nil {
		return ez.New(ez.EINVALID, "The workflow has no nodes section", nil)
	}
	nodeMap := findMapValue(nodesMap, nodeName)
	if nodeMap == nil {
		return ez.New(ez.ENOTFOUND, "Node "+nodeName+" was not found in "+workflowID, nil)
	}

	configMap := ensureMapValue(nodeMap, "config")
	if update.Instruction != nil {
		setScalarValue(configMap, "instruction", *update.Instruction)
	}
	if update.Model != nil || update.Harness != nil ||
		update.ReasoningEffort != nil {
		harnessMap := ensureMapValue(configMap, "harness")
		if update.Harness != nil {
			setScalarValue(harnessMap, "id", *update.Harness)
		}
		if update.Model != nil {
			setScalarValue(harnessMap, "model", *update.Model)
		}
		if update.ReasoningEffort != nil {
			if *update.ReasoningEffort == "" {
				removeMapKey(harnessMap, "reasoning_effort")
			} else {
				setScalarValue(
					harnessMap,
					"reasoning_effort",
					*update.ReasoningEffort,
				)
			}
		}
	}

	edited, err := encodeYAMLDoc(&doc)
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = r.saveHead(ctx, record, edited, record.Version+1)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func findMapValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func ensureMapValue(mapping *yaml.Node, key string) *yaml.Node {
	existing := findMapValue(mapping, key)
	if existing != nil {
		return existing
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valueNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mapping.Content = append(mapping.Content, keyNode, valueNode)
	return valueNode
}

func removeMapKey(mapping *yaml.Node, key string) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(
				mapping.Content[:i],
				mapping.Content[i+2:]...,
			)
			return
		}
	}
}

func setScalarValue(mapping *yaml.Node, key, value string) {
	target := findMapValue(mapping, key)
	if target == nil {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
		target = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str"}
		mapping.Content = append(mapping.Content, keyNode, target)
	}

	target.Kind = yaml.ScalarNode
	target.Tag = "!!str"
	target.Value = value
	target.Content = nil
	// Multiline text needs a block style; short values render plain.
	if strings.Contains(value, "\n") {
		target.Style = yaml.LiteralStyle
	} else {
		target.Style = 0
	}
}
