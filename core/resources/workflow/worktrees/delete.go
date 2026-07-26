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
	const op = "workflow.worktrees.DeleteRequest.Validate"

	if strings.TrimSpace(r.Repo) == "" || strings.TrimSpace(r.Path) == "" {
		return ez.New(op, ez.EINVALID, "repo and path are required", nil)
	}

	return nil
}

type DeleteResponse struct {
	Removed string `json:"removed"`
}

func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*DeleteResponse, error) {
	const op = "workflow.worktrees.API.Delete"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	repo, isGit, err := api.manager.RepoRoot(ctx, request.Repo)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}
	if !isGit {
		return nil, ez.New(op, ez.EINVALID, request.Repo+" is not a git repository", nil)
	}

	err = api.manager.Remove(ctx, repo, request.Path, request.Force)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &DeleteResponse{Removed: request.Path}, nil
}
