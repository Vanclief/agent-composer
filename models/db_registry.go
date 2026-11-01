package models

import (
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/agent-composer/models/hook"
	"github.com/vanclief/agent-composer/models/messaging"
	"github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/models/user"
	"github.com/vanclief/agent-composer/models/workflow"
)

var REGISTRABLE = []interface{}{}

var ALL = []interface{}{
	(*hook.Hook)(nil),
	(*messaging.Participant)(nil), (*messaging.Message)(nil),
	(*agent.Conversation)(nil),
	(*agent.Spec)(nil),
	(*execution.WorkflowExecution)(nil),
	(*execution.NodeExecution)(nil),
	(*task.Task)(nil),
	(*user.User)(nil),
	(*workflow.Workflow)(nil),
	(*workflow.WorkflowVersion)(nil),
}
