package nodeexecutions

import (
	"github.com/uptrace/bun"
	"github.com/vanclief/agent-composer/core/controller"
)

type API struct {
	db bun.IDB
}

func NewAPI(ctrl *controller.Controller) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	return &API{
		db: ctrl.DB,
	}
}
