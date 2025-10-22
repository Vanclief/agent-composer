package parrots

import (
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/agent-composer/runtime/orchestra"
	"github.com/vanclief/agent-composer/server/controller"
	"github.com/vanclief/agent-composer/server/resources/parrots/runs"
	"github.com/vanclief/agent-composer/server/resources/parrots/templates"
)

type API struct {
	db        *relational.DB
	Runs      *runs.API
	Templates *templates.API
}

func NewAPI(ctrl *controller.Controller, orchestrator *orchestra.Orchestrator) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	runs := runs.NewAPI(ctrl)
	templates := templates.NewAPI(ctrl, orchestrator)

	api := &API{
		db:        ctrl.DB,
		Runs:      runs,
		Templates: templates,
	}

	return api
}
