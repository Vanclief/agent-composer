package workflow

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/vanclief/ez"
	yaml "gopkg.in/yaml.v3"
)

// Drafts are proposed blueprints awaiting an explicit save. They live
// outside the registry (the registry only reads its own top-level
// files), so a draft is never executable until promoted.

func resolveDraftPath(workflowID string) (string, error) {
	home, err := ResolveHomeDir()
	if err != nil {
		return "", ez.Wrap(err)
	}

	return filepath.Join(home, "drafts", workflowID+".yaml"), nil
}

func resolveVersionsDir(workflowID string) (string, error) {
	home, err := ResolveHomeDir()
	if err != nil {
		return "", ez.Wrap(err)
	}

	return filepath.Join(home, "versions", workflowID), nil
}

// ReadDraft returns the draft blueprint for a workflow, or "" when
// none exists.
func ReadDraft(workflowID string) (string, error) {
	path, err := resolveDraftPath(strings.TrimSpace(workflowID))
	if err != nil {
		return "", ez.Wrap(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}

		return "", ez.Wrap(err)
	}

	return string(raw), nil
}

// WriteDraft stores a proposed blueprint. The content must already be
// compile-checked by the caller.
func WriteDraft(workflowID string, raw []byte) error {
	path, err := resolveDraftPath(strings.TrimSpace(workflowID))
	if err != nil {
		return ez.Wrap(err)
	}

	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return ez.Wrap(err)
	}

	err = writeFileAtomically(path, raw)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// DeleteDraft discards a draft; a missing draft is not an error.
func DeleteDraft(workflowID string) error {
	path, err := resolveDraftPath(strings.TrimSpace(workflowID))
	if err != nil {
		return ez.Wrap(err)
	}

	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return ez.Wrap(err)
	}

	return nil
}

// slugifyWorkflowID turns a display name into a registry-style id:
// lowercase snake_case, matching the installed files' convention.
func slugifyWorkflowID(name string) string {
	var builder strings.Builder
	pendingSeparator := false
	for _, char := range strings.ToLower(strings.TrimSpace(name)) {
		isWordChar := (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9')
		if !isWordChar {
			pendingSeparator = builder.Len() > 0
			continue
		}
		if pendingSeparator {
			builder.WriteByte('_')
			pendingSeparator = false
		}
		builder.WriteRune(char)
	}

	return builder.String()
}

type CreatedDraft struct {
	WorkflowID string
	Spec       string
}

// CreateDraft scaffolds a new named workflow as a draft: just the
// workflow header, no nodes — the composer and inspector fill in the
// rest. The id derives from the name unless the caller picks one;
// collisions with installed workflows or existing drafts are rejected.
func CreateDraft(name, description, explicitID string) (*CreatedDraft, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, ez.New(ez.EINVALID, "A workflow name is required", nil)
	}

	workflowID := strings.TrimSpace(explicitID)
	if workflowID == "" {
		workflowID = slugifyWorkflowID(trimmedName)
		if workflowID == "" {
			return nil, ez.New(ez.EINVALID, "The name must contain letters or digits", nil)
		}
	} else {
		err := ValidateWorkflowID(workflowID)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	_, err := loadRegistryBlueprintEntryByWorkflowID(workflowID)
	if err == nil {
		return nil, ez.New(ez.EINVALID, "Workflow "+workflowID+" already exists", nil)
	}
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		return nil, ez.Wrap(err)
	}

	existingDraft, err := ReadDraft(workflowID)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if existingDraft != "" {
		return nil, ez.New(ez.EINVALID, "A draft for "+workflowID+" already exists", nil)
	}

	var scaffold struct {
		Workflow struct {
			ID          string `yaml:"id"`
			UUID        string `yaml:"uuid"`
			Name        string `yaml:"name"`
			Version     string `yaml:"version"`
			Description string `yaml:"description,omitempty"`
		} `yaml:"workflow"`
	}
	scaffold.Workflow.ID = workflowID
	scaffold.Workflow.UUID = uuid.NewString()
	scaffold.Workflow.Name = trimmedName
	scaffold.Workflow.Version = "1"
	scaffold.Workflow.Description = strings.TrimSpace(description)

	raw, err := yaml.Marshal(&scaffold)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = WriteDraft(workflowID, raw)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &CreatedDraft{
		WorkflowID: workflowID,
		Spec:       string(raw),
	}, nil
}

// ListDraftOnlyBlueprints returns summaries for drafts whose workflow
// is not installed in the registry — newly composed workflows that
// have never been saved.
func ListDraftOnlyBlueprints() ([]WorkflowSummary, error) {
	home, err := ResolveHomeDir()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	entries, err := os.ReadDir(filepath.Join(home, "drafts"))
	if err != nil {
		if os.IsNotExist(err) {
			return []WorkflowSummary{}, nil
		}

		return nil, ez.Wrap(err)
	}

	installed, err := ListBlueprints()
	if err != nil {
		return nil, ez.Wrap(err)
	}
	installedIDs := make(map[string]bool, len(installed))
	for _, summary := range installed {
		installedIDs[summary.ID] = true
	}

	summaries := []WorkflowSummary{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		blueprint, err := LoadBlueprintFile(
			filepath.Join(home, "drafts", entry.Name()),
		)
		if err != nil {
			// A broken draft should not take the list down.
			continue
		}

		if installedIDs[blueprint.Workflow.ID] {
			continue
		}

		summary, err := workflowSummaryFromBlueprint(blueprint)
		if err != nil {
			continue
		}

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// nextVersion picks the version stamped on save: the saved file's
// integer version plus one, 1 for a first save, 2 when the current
// version is not an integer.
func nextVersion(currentVersion string, installed bool) string {
	if !installed {
		return "1"
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(currentVersion))
	if err != nil {
		return "2"
	}

	return strconv.Itoa(parsed + 1)
}

// stampWorkflowHeader rewrites workflow.version and workflow.uuid in
// place, preserving the rest of the document byte-for-byte where
// possible. Empty values leave their field untouched.
func stampWorkflowHeader(raw []byte, version, workflowUUID string) ([]byte, error) {
	var doc yaml.Node
	err := yaml.Unmarshal(raw, &doc)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if len(doc.Content) == 0 {
		return nil, ez.New(ez.EINVALID, "the draft is empty", nil)
	}

	workflowMap := findMapValue(doc.Content[0], "workflow")
	if workflowMap == nil {
		return nil, ez.New(ez.EINVALID, "the draft has no workflow section", nil)
	}

	if version != "" {
		setScalarValue(workflowMap, "version", version)
	}
	if workflowUUID != "" {
		setScalarValue(workflowMap, "uuid", workflowUUID)
	}

	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	err = encoder.Encode(&doc)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	err = encoder.Close()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return buffer.Bytes(), nil
}

// StampWorkflowUUID forces workflow.uuid in a blueprint's bytes —
// used to carry a workflow's permanent identity into a proposal that
// dropped or fabricated it.
func StampWorkflowUUID(raw []byte, workflowUUID string) ([]byte, error) {
	return stampWorkflowHeader(raw, "", workflowUUID)
}

// archivePath finds a free file name for the outgoing version.
func archivePath(dir string, version string) string {
	base := filepath.Join(dir, "v"+version+".yaml")
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base
	}

	for i := 2; ; i++ {
		candidate := filepath.Join(
			dir,
			fmt.Sprintf("v%s-%d.yaml", version, i),
		)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

type SavedDraft struct {
	WorkflowID string
	Version    string
	Spec       string
}

// SaveDraft promotes a draft: compile it, stamp the next version,
// archive the outgoing file, replace the registry file atomically,
// and delete the draft.
func SaveDraft(workflowID string) (*SavedDraft, error) {
	trimmedID := strings.TrimSpace(workflowID)
	draft, err := ReadDraft(trimmedID)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if draft == "" {
		return nil, ez.New(ez.ENOTFOUND, "There is no draft for "+trimmedID, nil)
	}

	draftPath, err := resolveDraftPath(trimmedID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	blueprint, err := LoadBlueprintFile(draftPath)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if blueprint.Workflow.ID != trimmedID {
		return nil, ez.New(ez.EINVALID, "The draft's workflow.id does not match "+trimmedID, nil)
	}

	// Where the registry file lives — an existing entry keeps its
	// path, a first save lands on <id>.yaml in the registry.
	installed := true
	entry, err := loadRegistryBlueprintEntryByWorkflowID(trimmedID)
	if err != nil {
		if ez.ErrorCode(err) != ez.ENOTFOUND {
			return nil, ez.Wrap(err)
		}
		installed = false
	}

	currentVersion := ""
	targetPath := ""
	if installed {
		currentVersion = entry.Blueprint.Workflow.Version
		targetPath = entry.Path
	} else {
		workflowDir, err := ResolveWorkflowDir()
		if err != nil {
			return nil, ez.Wrap(err)
		}
		err = os.MkdirAll(workflowDir, 0755)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		targetPath = filepath.Join(workflowDir, trimmedID+".yaml")
	}

	version := nextVersion(currentVersion, installed)

	// The permanent identity: the installed file's uuid always wins —
	// a draft cannot change it. First installs mint one.
	workflowUUID := ""
	if installed {
		workflowUUID = strings.TrimSpace(entry.Blueprint.Workflow.UUID)
	}
	if workflowUUID == "" {
		workflowUUID = strings.TrimSpace(blueprint.Workflow.UUID)
	}
	if workflowUUID == "" {
		workflowUUID = uuid.NewString()
	}

	raw, err := os.ReadFile(draftPath)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	stamped, err := stampWorkflowHeader(raw, version, workflowUUID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	// The stamped draft must still compile before it replaces the
	// registry file.
	scratch, err := os.CreateTemp("", "agc-save-*.yaml")
	if err != nil {
		return nil, ez.Wrap(err)
	}
	scratchPath := scratch.Name()
	defer os.Remove(scratchPath)

	_, err = scratch.Write(stamped)
	if closeErr := scratch.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	compiled, err := LoadBlueprintFile(scratchPath)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	_, err = Compile(compiled)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	// Archive the outgoing version before it is replaced.
	if installed {
		versionsDir, err := resolveVersionsDir(trimmedID)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		err = os.MkdirAll(versionsDir, 0755)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		outgoing, err := os.ReadFile(entry.Path)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		archiveVersion := strings.TrimSpace(currentVersion)
		if archiveVersion == "" {
			archiveVersion = "0"
		}
		err = writeFileAtomically(
			archivePath(versionsDir, archiveVersion),
			outgoing,
		)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	err = writeFileAtomically(targetPath, stamped)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = DeleteDraft(trimmedID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &SavedDraft{
		WorkflowID: trimmedID,
		Version:    version,
		Spec:       string(stamped),
	}, nil
}
