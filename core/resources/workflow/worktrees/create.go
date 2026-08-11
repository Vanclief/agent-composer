package worktrees

import (
	"context"
	"strings"

	"github.com/vanclief/ez"
)

type CreateRequest struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
	Base   string `json:"base,omitempty"`
}

func (r *CreateRequest) Validate() error {
	if strings.TrimSpace(r.Repo) == "" || strings.TrimSpace(r.Branch) == "" {
		return ez.New(ez.EINVALID, "repo and branch are required", nil)
	}

	return nil
}

type CreateResponse struct {
	Path    string `json:"path"`
	Branch  string `json:"branch"`
	Created bool   `json:"created"`
}

func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*CreateResponse, error) {
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

	branch := strings.TrimSpace(request.Branch)
	path, created, err := api.manager.Resolve(ctx, repo, branch, request.Base)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &CreateResponse{
		Path:    path,
		Branch:  branch,
		Created: created,
	}, nil
}
