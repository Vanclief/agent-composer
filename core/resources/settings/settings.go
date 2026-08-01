package settings

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/vanclief/agent-composer/models/agent"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

// ComposerSettings selects the agent behind "Describe a change…".
type ComposerSettings struct {
	Harness string `json:"harness"`
	Model   string `json:"model"`
}

// Data is everything settings.json holds. Empty fields mean "use the
// default" — for the composer, the first available harness.
type Data struct {
	Composer ComposerSettings `json:"composer"`
}

func settingsPath() (string, error) {
	home, err := workflowruntime.ResolveHomeDir()
	if err != nil {
		return "", ez.Wrap("settings.settingsPath", err)
	}

	return filepath.Join(home, "config", "settings.json"), nil
}

// Load reads settings.json; a missing file is just defaults.
func Load() (Data, error) {
	const op = "settings.Load"

	path, err := settingsPath()
	if err != nil {
		return Data{}, ez.Wrap(op, err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Data{}, nil
		}

		return Data{}, ez.Wrap(op, err)
	}

	var data Data
	err = json.Unmarshal(raw, &data)
	if err != nil {
		return Data{}, ez.Wrap(op, err)
	}

	return data, nil
}

// Save writes settings.json atomically.
func Save(data Data) error {
	const op = "settings.Save"

	path, err := settingsPath()
	if err != nil {
		return ez.Wrap(op, err)
	}

	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return ez.Wrap(op, err)
	}

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return ez.Wrap(op, err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), "settings.*.tmp")
	if err != nil {
		return ez.Wrap(op, err)
	}
	tempPath := tempFile.Name()

	_, err = tempFile.Write(raw)
	if closeErr := tempFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tempPath)
		return ez.Wrap(op, err)
	}

	err = os.Rename(tempPath, path)
	if err != nil {
		_ = os.Remove(tempPath)
		return ez.Wrap(op, err)
	}

	return nil
}

type API struct{}

func NewAPI() *API {
	return &API{}
}

type GetRequest struct{}

func (r *GetRequest) Validate() error {
	return nil
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*Data, error) {
	const op = "settings.API.Get"

	data, err := Load()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &data, nil
}

type UpdateRequest struct {
	Composer ComposerSettings `json:"composer"`
}

func (r *UpdateRequest) Validate() error {
	const op = "settings.UpdateRequest.Validate"

	harness := strings.TrimSpace(r.Composer.Harness)
	if harness != "" {
		err := agent.Harness(harness).Validate()
		if err != nil {
			return ez.New(op, ez.EINVALID, "Unknown harness "+harness, err)
		}
	}

	if harness != "" && strings.TrimSpace(r.Composer.Model) == "" {
		return ez.New(op, ez.EINVALID, "model is required when a harness is set", nil)
	}

	return nil
}

func (api *API) Update(ctx context.Context, requester interface{}, request *UpdateRequest) (*Data, error) {
	const op = "settings.API.Update"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	data, err := Load()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	data.Composer = ComposerSettings{
		Harness: strings.TrimSpace(request.Composer.Harness),
		Model:   strings.TrimSpace(request.Composer.Model),
	}

	err = Save(data)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &data, nil
}
