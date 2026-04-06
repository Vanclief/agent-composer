package executions

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/agent-composer/models/agent"
	executionmodels "github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/ez"
)

type ExecutionFailureDetails struct {
	ExecutionID     string
	WorkflowID      string
	WorkflowVersion string
	NodeExecutionID string
	NodeID          string
	NodeError       string
	ConversationID  string
	HarnessExitCode int
	HarnessError    string
}

type ExecutionFailedError struct {
	Details ExecutionFailureDetails
	Err     error
}

func (e *ExecutionFailedError) Error() string {
	if e == nil {
		return "workflow execution failed"
	}

	fields := []string{}

	if strings.TrimSpace(e.Details.ExecutionID) != "" {
		fields = append(fields, "execution_id="+e.Details.ExecutionID)
	}
	if strings.TrimSpace(e.Details.WorkflowID) != "" {
		fields = append(fields, "workflow_id="+e.Details.WorkflowID)
	}
	if strings.TrimSpace(e.Details.NodeID) != "" {
		fields = append(fields, "node_id="+e.Details.NodeID)
	}
	if strings.TrimSpace(e.Details.NodeExecutionID) != "" {
		fields = append(fields, "node_execution_id="+e.Details.NodeExecutionID)
	}
	if strings.TrimSpace(e.Details.ConversationID) != "" {
		fields = append(fields, "conversation_id="+e.Details.ConversationID)
	}
	if e.Details.HarnessExitCode != 0 {
		fields = append(fields, "harness_exit_code="+strconv.Itoa(e.Details.HarnessExitCode))
	}
	if strings.TrimSpace(e.Details.HarnessError) != "" {
		fields = append(fields, "harness_error="+strconv.Quote(e.Details.HarnessError))
	} else if strings.TrimSpace(e.Details.NodeError) != "" {
		fields = append(fields, "node_error="+strconv.Quote(e.Details.NodeError))
	}

	message := "workflow execution failed"
	if len(fields) > 0 {
		message = message + " (" + strings.Join(fields, ", ") + ")"
	}

	if e.Err != nil {
		message = message + ": " + e.Err.Error()
	}

	return message
}

func (e *ExecutionFailedError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

func loadExecutionFailureDetails(ctx context.Context, db bun.IDB, details ExecutionFailureDetails) (ExecutionFailureDetails, error) {
	const op = "workflow.executions.loadExecutionFailureDetails"

	if db == nil {
		return details, nil
	}

	trimmedExecutionID := strings.TrimSpace(details.ExecutionID)
	if trimmedExecutionID == "" {
		return details, nil
	}

	executionUUID, err := uuid.Parse(trimmedExecutionID)
	if err != nil {
		return details, ez.New(op, ez.EINVALID, "invalid execution id", err)
	}

	var node executionmodels.NodeExecution
	err = db.NewSelect().
		Model(&node).
		Where("workflow_execution_id = ?", executionUUID).
		Where("status = ?", executionmodels.NodeExecutionStatusFailed).
		OrderExpr("created_at DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return details, nil
		}

		return details, ez.Wrap(op, err)
	}

	details.NodeExecutionID = node.ID.String()
	details.NodeID = strings.TrimSpace(node.NodeID)

	rawNodeError, found := node.Trace["error"]
	if found {
		nodeError, ok := rawNodeError.(string)
		if ok {
			details.NodeError = strings.TrimSpace(nodeError)
		}
	}

	var conversation agent.Conversation
	err = db.NewSelect().
		Model(&conversation).
		Where("node_execution_id = ?", node.ID).
		OrderExpr("created_at DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return details, nil
		}

		return details, ez.Wrap(op, err)
	}

	details.ConversationID = conversation.ID.String()
	details.HarnessExitCode = conversation.HarnessExitCode
	details.HarnessError = strings.TrimSpace(conversation.HarnessError)

	return details, nil
}
