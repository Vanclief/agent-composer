package runs

import (
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/agent-composer/server/controller"
)

type API struct {
	db *relational.DB
}

func NewAPI(ctrl *controller.Controller) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	api := &API{
		db: ctrl.DB,
	}

	return api
}
