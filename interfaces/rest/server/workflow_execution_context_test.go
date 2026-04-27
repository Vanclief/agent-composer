package server

import (
	"context"
	"testing"
)

type workflowContextKey string

func TestWorkflowExecutionStartContextUsesRootContext(t *testing.T) {
	rootCtx := context.WithValue(context.Background(), workflowContextKey("root_id"), "test-root")
	rootCtx, cancel := context.WithCancel(rootCtx)

	server := &Server{
		RootContext: rootCtx,
	}

	startCtx := server.workflowExecutionStartContext()

	if startCtx.Err() != nil {
		t.Fatalf("expected start context to be active, got %v", startCtx.Err())
	}

	_, hasDeadline := startCtx.Deadline()
	if hasDeadline {
		t.Fatal("expected start context to avoid request deadlines")
	}

	value := startCtx.Value(workflowContextKey("root_id"))
	if value != "test-root" {
		t.Fatalf("expected root value to be preserved, got %#v", value)
	}

	cancel()

	if startCtx.Err() != context.Canceled {
		t.Fatalf("expected root cancellation to reach start context, got %v", startCtx.Err())
	}
}
