package conversations

import (
	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/runtime"
	"github.com/vanclief/compose/drivers/databases/relational"
)

type API struct {
	db *relational.DB
	rt *runtime.Runtime
}

func NewAPI(ctrl *controller.Controller, rt *runtime.Runtime) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	} else if rt == nil {
		panic("Runtime reference is nil")
	}

	api := &API{
		db: ctrl.DB,
		rt: rt,
	}

	return api
}
