package templates

import (
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/agent-composer/runtime/orchestra"
	"github.com/vanclief/agent-composer/server/controller"
)

type API struct {
	db           *relational.DB
	orchestrator *orchestra.Orchestrator
}

func NewAPI(ctrl *controller.Controller, orchestrator *orchestra.Orchestrator) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	} else if orchestrator == nil {
		panic("Orchestrator reference is nil")
	}

	api := &API{
		db:           ctrl.DB,
		orchestrator: orchestrator,
	}

	return api
}
