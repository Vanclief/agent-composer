package agents

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/knowledge"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
)

const testRules = `subject: go code
rules:
  - id: go-no-explicit-semicolons
    rule: Never write an explicit ";" in Go outside a 3-clause "for" header.
  - id: go-one-statement-per-line
    rule: Write one Go statement per line.
`

// withRules writes a knowledge dir holding testRules and the ruled
// template naming rules, and wires it into env.
func withRules(t *testing.T, env *testEnv, rules string) {
	t.Helper()

	knowledgeDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(knowledgeDir, "rules"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(knowledgeDir, "rules", "go.yaml"), []byte(testRules), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	body := "harness: claude_code\nreads: []\nrules: " + rules + "\n" + testSandbox + "instructions: |\n  Review.\n"
	err = os.WriteFile(filepath.Join(env.api.dir, "ruled.yaml"), []byte(body), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	env.api.SetRules(knowledge.NewAPI(knowledgeDir))
}

func TestTemplateRulesEndAgentMD(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	withRules(t, env, "[go-no-explicit-semicolons]")

	created := env.create(t, "ruled")
	wantRules := []AgentRule{{ID: "go-no-explicit-semicolons", Rule: `Never write an explicit ";" in Go outside a 3-clause "for" header.`}}
	if !reflect.DeepEqual(created.Rules, wantRules) {
		t.Fatalf("rules = %+v, want %+v", created.Rules, wantRules)
	}

	env.wake(t, created.Name)

	list, err := env.sbx.List(ctx, nil, &sandboxes.ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Sandboxes) != 1 {
		t.Fatalf("sandboxes = %+v", list.Sandboxes)
	}

	want := "Review.\n\n" +
		"## Rules you follow\n\n" +
		"- Never write an explicit \";\" in Go outside a 3-clause \"for\" header. (go-no-explicit-semicolons)\n\n" +
		"Check your work against these before you answer or deliver; `agc rules show` prints every rule of the repository.\n"
	if list.Sandboxes[0].Instructions != want {
		t.Fatalf("AGENT.md = %q, want %q", list.Sandboxes[0].Instructions, want)
	}
}

func TestTemplateWithoutRulesKeepsInstructions(t *testing.T) {
	tmpl := Template{Instructions: "Review.\n"}
	if tmpl.AgentMD() != "Review.\n" {
		t.Fatalf("AGENT.md = %q", tmpl.AgentMD())
	}
}

func TestUnknownRuleFailsLoadTemplate(t *testing.T) {
	env := newTestEnv(t)
	withRules(t, env, "[go-no-explicit-semicolons, go-no-tabs]")

	_, err := LoadTemplate(env.api.dir, "ruled", env.api.rules)
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("err = %v, want EINVALID", err)
	}
	msg := ez.ErrorMessage(err)
	if !strings.Contains(msg, `"go-no-tabs"`) || !strings.Contains(msg, "go-no-explicit-semicolons, go-one-statement-per-line") {
		t.Fatalf("message = %q, want the unknown id and the known ones", msg)
	}

	_, err = env.api.Create(context.Background(), nil, &CreateRequest{Role: "ruled"})
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("create err = %v, want EINVALID", err)
	}
}

func TestRulesNeedAKnowledgeDir(t *testing.T) {
	env := newTestEnv(t)
	withRules(t, env, "[go-no-explicit-semicolons]")

	_, err := LoadTemplate(env.api.dir, "ruled", nil)
	if err == nil {
		t.Fatal("a template naming rules loaded with no knowledge directory")
	}
}
