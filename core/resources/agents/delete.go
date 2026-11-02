package agents

import (
	"context"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
)

// DeleteRequest names the agent to delete; the name comes from the path.
type DeleteRequest struct {
	Name string `json:"-"`
}

func (r DeleteRequest) Validate() error {
	if r.Name == "" {
		return ez.New(ez.EINVALID, "name is required", nil)
	}

	return nil
}

type DeleteResponse struct {
	Deleted bool   `json:"deleted"`
	Name    string `json:"name"`
}

// Delete deletes the home sandbox, then the row. The home's volume, the
// agent's memory and transcripts, is kept, as for every sandbox (ADR 0008),
// and so are its messages and the tasks it managed.
func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*DeleteResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.waking.Lock()
	defer api.waking.Unlock()

	record, err := api.find(ctx, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if record.Home != "" {
		_, err = api.sbx.Delete(ctx, nil, &sandboxes.DeleteRequest{Name: record.Home})
		if err != nil && ez.ErrorCode(err) != ez.ENOTFOUND {
			return nil, ez.Wrap(err)
		}
	}

	view, err := api.viewWith(ctx, record, StateDeleted)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = record.Delete(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.publish(view)

	return &DeleteResponse{Deleted: true, Name: record.Name}, nil
}
