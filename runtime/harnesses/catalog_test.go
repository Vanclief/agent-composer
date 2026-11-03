package harnesses

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/vanclief/agent-composer/models/agent"
)

func TestListHarnessReads(t *testing.T) {
	reads := ListHarnessReads()

	ids := []agent.Harness{}
	byID := map[agent.Harness][]string{}
	for _, harness := range reads {
		ids = append(ids, harness.ID)
		byID[harness.ID] = harness.Reads
	}

	wantIDs := []agent.Harness{agent.HarnessCodexCLI, agent.HarnessClaudeCode, agent.HarnessPi}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("ids = %v, want %v", ids, wantIDs)
	}

	if !reflect.DeepEqual(byID[agent.HarnessClaudeCode], []string{"CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_API_KEY"}) {
		t.Fatalf("claude_code reads %v", byID[agent.HarnessClaudeCode])
	}
	if !reflect.DeepEqual(byID[agent.HarnessCodexCLI], []string{"CODEX_API_KEY"}) {
		t.Fatalf("codex_cli reads %v", byID[agent.HarnessCodexCLI])
	}

	envRe := regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	for _, harness := range reads {
		if harness.Reads == nil {
			t.Fatalf("%s: reads is nil, want a list (JSON [])", harness.ID)
		}
		for _, name := range harness.Reads {
			if !envRe.MatchString(name) {
				t.Fatalf("%s reads %q, not a variable name", harness.ID, name)
			}
		}
	}

	reads[1].Reads[0] = "MUTATED"
	if ListHarnessReads()[1].Reads[0] != "CLAUDE_CODE_OAUTH_TOKEN" {
		t.Fatal("ListHarnessReads returned a slice aliasing the catalog")
	}
}
