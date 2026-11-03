package agents

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The agent view carries the template's sandbox.skills, read on every
// request, and [] when the template names none.
func TestAgentViewCarriesTemplateSkills(t *testing.T) {
	env := newTestEnv(t)

	path := filepath.Join(env.api.dir, "designer.yaml")
	err := os.WriteFile(path, []byte("harness: claude_code\n"+testSandbox+"  skills: [rtsdesign]\ninstructions: Design.\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	designer := env.create(t, "designer")
	if !reflect.DeepEqual(designer.Skills, []string{"rtsdesign"}) {
		t.Fatalf("created view skills = %q", designer.Skills)
	}

	reviewer := env.create(t, "reviewer")
	if reviewer.Skills == nil || len(reviewer.Skills) != 0 {
		t.Fatalf("reviewer skills = %#v, want []", reviewer.Skills)
	}

	err = os.WriteFile(path, []byte("harness: claude_code\n"+testSandbox+"  skills: [rtsdesign, code-review]\ninstructions: Design.\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	got := env.get(t, designer.Name)
	if !reflect.DeepEqual(got.Skills, []string{"rtsdesign", "code-review"}) {
		t.Fatalf("view after a template edit skills = %q", got.Skills)
	}

	list, err := env.api.List(context.Background(), nil, &ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range list.Agents {
		if view.Name == designer.Name && len(view.Skills) != 2 {
			t.Fatalf("list skills = %q", view.Skills)
		}
	}
}
