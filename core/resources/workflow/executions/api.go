package executions

import (
	"github.com/uptrace/bun"
	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/runtime"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/agent-composer/worktree"
)

type API struct {
	db          bun.IDB
	rt          *runtime.Runtime
	worktrees   *worktree.Manager
	newRecorder func() workflowruntime.ExecutionRecorder
}

func NewAPI(ctrl *controller.Controller, rt *runtime.Runtime, worktrees *worktree.Manager) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}
	if rt == nil {
		panic("Runtime reference is nil")
	}

	return &API{
		db:        ctrl.DB,
		rt:        rt,
		worktrees: worktrees,
		newRecorder: func() workflowruntime.ExecutionRecorder {
			return workflowruntime.NewDBRecorder(ctrl.DB)
		},
	}
}
