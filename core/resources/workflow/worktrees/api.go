package worktrees

import (
	"github.com/vanclief/agent-composer/worktree"
)

type API struct {
	manager *worktree.Manager
}

func NewAPI(manager *worktree.Manager) *API {
	if manager == nil {
		panic("Worktree manager reference is nil")
	}

	return &API{manager: manager}
}
