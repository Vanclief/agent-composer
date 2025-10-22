package models

import (
	"os/user"

	"github.com/vanclief/agent-composer/models/hook"
	"github.com/vanclief/agent-composer/models/parrot"
)

var REGISTRABLE = []interface{}{}

var ALL = []interface{}{
	(*hook.Hook)(nil),
	(*parrot.Run)(nil),
	(*parrot.Template)(nil),
	(*user.User)(nil),
}
