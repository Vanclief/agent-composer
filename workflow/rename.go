package workflow

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vanclief/ez"
	yaml "gopkg.in/yaml.v3"
)

// Workflow ids are friendly slugs, so they double as file names, URLs,
// CLI arguments, and cross-workflow references. Renaming one therefore
// cascades: the registry file, the draft, the versions archive, and
// every blueprint that embeds the workflow all follow the new id. The
// executions table is the caller's to update — this package has no
// database access.

var workflowIDPattern = regexp.MustCompile(`^[a-z0-9]+([_-][a-z0-9]+)*$`)

// ValidateWorkflowID accepts lowercase slug ids: letters and digits
// separated by single underscores or hyphens.
func ValidateWorkflowID(id string) error {
	if !workflowIDPattern.MatchString(id) {
		return ez.New(ez.EINVALID, "Workflow ids are lowercase slugs like parallel_pr_review", nil)
	}

	return nil
}

type RenameResult struct {
	WorkflowID string
	// UpdatedRefs lists workflow ids whose blueprints embedded the
	// renamed workflow and were rewritten to the new id.
	UpdatedRefs []string
}

// encodeYAMLDoc renders an edited yaml.v3 document back to bytes.
func encodeYAMLDoc(doc *yaml.Node) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	err := encoder.Encode(doc)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	err = encoder.Close()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return buffer.Bytes(), nil
}

// rewriteWorkflowHeader surgically sets workflow.id, workflow.name,
// and/or workflow.description in a blueprint's bytes. Empty id/name
// leave the field unchanged; a nil description does too, while ""
// removes it.
func rewriteWorkflowHeader(raw []byte, id, name string, description *string) ([]byte, error) {
	var doc yaml.Node
	err := yaml.Unmarshal(raw, &doc)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if len(doc.Content) == 0 {
		return nil, ez.New(ez.EINVALID, "The workflow file is empty", nil)
	}

	workflowMap := findMapValue(doc.Content[0], "workflow")
	if workflowMap == nil {
		return nil, ez.New(ez.EINVALID, "The file has no workflow section", nil)
	}

	if id != "" {
		setScalarValue(workflowMap, "id", id)
	}
	if name != "" {
		setScalarValue(workflowMap, "name", name)
	}
	if description != nil {
		if *description == "" {
			removeMapKey(workflowMap, "description")
		} else {
			setScalarValue(workflowMap, "description", *description)
		}
	}

	return encodeYAMLDoc(&doc)
}

// rewriteWorkflowReferences rewrites nodes.*.workflow_id values from
// oldID to newID. Returns nil bytes when nothing referenced oldID.
func rewriteWorkflowReferences(raw []byte, oldID, newID string) ([]byte, error) {
	var doc yaml.Node
	err := yaml.Unmarshal(raw, &doc)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}

	nodesMap := findMapValue(doc.Content[0], "nodes")
	if nodesMap == nil || nodesMap.Kind != yaml.MappingNode {
		return nil, nil
	}

	changed := false
	for i := 0; i+1 < len(nodesMap.Content); i += 2 {
		reference := findMapValue(nodesMap.Content[i+1], "workflow_id")
		if reference != nil && reference.Value == oldID {
			reference.Value = newID
			changed = true
		}
	}
	if !changed {
		return nil, nil
	}

	return encodeYAMLDoc(&doc)
}

// compileBlueprintBytes verifies that blueprint bytes still compile.
func compileBlueprintBytes(raw []byte) error {
	scratch, err := os.CreateTemp("", "agc-rename-*.yaml")
	if err != nil {
		return ez.Wrap(err)
	}
	scratchPath := scratch.Name()
	defer os.Remove(scratchPath)

	_, err = scratch.Write(raw)
	if closeErr := scratch.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return ez.Wrap(err)
	}

	blueprint, err := LoadBlueprintFile(scratchPath)
	if err != nil {
		return ez.Wrap(err)
	}
	_, err = Compile(blueprint)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// RenameWorkflowID moves a workflow (installed, drafted, or both) to a
// new id and updates every blueprint that embeds it.
func RenameWorkflowID(oldID, newID string) (*RenameResult, error) {
	oldID = strings.TrimSpace(oldID)
	newID = strings.TrimSpace(newID)
	if oldID == newID {
		return &RenameResult{WorkflowID: newID}, nil
	}
	err := ValidateWorkflowID(newID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	// The new id must be free — installed or drafted.
	_, err = loadRegistryBlueprintEntryByWorkflowID(newID)
	if err == nil {
		return nil, ez.New(ez.EINVALID, "Workflow "+newID+" already exists", nil)
	}
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		return nil, ez.Wrap(err)
	}
	collidingDraft, err := ReadDraft(newID)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if collidingDraft != "" {
		return nil, ez.New(ez.EINVALID, "A draft for "+newID+" already exists", nil)
	}

	installed := true
	entry, err := loadRegistryBlueprintEntryByWorkflowID(oldID)
	if err != nil {
		if ez.ErrorCode(err) != ez.ENOTFOUND {
			return nil, ez.Wrap(err)
		}
		installed = false
	}
	draft, err := ReadDraft(oldID)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if !installed && draft == "" {
		return nil, ez.New(ez.ENOTFOUND, "Workflow "+oldID+" was not found", nil)
	}

	// 1. The registry file moves first, so reference rewrites compile
	// against the new id.
	if installed {
		raw, err := os.ReadFile(entry.Path)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		renamed, err := rewriteWorkflowHeader(raw, newID, "", nil)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		err = compileBlueprintBytes(renamed)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		workflowDir, err := ResolveWorkflowDir()
		if err != nil {
			return nil, ez.Wrap(err)
		}
		err = writeFileAtomically(
			filepath.Join(workflowDir, newID+".yaml"),
			renamed,
		)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		err = os.Remove(entry.Path)
		if err != nil && !os.IsNotExist(err) {
			return nil, ez.Wrap(err)
		}
	}

	// 2. The draft follows.
	if draft != "" {
		renamedDraft, err := rewriteWorkflowHeader([]byte(draft), newID, "", nil)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		err = WriteDraft(newID, renamedDraft)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		err = DeleteDraft(oldID)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	// 3. The versions archive keeps its history under the new id.
	oldVersionsDir, err := resolveVersionsDir(oldID)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	newVersionsDir, err := resolveVersionsDir(newID)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	err = os.Rename(oldVersionsDir, newVersionsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, ez.Wrap(err)
	}

	// 4. Blueprints embedding the old id follow it — installed files
	// (compile-checked) and drafts (re-verified at their own save).
	updatedRefs := []string{}
	if installed {
		summaries, err := ListBlueprints()
		if err != nil {
			return nil, ez.Wrap(err)
		}
		for _, summary := range summaries {
			if summary.ID == newID {
				continue
			}
			refEntry, err := loadRegistryBlueprintEntryByWorkflowID(summary.ID)
			if err != nil {
				continue
			}
			raw, err := os.ReadFile(refEntry.Path)
			if err != nil {
				continue
			}
			rewritten, err := rewriteWorkflowReferences(raw, oldID, newID)
			if err != nil || rewritten == nil {
				continue
			}
			err = compileBlueprintBytes(rewritten)
			if err != nil {
				return nil, ez.Wrap(err)
			}
			err = writeFileAtomically(refEntry.Path, rewritten)
			if err != nil {
				return nil, ez.Wrap(err)
			}
			updatedRefs = append(updatedRefs, summary.ID)
		}
	}

	home, err := ResolveHomeDir()
	if err != nil {
		return nil, ez.Wrap(err)
	}
	draftEntries, err := os.ReadDir(filepath.Join(home, "drafts"))
	if err != nil && !os.IsNotExist(err) {
		return nil, ez.Wrap(err)
	}
	for _, draftEntry := range draftEntries {
		if draftEntry.IsDir() ||
			!strings.HasSuffix(draftEntry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(home, "drafts", draftEntry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		rewritten, err := rewriteWorkflowReferences(raw, oldID, newID)
		if err != nil || rewritten == nil {
			continue
		}
		err = writeFileAtomically(path, rewritten)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	return &RenameResult{
		WorkflowID:  newID,
		UpdatedRefs: updatedRefs,
	}, nil
}

// SetWorkflowHeader rewrites workflow.name and/or
// workflow.description wherever the workflow lives — the registry
// file and/or the pending draft. An empty name is unchanged; a nil
// description is unchanged, "" clears it.
func SetWorkflowHeader(workflowID, name string, description *string) error {
	workflowID = strings.TrimSpace(workflowID)
	name = strings.TrimSpace(name)
	if name == "" && description == nil {
		return ez.New(ez.EINVALID, "Nothing to update", nil)
	}

	installed := true
	entry, err := loadRegistryBlueprintEntryByWorkflowID(workflowID)
	if err != nil {
		if ez.ErrorCode(err) != ez.ENOTFOUND {
			return ez.Wrap(err)
		}
		installed = false
	}
	draft, err := ReadDraft(workflowID)
	if err != nil {
		return ez.Wrap(err)
	}
	if !installed && draft == "" {
		return ez.New(ez.ENOTFOUND, "Workflow "+workflowID+" was not found", nil)
	}

	if installed {
		raw, err := os.ReadFile(entry.Path)
		if err != nil {
			return ez.Wrap(err)
		}
		renamed, err := rewriteWorkflowHeader(raw, "", name, description)
		if err != nil {
			return ez.Wrap(err)
		}
		err = compileBlueprintBytes(renamed)
		if err != nil {
			return ez.Wrap(err)
		}
		err = writeFileAtomically(entry.Path, renamed)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	if draft != "" {
		renamedDraft, err := rewriteWorkflowHeader([]byte(draft), "", name, description)
		if err != nil {
			return ez.Wrap(err)
		}
		err = WriteDraft(workflowID, renamedDraft)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	return nil
}
