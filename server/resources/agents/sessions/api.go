package sessions

import (
	"github.com/vanclief/agent-composer/server/controller"
	"github.com/vanclief/compose/drivers/databases/relational"
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
