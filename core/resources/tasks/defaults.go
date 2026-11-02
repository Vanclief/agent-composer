package tasks

import (
	"context"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// GetDefaultsRequest reads the defaults Create fills in (ADR 0009).
type GetDefaultsRequest struct{}

func (r GetDefaultsRequest) Validate() error {
	return nil
}

// DefaultsResponse is the effective harness, model and owner after the
// config file and AGC_DEFAULT_HARNESS|MODEL|OWNER, empty when unset, and
// the participant agc itself posts as.
type DefaultsResponse struct {
	Harness string `json:"harness"`
	Model   string `json:"model"`
	Owner   string `json:"owner"`
	System  string `json:"system"`
}

func (api *API) GetDefaults(ctx context.Context, requester interface{}, request *GetDefaultsRequest) (*DefaultsResponse, error) {
	return &DefaultsResponse{
		Harness: api.defaults.Harness,
		Model:   api.defaults.Model,
		Owner:   api.defaults.Owner,
		System:  msgmodels.SystemName,
	}, nil
}
