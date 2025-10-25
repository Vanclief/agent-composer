package models

import (
	"os/user"

	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/models/hook"
)

var REGISTRABLE = []interface{}{}

var ALL = []interface{}{
	(*hook.Hook)(nil),
	(*agent.Conversation)(nil),
	(*agent.Spec)(nil),
	(*user.User)(nil),
}
