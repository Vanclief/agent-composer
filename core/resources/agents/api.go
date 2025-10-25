package agents

import (
	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/agents/conversations"
	"github.com/vanclief/agent-composer/core/resources/agents/specs"
	"github.com/vanclief/agent-composer/runtime"
	"github.com/vanclief/compose/drivers/databases/relational"
)

type API struct {
	db            *relational.DB
	Conversations *conversations.API
	AgentSpecs    *specs.API
}

func NewAPI(ctrl *controller.Controller, rt *runtime.Runtime) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	conversationsAPI := conversations.NewAPI(ctrl, rt)
	agentSpecs := specs.NewAPI(ctrl, rt)

	api := &API{
		db:            ctrl.DB,
		Conversations: conversationsAPI,
		AgentSpecs:    agentSpecs,
	}

	return api
}
