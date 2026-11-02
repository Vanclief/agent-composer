package sandboxes

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/executor/sandbox"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// GCInterval is how often agc rest runs the collector (ADR 0008 "Garbage
// collection").
const GCInterval = 10 * time.Minute

// GCRequest collects the stopped sandboxes of tasks done longer than Keep
// ago. Keep is a duration such as 24h; empty is keep_done, or the built-in
// 24h when keep_done is 0 (the collector off). DryRun only lists them.
type GCRequest struct {
	DryRun bool   `json:"dry_run"`
	Keep   string `json:"keep"`
}

func (r GCRequest) Validate() error {
	if r.Keep == "" {
		return nil
	}

	_, err := controller.ParseKeep(r.Keep)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// Collected is one sandbox the collector deletes, or would.
type Collected struct {
	Name string `json:"name"`
	Task string `json:"task"`
	// DoneFor is how long ago the task was done, such as 26h3m0s.
	DoneFor string `json:"done_for"`
	Disk    string `json:"disk"`
	DiskMiB uint32 `json:"disk_mib"`
	// Error is why the delete failed; empty when it did not.
	Error string `json:"error,omitempty"`
}

// GCResponse lists what went (or would go, with DryRun). Freed counts the
// sandboxes deleted, 0 on a dry run, and FreedDisk their disks.
type GCResponse struct {
	DryRun    bool        `json:"dry_run"`
	Keep      string      `json:"keep"`
	Sandboxes []Collected `json:"sandboxes"`
	Freed     int         `json:"freed"`
	FreedDisk string      `json:"freed_disk"`
}

// GC deletes, through Remove like DELETE /api/sandboxes/{id}, every
// sandbox that is stopped, is not an agent's home and belongs to a task
// done longer than keep ago. Volumes are kept. A failed delete is logged
// and reported, and the rest go on.
func (api *API) GC(ctx context.Context, requester interface{}, request *GCRequest) (*GCResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	keep := api.keepDone
	if keep == 0 {
		keep = controller.BuiltinKeepDone
	}
	if request.Keep != "" {
		keep, err = controller.ParseKeep(request.Keep)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	return api.collect(ctx, keep, request.DryRun)
}

// StartGC runs the collector every GCInterval until ctx ends, one run at a
// time. Only agc rest calls it. Each tick deletes the sandboxes of tasks
// done longer than keep_done ago and archives the channels of tasks done
// longer than archive_done ago (ADR 0005 "Archived channels"); a setting
// of 0 turns its half off, and both 0 stops the ticker altogether.
func (api *API) StartGC(ctx context.Context) {
	if api.keepDone == 0 {
		log.Info().Msg("sandboxes: collector off, keep_done is 0")
	}
	if api.archiveDone == 0 {
		log.Info().Msg("messaging: archiving off, archive_done is 0")
	}
	if api.keepDone == 0 && api.archiveDone == 0 {
		return
	}

	log.Info().Dur("every", GCInterval).Dur("keep_done", api.keepDone).Dur("archive_done", api.archiveDone).Msg("sandboxes: collector started")

	ticker := time.NewTicker(GCInterval)
	defer ticker.Stop()

	for {
		api.tick(ctx)

		select {
		case <-ctx.Done():
			log.Info().Msg("sandboxes: collector stopped")
			return
		case <-ticker.C:
		}
	}
}

// tick is one run of the collector: sandboxes, then channels.
func (api *API) tick(ctx context.Context) {
	if api.keepDone > 0 {
		_, err := api.collect(ctx, api.keepDone, false)
		if err != nil {
			log.Warn().Err(err).Msg("sandboxes: collector run failed")
		}
	}

	if api.archiveDone > 0 {
		_, err := api.archiveChannels(ctx, api.archiveDone)
		if err != nil {
			log.Warn().Err(err).Msg("messaging: archiving run failed")
		}
	}
}

// collect finds the sandboxes to go and, unless dryRun, removes them. Runs
// are serialized, so the timer and a hand run never overlap.
func (api *API) collect(ctx context.Context, keep time.Duration, dryRun bool) (*GCResponse, error) {
	api.collecting.Lock()
	defer api.collecting.Unlock()

	records, err := taskmodels.ListSandboxes(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	owners, err := api.tasksByID(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	now := api.now()
	response := &GCResponse{DryRun: dryRun, Keep: keep.String(), Sandboxes: []Collected{}}
	var freed uint32

	for i := range records {
		record := &records[i]

		owner := owners.of(record)
		if !api.collectable(ctx, record, owner, now, keep) {
			continue
		}

		disk, mib := diskOf(record)
		item := Collected{
			Name:    record.Name,
			Task:    owner.Name,
			DoneFor: now.Sub(*owner.DoneAt).Round(time.Minute).String(),
			Disk:    disk,
			DiskMiB: mib,
		}

		if !dryRun {
			err = api.Remove(ctx, record)
			if err != nil {
				log.Warn().Err(err).Str("sandbox", item.Name).Str("task", item.Task).Msg("sandboxes: collector cannot delete sandbox")
				item.Error = ez.ErrorMessage(err)
				response.Sandboxes = append(response.Sandboxes, item)
				continue
			}

			log.Info().Str("sandbox", item.Name).Str("task", item.Task).Str("done_for", item.DoneFor).Str("disk", item.Disk).Msg("sandboxes: collector deleted sandbox")
			response.Freed++
			freed += mib
		}

		response.Sandboxes = append(response.Sandboxes, item)
	}

	response.FreedDisk = sandbox.FormatSize(freed)

	return response, nil
}

// collectable is whether the collector takes record: not a home, its task
// done longer than keep ago, booted, and stopped by the runtime's word.
func (api *API) collectable(ctx context.Context, record *taskmodels.Sandbox, owner *taskmodels.Task, now time.Time, keep time.Duration) bool {
	if record.TaskID == uuid.Nil || record.Agent != "" {
		return false
	}

	if owner.DoneAt == nil || now.Sub(*owner.DoneAt) < keep {
		return false
	}

	if record.BootedAt == nil || record.BootError != nil {
		return false
	}

	status, err := api.rt.Status(ctx, record.Name)
	if err != nil {
		return false
	}

	return status == StatusStopped
}

// diskOf is the sandbox's disk from its spec, default 20G, as a size and in
// MiB. An unreadable spec counts as the default.
func diskOf(record *taskmodels.Sandbox) (string, uint32) {
	spec, err := decodeSpec(record)
	if err != nil {
		return sandbox.FormatSize(sandbox.DefaultDiskMiB), sandbox.DefaultDiskMiB
	}

	mib, err := spec.DiskMiB()
	if err != nil || mib == 0 {
		mib = sandbox.DefaultDiskMiB
	}

	return sandbox.FormatSize(mib), mib
}
