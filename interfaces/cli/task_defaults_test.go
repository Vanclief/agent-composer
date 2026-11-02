package cli

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/tasks"
)

func TestFillTaskDefaults(t *testing.T) {
	t.Setenv("AGC_MSG_STATE", filepath.Join(t.TempDir(), "msg.json"))
	defaults := controller.DefaultsSettings{Harness: "claude_code", Model: "claude-opus-5-5", Owner: "franco"}

	// Not joined: the configured owner manages.
	request := &tasks.CreateRequest{Title: "Fix login"}

	applied, err := fillTaskDefaults(request, defaults)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"manager franco (the configured owner)", "assignee claude_code", "model claude-opus-5-5", "owner franco"}
	if !reflect.DeepEqual(applied, want) {
		t.Fatalf("applied = %q", applied)
	}
	if request.Manager != "franco" || request.Assignee != "claude_code" || request.Model != "claude-opus-5-5" || request.Owner != "franco" {
		t.Fatalf("request = %+v", request)
	}

	// Joined as orchestrator, with the file naming a person and an owner.
	err = saveMsgState(msgState{ParticipantID: "p1", Name: "orchestrator"})
	if err != nil {
		t.Fatal(err)
	}

	request = &tasks.CreateRequest{Title: "Call the bank", Assignee: "ana", Owner: "ana"}

	applied, err = fillTaskDefaults(request, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(applied, []string{"manager orchestrator (your messaging name)"}) || request.Model != "" {
		t.Fatalf("applied = %q, request = %+v", applied, request)
	}

	// No owner configured and none joined: the manager is missing.
	t.Setenv("AGC_MSG_STATE", filepath.Join(t.TempDir(), "none.json"))

	_, err = fillTaskDefaults(&tasks.CreateRequest{Title: "x"}, controller.DefaultsSettings{Harness: "claude_code"})
	if err == nil {
		t.Fatal("expected an error without a manager")
	}
}
