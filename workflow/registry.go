package workflow

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	workflowmodels "github.com/vanclief/agent-composer/models/workflow"
	"github.com/vanclief/ez"
)

// Registry is the workflow library, stored in the application
// database. Every mutation funnels through saveHead, so the version
// counter and the history in workflow_versions are complete by
// construction — which the old directory of world-writable YAML files
// could never guarantee.
type Registry struct {
	db bun.IDB
}

func NewRegistry(db bun.IDB) *Registry {
	if db == nil {
		return nil
	}

	return &Registry{db: db}
}

// getInstalledBySlug returns the row only when a saved spec exists —
// a draft-only workflow is not installed.
func (r *Registry) getInstalledBySlug(ctx context.Context, workflowID string) (*workflowmodels.Workflow, error) {
	trimmedID := strings.TrimSpace(workflowID)
	if trimmedID == "" {
		return nil, ez.New(ez.EINVALID, "workflow_slug is required", nil)
	}

	record, err := workflowmodels.GetWorkflowBySlug(ctx, r.db, trimmedID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if record.Spec == "" {
		return nil, ez.New(ez.ENOTFOUND, "Workflow "+trimmedID+" was not found", nil)
	}

	return record, nil
}

// seedVersion picks the first installed version: the spec's declared
// integer version when it has one, else 1 — so a workflow imported
// from the old file registry keeps its counter.
func seedVersion(declared string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(declared))
	if err != nil || parsed < 1 {
		return 1
	}

	return parsed
}

// saveHead installs raw as the workflow's next head: the version and
// the permanent uuid are stamped into the YAML, the result must
// compile, the row is updated (the slug follows the spec's
// workflow.slug), and the new head is recorded in workflow_versions.
func (r *Registry) saveHead(ctx context.Context, record *workflowmodels.Workflow, raw []byte, version int) (*Spec, error) {
	stamped, err := stampWorkflowHeader(raw, strconv.Itoa(version), record.ID.String())
	if err != nil {
		return nil, ez.Wrap(err)
	}

	spec, err := ParseSpec(stamped, "")
	if err != nil {
		return nil, ez.Wrap(err)
	}

	_, err = r.Compile(ctx, spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record.Slug = strings.TrimSpace(spec.Workflow.Slug)
	record.Version = version
	record.Spec = string(stamped)

	err = record.Update(ctx, r.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	versionRecord := &workflowmodels.WorkflowVersion{
		WorkflowID: record.ID,
		Version:    version,
		Spec:       record.Spec,
	}

	err = versionRecord.Insert(ctx, r.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return spec, nil
}

// createRow inserts a fresh registry row. A valid, unclaimed uuid in
// the imported spec is adopted so an export/import round trip keeps
// the workflow's identity — otherwise one is minted.
func (r *Registry) createRow(ctx context.Context, slug string, preferredUUID string) (*workflowmodels.Workflow, error) {
	record := &workflowmodels.Workflow{Slug: slug}

	trimmedUUID := strings.TrimSpace(preferredUUID)
	if trimmedUUID != "" {
		parsed, parseErr := uuid.Parse(trimmedUUID)
		if parseErr == nil && parsed != uuid.Nil {
			taken, takenErr := r.uuidTaken(ctx, parsed)
			if takenErr != nil {
				return nil, ez.Wrap(takenErr)
			}
			if !taken {
				record.ID = parsed
			}
		}
	}

	err := record.Insert(ctx, r.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return record, nil
}

func (r *Registry) uuidTaken(ctx context.Context, id uuid.UUID) (bool, error) {
	count, err := r.db.NewSelect().
		Model((*workflowmodels.Workflow)(nil)).
		Where("id = ?", id).
		Count(ctx)
	if err != nil {
		return false, ez.Wrap(err)
	}

	return count > 0, nil
}

// List returns the installed workflows.
func (r *Registry) List(ctx context.Context) ([]WorkflowSummary, error) {
	records, err := workflowmodels.ListWorkflows(ctx, r.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	summaries := make([]WorkflowSummary, 0, len(records))
	for _, record := range records {
		if record.Spec == "" {
			continue
		}

		// A row that fails to parse must not take the whole list down.
		spec, err := ParseSpec([]byte(record.Spec), "")
		if err != nil {
			continue
		}

		summary, err := workflowSummaryFromSpec(spec)
		if err != nil {
			continue
		}

		summary.HasDraft = record.Draft != ""
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// ListDraftOnly returns workflows that exist only as drafts — composed
// but never saved.
func (r *Registry) ListDraftOnly(ctx context.Context) ([]WorkflowSummary, error) {
	records, err := workflowmodels.ListWorkflows(ctx, r.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	summaries := []WorkflowSummary{}
	for _, record := range records {
		if record.Spec != "" || record.Draft == "" {
			continue
		}

		spec, err := ParseSpec([]byte(record.Draft), "")
		if err != nil {
			continue
		}

		summary, err := workflowSummaryFromSpec(spec)
		if err != nil {
			continue
		}

		summary.HasDraft = true
		summary.DraftOnly = true
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// Load parses the installed spec for a workflow id.
func (r *Registry) Load(ctx context.Context, workflowID string) (*Spec, error) {
	record, err := r.getInstalledBySlug(ctx, workflowID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	spec, err := ParseSpec([]byte(record.Spec), "")
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return spec, nil
}

// SpecBytes returns the installed spec's raw YAML.
func (r *Registry) SpecBytes(ctx context.Context, workflowID string) ([]byte, error) {
	record, err := r.getInstalledBySlug(ctx, workflowID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return []byte(record.Spec), nil
}

// ImportFile installs a spec file. Importing over an installed
// workflow requires overwrite and continues its version history — the
// row's identity and counter survive.
func (r *Registry) ImportFile(ctx context.Context, sourcePath string, overwrite bool) (WorkflowSummary, error) {
	trimmedPath := strings.TrimSpace(sourcePath)
	if trimmedPath == "" {
		return WorkflowSummary{}, ez.New(ez.EINVALID, "source path is required", nil)
	}

	raw, err := os.ReadFile(trimmedPath)
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(err)
	}

	spec, err := ParseSpec(raw, trimmedPath)
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(err)
	}

	_, err = r.Compile(ctx, spec)
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(err)
	}

	summary, err := workflowSummaryFromSpec(spec)
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(err)
	}

	installed := false
	record, err := workflowmodels.GetWorkflowBySlug(ctx, r.db, summary.Slug)
	if err == nil {
		installed = record.Spec != ""
	} else {
		if ez.ErrorCode(err) != ez.ENOTFOUND {
			return WorkflowSummary{}, ez.Wrap(err)
		}

		record, err = r.createRow(ctx, summary.Slug, spec.Workflow.ID)
		if err != nil {
			return WorkflowSummary{}, ez.Wrap(err)
		}
	}

	if installed && !overwrite {
		return WorkflowSummary{}, ez.New(ez.EINVALID, "workflow_slug already exists in registry: "+summary.Slug, nil)
	}

	version := seedVersion(spec.Workflow.Version)
	if installed {
		version = record.Version + 1
	}

	saved, err := r.saveHead(ctx, record, raw, version)
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(err)
	}

	summary, err = workflowSummaryFromSpec(saved)
	if err != nil {
		return WorkflowSummary{}, ez.Wrap(err)
	}

	return summary, nil
}

// ExportToFile writes the installed spec's YAML to a file.
func (r *Registry) ExportToFile(ctx context.Context, workflowID string, targetPath string, overwrite bool) error {
	trimmedTargetPath := strings.TrimSpace(targetPath)
	if trimmedTargetPath == "" {
		return ez.New(ez.EINVALID, "target path is required", nil)
	}

	_, err := os.Stat(trimmedTargetPath)
	if err == nil && !overwrite {
		return ez.New(ez.EINVALID, "target file already exists: "+trimmedTargetPath, nil)
	}
	if err != nil && !os.IsNotExist(err) {
		return ez.Wrap(err)
	}

	raw, err := r.SpecBytes(ctx, workflowID)
	if err != nil {
		return ez.Wrap(err)
	}

	err = writeFileAtomically(trimmedTargetPath, raw)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// WorkflowVersionInfo describes one entry of a workflow's history.
type WorkflowVersionInfo struct {
	Version   int       `json:"version"`
	Current   bool      `json:"current,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ListVersions returns a workflow's history, newest first.
func (r *Registry) ListVersions(ctx context.Context, workflowID string) ([]WorkflowVersionInfo, error) {
	record, err := r.getInstalledBySlug(ctx, workflowID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	records, err := workflowmodels.ListWorkflowVersions(ctx, r.db, record.ID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	infos := make([]WorkflowVersionInfo, 0, len(records))
	for _, versionRecord := range records {
		infos = append(infos, WorkflowVersionInfo{
			Version:   versionRecord.Version,
			Current:   versionRecord.Version == record.Version,
			CreatedAt: versionRecord.CreatedAt,
		})
	}

	return infos, nil
}

// GetVersionSpec returns the YAML of one past version.
func (r *Registry) GetVersionSpec(ctx context.Context, workflowID string, version int) (string, error) {
	record, err := r.getInstalledBySlug(ctx, workflowID)
	if err != nil {
		return "", ez.Wrap(err)
	}

	versionRecord, err := workflowmodels.GetWorkflowVersion(ctx, r.db, record.ID, version)
	if err != nil {
		return "", ez.Wrap(err)
	}

	return versionRecord.Spec, nil
}

// RestoreVersion re-installs a past version as a new head. History
// only moves forward — a restore adds a version, it never rewrites
// the past.
func (r *Registry) RestoreVersion(ctx context.Context, workflowID string, version int) (*SavedDraft, error) {
	record, err := r.getInstalledBySlug(ctx, workflowID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	versionRecord, err := workflowmodels.GetWorkflowVersion(ctx, r.db, record.ID, version)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	_, err = r.saveHead(ctx, record, []byte(versionRecord.Spec), record.Version+1)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &SavedDraft{
		WorkflowSlug: record.Slug,
		Version:    strconv.Itoa(record.Version),
		Spec:       record.Spec,
	}, nil
}

// VerifyProposedSpec compiles a composer proposal and confirms
// its workflow.slug, before the proposal may become a draft.
func (r *Registry) VerifyProposedSpec(ctx context.Context, raw []byte, expectedID string) (string, error) {
	spec, err := ParseSpec(raw, "")
	if err != nil {
		return "", ez.Wrap(err)
	}

	_, err = r.Compile(ctx, spec)
	if err != nil {
		return "", ez.Wrap(err)
	}

	proposedID := strings.TrimSpace(spec.Workflow.Slug)
	if expectedID != "" && proposedID != expectedID {
		return "", ez.New(ez.EINVALID, "The proposal changed workflow.slug from "+expectedID+" to "+proposedID, nil)
	}

	return proposedID, nil
}
