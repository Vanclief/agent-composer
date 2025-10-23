package agents

import (
	"github.com/vanclief/agent-composer/runtime"
	"github.com/vanclief/agent-composer/server/controller"
	"github.com/vanclief/agent-composer/server/resources/agents/sessions"
	"github.com/vanclief/agent-composer/server/resources/agents/specs"
	"github.com/vanclief/compose/drivers/databases/relational"
)

type API struct {
	db         *relational.DB
	Sessions   *sessions.API
	AgentSpecs *specs.API
}

func NewAPI(ctrl *controller.Controller, rt *runtime.Runtime) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	sessionsAPI := sessions.NewAPI(ctrl, rt)
	agentSpecs := specs.NewAPI(ctrl, rt)

	api := &API{
		db:         ctrl.DB,
		Sessions:   sessionsAPI,
		AgentSpecs: agentSpecs,
	}

	return api
}
