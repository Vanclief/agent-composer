package worktrees

import (
	"context"
	"os"
	"strings"

	"github.com/vanclief/agent-composer/worktree"
	"github.com/vanclief/ez"
)

type ListRequest struct {
	Repo string `json:"repo"`
	// Fetch refreshes origin refs first, so remote branches are current.
	Fetch bool `json:"fetch,omitempty"`
}

func (r *ListRequest) Validate() error {
	const op = "workflow.worktrees.ListRequest.Validate"

	if strings.TrimSpace(r.Repo) == "" {
		return ez.New(op, ez.EINVALID, "repo is required", nil)
	}

	return nil
}

type ListResponse struct {
	// Exists reports whether the path is a directory at all — the UI
	// uses it to validate a project path before adding it.
	Exists    bool              `json:"exists"`
	IsGit     bool              `json:"is_git"`
	Repo      string            `json:"repo,omitempty"`
	Worktrees []worktree.Info   `json:"worktrees"`
	Branches  []worktree.Branch `json:"branches,omitempty"`
}

func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	const op = "workflow.worktrees.API.List"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	info, statErr := os.Stat(strings.TrimSpace(request.Repo))
	exists := statErr == nil && info.IsDir()

	repo, isGit, err := api.manager.RepoRoot(ctx, request.Repo)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}
	if !isGit {
		// Not an error: the UI hides the workspace picker for plain dirs.
		return &ListResponse{
			Exists:    exists,
			Worktrees: []worktree.Info{},
		}, nil
	}

	if request.Fetch {
		// Explicitly requested — a failure (e.g. no remote) surfaces.
		err = api.manager.Fetch(ctx, repo)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	}

	// Manually deleted directories leave stale entries behind.
	_ = api.manager.Prune(ctx, repo)

	infos, err := api.manager.List(ctx, repo)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	branches, err := api.manager.Branches(ctx, repo)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &ListResponse{
		Exists:    true,
		IsGit:     true,
		Repo:      repo,
		Worktrees: infos,
		Branches:  branches,
	}, nil
}
