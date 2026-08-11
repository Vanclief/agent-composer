package worktrees

import (
	"context"
	"strings"

	"github.com/vanclief/ez"
)

type DeleteRequest struct {
	Repo  string `json:"repo"`
	Path  string `json:"path"`
	Force bool   `json:"force,omitempty"`
}

func (r *DeleteRequest) Validate() error {
	if strings.TrimSpace(r.Repo) == "" || strings.TrimSpace(r.Path) == "" {
		return ez.New(ez.EINVALID, "repo and path are required", nil)
	}

	return nil
}

type DeleteResponse struct {
	Removed string `json:"removed"`
}

func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*DeleteResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	repo, isGit, err := api.manager.RepoRoot(ctx, request.Repo)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if !isGit {
		return nil, ez.New(ez.EINVALID, request.Repo+" is not a git repository", nil)
	}

	err = api.manager.Remove(ctx, repo, request.Path, request.Force)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &DeleteResponse{Removed: request.Path}, nil
}
