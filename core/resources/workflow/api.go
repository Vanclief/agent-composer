package workflow

import (
	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/workflow/executions"
	"github.com/vanclief/agent-composer/core/resources/workflow/nodeexecutions"
	"github.com/vanclief/agent-composer/runtime"
)

type API struct {
	Executions     *executions.API
	NodeExecutions *nodeexecutions.API
}

func NewAPI(ctrl *controller.Controller, rt *runtime.Runtime) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	executionsAPI := executions.NewAPI(ctrl, rt)
	nodeExecutionsAPI := nodeexecutions.NewAPI(ctrl)

	return &API{
		Executions:     executionsAPI,
		NodeExecutions: nodeExecutionsAPI,
	}
}
