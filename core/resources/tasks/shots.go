package tasks

import (
	"context"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/vanclief/ez"

	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// EnvPublicURL is the agc server's address as people and agents reach it,
// such as https://agc.example.com. A delivery's shot URLs start with it;
// unset, they are root-relative paths (/api/tasks/<task>/shots/<label>.png),
// which the panel opens from the same server.
const EnvPublicURL = "AGC_PUBLIC_URL"

// SetShotsDir sets where delivered screenshots are kept: <dir>/<task>/<label>.png
// (ADR 0010, "Delivery report"). Unset, a delivery with shots is refused.
// Call it before the API is used.
func (api *API) SetShotsDir(dir string) {
	api.shotsDir = dir
}

// ShotPath is the URL path a task's shot is served at.
func ShotPath(taskName string, label string) string {
	return "/api/tasks/" + url.PathEscape(taskName) + "/shots/" + url.PathEscape(label) + ".png"
}

// shotURL is ShotPath under AGC_PUBLIC_URL when it is set.
func shotURL(taskName string, label string) string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv(EnvPublicURL)), "/")

	return base + ShotPath(taskName, label)
}

// storeShots replaces the task's kept shots with files, one <label>.png
// each, and returns report with each shot's File and URL set to where it
// is kept and served. A delivery without shots removes the old ones, so
// the files always match the stored report.
func (api *API) storeShots(taskName string, report *taskmodels.Report, files map[string][]byte) (*taskmodels.Report, error) {
	var shots []taskmodels.Shot
	if report != nil {
		shots = report.Normalized().Shots
	}

	if len(shots) == 0 && len(files) > 0 {
		return nil, ez.New(ez.EINVALID, "files are sent but the report has no shots: name each file in report.shots", nil)
	}

	if api.shotsDir == "" {
		if len(shots) > 0 {
			return nil, ez.New(ez.EUNAVAILABLE, "this agc does not keep screenshots: deliver without shots", nil)
		}

		return report, nil
	}

	dir := filepath.Join(api.shotsDir, taskName)

	if len(shots) == 0 {
		err := os.RemoveAll(dir)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		return report, nil
	}

	err := report.ValidateShotFiles(files)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = os.MkdirAll(api.shotsDir, 0o700)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	staging, err := os.MkdirTemp(api.shotsDir, "."+taskName+"-")
	if err != nil {
		return nil, ez.Wrap(err)
	}
	defer os.RemoveAll(staging)

	stored := *report
	stored.Shots = make([]taskmodels.Shot, 0, len(shots))
	for _, shot := range shots {
		name := shot.Label + ".png"

		err = os.WriteFile(filepath.Join(staging, name), files[shot.Label], 0o600)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		stored.Shots = append(stored.Shots, taskmodels.Shot{Label: shot.Label, File: name, URL: shotURL(taskName, shot.Label)})
	}

	err = os.RemoveAll(dir)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = os.Rename(staging, dir)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &stored, nil
}

// GetShotRequest names one delivered screenshot of a task.
type GetShotRequest struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Label string    `json:"label"`
}

func (r GetShotRequest) Validate() error {
	err := validateRef(r.ID, r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	if !taskmodels.IsShotLabel(r.Label) {
		return ez.New(ez.ENOTFOUND, "no shot "+r.Label+": a shot label is a slug such as before or after", nil)
	}

	return nil
}

// OpenShot opens the task's kept screenshot for the label, a PNG (ADR
// 0010, "Delivery report"); the caller closes it. Not found when the task
// or the shot is absent.
func (api *API) OpenShot(ctx context.Context, requester interface{}, request *GetShotRequest) (*os.File, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if api.shotsDir == "" {
		return nil, ez.New(ez.ENOTFOUND, "task "+record.Name+" has no shot "+request.Label, nil)
	}

	file, err := os.Open(filepath.Join(api.shotsDir, record.Name, request.Label+".png"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ez.New(ez.ENOTFOUND, "task "+record.Name+" has no shot "+request.Label, nil)
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return file, nil
}
