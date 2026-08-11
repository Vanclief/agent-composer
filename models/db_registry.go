package models

import (
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/agent-composer/models/hook"
	"github.com/vanclief/agent-composer/models/user"
)

var REGISTRABLE = []interface{}{}

var ALL = []interface{}{
	(*hook.Hook)(nil),
	(*agent.Conversation)(nil),
	(*agent.Spec)(nil),
	(*execution.WorkflowExecution)(nil),
	(*execution.NodeExecution)(nil),
	(*user.User)(nil),
}
