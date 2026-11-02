package tasks

import (
	"context"
	"strings"

	"github.com/vanclief/ez"

	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// Saved specs are reusable sandbox specs: the sandbox block of a task.yaml
// at top level (task.Store).

type ListSpecsRequest struct{}

func (r ListSpecsRequest) Validate() error {
	return nil
}

type ListSpecsResponse struct {
	Specs []task.Saved `json:"specs"`
}

func (api *API) ListSpecs(ctx context.Context, requester interface{}, request *ListSpecsRequest) (*ListSpecsResponse, error) {
	specs, err := api.specs.List()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &ListSpecsResponse{Specs: specs}, nil
}

type GetSpecRequest struct {
	Name string `json:"name"`
}

func (r GetSpecRequest) Validate() error {
	return taskmodels.ValidateName(r.Name)
}

func (api *API) GetSpec(ctx context.Context, requester interface{}, request *GetSpecRequest) (*task.Saved, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	saved, err := api.specs.Get(request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &saved, nil
}

// SaveSpecRequest may name the spec in free text: "Backend review" is
// saved as backend-review.
type SaveSpecRequest struct {
	Name string `json:"name"`
	YAML string `json:"yaml"`
}

func (r SaveSpecRequest) Validate() error {
	_, err := taskmodels.Slug(r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	if strings.TrimSpace(r.YAML) == "" {
		return ez.New(ez.EINVALID, "yaml is required", nil)
	}

	return nil
}

func (api *API) SaveSpec(ctx context.Context, requester interface{}, request *SaveSpecRequest) (*task.Saved, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	name, err := taskmodels.Slug(request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	saved, err := api.specs.Save(name, request.YAML)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &saved, nil
}

type DeleteSpecRequest struct {
	Name string `json:"name"`
}

func (r DeleteSpecRequest) Validate() error {
	return taskmodels.ValidateName(r.Name)
}

// DeleteSpec removes a saved spec and returns what it held.
func (api *API) DeleteSpec(ctx context.Context, requester interface{}, request *DeleteSpecRequest) (*task.Saved, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	saved, err := api.specs.Get(request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.specs.Delete(request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &saved, nil
}
