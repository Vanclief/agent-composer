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
	registry    *workflowruntime.Registry
	newRecorder func() workflowruntime.ExecutionRecorder
}

func NewAPI(ctrl *controller.Controller, rt *runtime.Runtime, worktrees *worktree.Manager, registry *workflowruntime.Registry) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}
	if rt == nil {
		panic("Runtime reference is nil")
	}
	if registry == nil {
		panic("Registry reference is nil")
	}

	return &API{
		db:        ctrl.DB,
		rt:        rt,
		worktrees: worktrees,
		registry:  registry,
		newRecorder: func() workflowruntime.ExecutionRecorder {
			return workflowruntime.NewDBRecorder(ctrl.DB)
		},
	}
}
