package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vanclief/ez"
)

func loadGood(t *testing.T) *Set {
	t.Helper()

	set, err := Load("testdata/good")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	return set
}

func TestLoadGood(t *testing.T) {
	set := loadGood(t)

	if len(set.Beliefs) != 5 || len(set.Decisions) != 2 || len(set.Rulesets) != 1 {
		t.Fatalf("loaded %d beliefs, %d decisions, %d rulesets", len(set.Beliefs), len(set.Decisions), len(set.Rulesets))
	}

	first := set.Beliefs[0]
	if first.File != filepath.Join("testdata", "good", "beliefs.yaml") {
		t.Fatalf("file = %q", first.File)
	}
	if first.Observed == nil || first.Observed.When != "2026-11-01" {
		t.Fatalf("observed = %+v, want when 2026-11-01", first.Observed)
	}

	ruleset := set.Rulesets[0]
	if ruleset.Subject != "testing" || len(ruleset.Rules) != 4 {
		t.Fatalf("ruleset = %+v", ruleset)
	}
}

// writeFiles puts each content in a temp dir under its name.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func TestLoadRejects(t *testing.T) {
	observed := "    observed: { who: franco, when: 2026-11-01 }\n"
	decided := "    decided: { who: franco, when: 2026-11-01 }\n"

	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name: "duplicate id across files",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: same\n    belief: One.\n" + observed,
				"b.yaml": "subject: s\nrules:\n  - id: same\n    rule: Two.\n",
			},
			want: "b.yaml: rule same: id is already used in",
		},
		{
			name: "dangling belief because",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n    because: [missing]\n",
			},
			want: "belief a: because names missing",
		},
		{
			name: "belief resting on a decision",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n    because: [d]\n  - id: b\n    belief: Two.\n" + observed,
				"d.yaml": "decisions:\n  - id: d\n    decision: D.\n    because: [b]\n" + decided,
			},
			want: "belief a: because names d, which is not a loaded belief",
		},
		{
			name: "belief cycle",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n    because: [b]\n  - id: b\n    belief: Two.\n    because: [a]\n",
			},
			want: "belief a: because forms a cycle: a -> b -> a",
		},
		{
			name: "belief with neither observed nor because",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n",
			},
			want: "a.yaml: belief a: needs observed: or because:",
		},
		{
			name: "dangling decision because",
			files: map[string]string{
				"d.yaml": "decisions:\n  - id: d\n    decision: D.\n    because: [missing]\n" + decided,
			},
			want: "decision d: because names missing",
		},
		{
			name: "decision cycle",
			files: map[string]string{
				"d.yaml": "decisions:\n  - id: d\n    decision: D.\n    because: [e]\n" + decided +
					"  - id: e\n    decision: E.\n    because: [d]\n" + decided,
			},
			want: "decision d: because forms a cycle: d -> e -> d",
		},
		{
			name: "decision without decided",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n" + observed,
				"d.yaml": "decisions:\n  - id: d\n    decision: D.\n    because: [a]\n",
			},
			want: "d.yaml: decision d: decided: needs who and when",
		},
		{
			name: "dangling rule because",
			files: map[string]string{
				"r.yaml": "subject: s\nrules:\n  - id: r\n    rule: R.\n    because: [missing]\n",
			},
			want: "r.yaml: rule r: because names missing",
		},
		{
			name: "id that is not a slug",
			files: map[string]string{
				"r.yaml": "subject: s\nrules:\n  - id: Not_A_Slug\n    rule: R.\n",
			},
			want: "rule Not_A_Slug: id must be",
		},
		{
			name: "file of no kind",
			files: map[string]string{
				"r.yaml": "rules:\n  - id: r\n    rule: R.\n",
			},
			want: "r.yaml: a file has exactly one of",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Load(writeFiles(t, test.files))
			if err == nil {
				t.Fatalf("Load succeeded, want %q", test.want)
			}
			if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), test.want) {
				t.Fatalf("error = %q (%s), want %q", ez.ErrorMessage(err), ez.ErrorCode(err), test.want)
			}
		})
	}
}

func TestWhy(t *testing.T) {
	set := loadGood(t)

	reason, err := set.Why("top")
	if err != nil {
		t.Fatal(err)
	}

	if reason.Belief.ID != "top" || len(reason.Because) != 1 {
		t.Fatalf("top = %+v", reason)
	}

	derived := reason.Because[0]
	if derived.Belief.ID != "derived" || len(derived.Because) != 2 {
		t.Fatalf("derived = %+v", derived)
	}

	for i, want := range []string{"observed-a", "observed-b"} {
		leaf := derived.Because[i]
		if leaf.Belief.ID != want || leaf.Belief.Observed == nil || len(leaf.Because) != 0 {
			t.Fatalf("leaf %d = %+v, want observed %s", i, leaf, want)
		}
	}

	_, err = set.Why("choice")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("Why of a decision: err = %v, want not found", err)
	}
}

// ids lists the id of every element of a slice of beliefs, decisions or
// rules.
func ids(items any) []string {
	result := []string{}
	list := reflect.ValueOf(items)
	for i := 0; i < list.Len(); i++ {
		result = append(result, list.Index(i).FieldByName("ID").String())
	}

	return result
}

func TestDoubt(t *testing.T) {
	set := loadGood(t)

	doubted, err := set.Doubt("observed-a")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ids(doubted.Beliefs), []string{"derived", "top"}) {
		t.Fatalf("beliefs = %v", ids(doubted.Beliefs))
	}
	if !reflect.DeepEqual(ids(doubted.Decisions), []string{"choice", "later-choice"}) {
		t.Fatalf("decisions = %v", ids(doubted.Decisions))
	}
	if !reflect.DeepEqual(ids(doubted.Rules), []string{"passes", "manual-one"}) {
		t.Fatalf("rules = %v", ids(doubted.Rules))
	}

	doubted, err = set.Doubt("unrelated")
	if err != nil {
		t.Fatal(err)
	}
	if len(doubted.Beliefs) != 0 || len(doubted.Decisions) != 0 || !reflect.DeepEqual(ids(doubted.Rules), []string{"unrelated-rule"}) {
		t.Fatalf("unrelated = %+v", doubted)
	}

	_, err = set.Doubt("missing")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("err = %v, want not found", err)
	}
}

func TestCheck(t *testing.T) {
	set := loadGood(t)

	results, err := set.Check(context.Background(), "testing", Shell)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("got %d results", len(results))
	}

	passes := results[0]
	if passes.Kind != Mechanical || passes.Passed == nil || !*passes.Passed || passes.Output != "fine" {
		t.Fatalf("passes = %+v", passes)
	}

	fails := results[1]
	if fails.Kind != Mechanical || fails.Passed == nil || *fails.Passed || !strings.Contains(fails.Output, "broken") {
		t.Fatalf("fails = %+v", fails)
	}

	for _, manual := range results[2:] {
		if manual.Kind != Manual || manual.Passed != nil {
			t.Fatalf("manual = %+v", manual)
		}
	}

	_, err = set.Check(context.Background(), filepath.Join("testdata", "good", "rules.yaml"), Shell)
	if err != nil {
		t.Fatalf("Check by file: %v", err)
	}

	_, err = set.Check(context.Background(), "nothing", Shell)
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("err = %v, want not found", err)
	}
}
