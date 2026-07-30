// Package harnessinfo reports which harnesses are installed on this
// machine and which models each can run.
package harnessinfo

import (
	"context"

	"github.com/vanclief/agent-composer/runtime/harnesses"
)

type API struct{}

func NewAPI() *API {
	return &API{}
}

type ListRequest struct{}

func (r *ListRequest) Validate() error {
	return nil
}

type ListResponse struct {
	Harnesses []harnesses.HarnessInfo `json:"harnesses"`
}

func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	return &ListResponse{
		Harnesses: harnesses.ListHarnessInfo(ctx),
	}, nil
}
