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

	if len(set.Evidence) != 6 || len(set.Beliefs) != 8 || len(set.Decisions) != 3 || len(set.Rulesets) != 1 {
		t.Fatalf("loaded %d evidence, %d beliefs, %d decisions, %d rulesets", len(set.Evidence), len(set.Beliefs), len(set.Decisions), len(set.Rulesets))
	}

	first := set.Beliefs[0]
	if first.File != filepath.Join("testdata", "good", "beliefs.yaml") {
		t.Fatalf("file = %q", first.File)
	}
	if !reflect.DeepEqual(first.Evidence, []string{"seen-a"}) {
		t.Fatalf("evidence = %v, want [seen-a]", first.Evidence)
	}

	seen := set.Evidence[0]
	want := Evidence{
		ID:       "seen-a",
		Evidence: "A was seen.",
		Who:      "franco",
		When:     "2026-11-01",
		Source:   "https://example.com/a",
		File:     filepath.Join("testdata", "good", "evidence.yaml"),
	}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("evidence = %+v, want %+v", seen, want)
	}

	retracted := set.Evidence[2].Retracted
	if retracted == nil || retracted.When != "2026-11-02" || retracted.Why == "" {
		t.Fatalf("retracted = %+v", retracted)
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
	decided := "    decided: { who: franco, when: 2026-11-01 }\n"

	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name: "duplicate id across files",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: same\n    belief: One.\n",
				"b.yaml": "subject: s\nrules:\n  - id: same\n    rule: Two.\n",
			},
			want: "b.yaml: rule same: id is already used in",
		},
		{
			name: "evidence id reused by a belief",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: same\n    belief: One.\n",
				"e.yaml": "evidence:\n  - id: same\n    evidence: Seen.\n    who: franco\n    when: 2026-11-01\n",
			},
			want: "a.yaml: belief same: id is already used in",
		},
		{
			name: "dangling evidence id",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n    evidence: [missing]\n",
			},
			want: "a.yaml: belief a: evidence names missing, which is not loaded evidence",
		},
		{
			name: "evidence without who",
			files: map[string]string{
				"e.yaml": "evidence:\n  - id: e\n    evidence: Seen.\n    when: 2026-11-01\n",
			},
			want: "e.yaml: evidence e: needs who and when",
		},
		{
			name: "evidence without when",
			files: map[string]string{
				"e.yaml": "evidence:\n  - id: e\n    evidence: Seen.\n    who: franco\n",
			},
			want: "e.yaml: evidence e: needs who and when",
		},
		{
			name: "evidence without text",
			files: map[string]string{
				"e.yaml": "evidence:\n  - id: e\n    who: franco\n    when: 2026-11-01\n",
			},
			want: "e.yaml: evidence e: evidence: text is required",
		},
		{
			name: "retraction without why",
			files: map[string]string{
				"e.yaml": "evidence:\n  - id: e\n    evidence: Seen.\n    who: franco\n    when: 2026-11-01\n" +
					"    retracted: { who: franco, when: 2026-11-02 }\n",
			},
			want: "e.yaml: evidence e: retracted: needs who, when and why",
		},
		{
			name: "belief still using observed",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n    observed: { who: franco, when: 2026-11-01 }\n",
			},
			want: "a.yaml: belief a: observed: is no longer a belief field",
		},
		{
			name: "belief resting on evidence",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n    because: [e]\n",
				"e.yaml": "evidence:\n  - id: e\n    evidence: Seen.\n    who: franco\n    when: 2026-11-01\n",
			},
			want: "belief a: because names e, which is not a loaded belief",
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
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n    because: [d]\n  - id: b\n    belief: Two.\n",
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
			name: "belief without text",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    evidence: []\n",
			},
			want: "a.yaml: belief a: belief: text is required",
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
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n",
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
		{
			name: "file of two kinds",
			files: map[string]string{
				"e.yaml": "evidence: []\nbeliefs: []\n",
			},
			want: "e.yaml: a file has exactly one of",
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

	for i, want := range []string{"seen-a", "seen-b"} {
		leaf := derived.Because[i]
		if len(leaf.Evidence) != 1 || leaf.Evidence[0].ID != want || leaf.Evidence[0].Who != "franco" || len(leaf.Because) != 0 {
			t.Fatalf("leaf %d = %+v, want evidence %s", i, leaf, want)
		}
	}

	reason, err = set.Why("unrelated")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids(reason.Evidence), []string{"seen-e-first", "seen-e-again"}) || reason.Evidence[0].Retracted == nil {
		t.Fatalf("unrelated evidence = %+v", reason.Evidence)
	}

	reason, err = set.Why("hunch")
	if err != nil {
		t.Fatal(err)
	}
	if len(reason.Evidence) != 0 || len(reason.Because) != 0 {
		t.Fatalf("hunch = %+v, want no evidence and no because", reason)
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

func TestDoubtEvidence(t *testing.T) {
	set := loadGood(t)

	doubted, err := set.Doubt("seen-a")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ids(doubted.Beliefs), []string{"observed-a", "derived", "top"}) {
		t.Fatalf("beliefs = %v", ids(doubted.Beliefs))
	}
	if !reflect.DeepEqual(ids(doubted.Decisions), []string{"choice", "later-choice"}) {
		t.Fatalf("decisions = %v", ids(doubted.Decisions))
	}
	if !reflect.DeepEqual(ids(doubted.Rules), []string{"passes", "manual-one"}) {
		t.Fatalf("rules = %v", ids(doubted.Rules))
	}

	doubted, err = set.Doubt("unused")
	if err != nil {
		t.Fatal(err)
	}
	if len(doubted.Beliefs) != 0 || len(doubted.Decisions) != 0 || len(doubted.Rules) != 0 {
		t.Fatalf("unused = %+v, want nothing", doubted)
	}

	if !reflect.DeepEqual(ids(set.Citing("seen-e-again")), []string{"unrelated"}) {
		t.Fatalf("citing = %v", ids(set.Citing("seen-e-again")))
	}
}

func TestStatus(t *testing.T) {
	set := loadGood(t)

	tests := []struct {
		id   string
		want Status
	}{
		{id: "top", want: Status{Label: Grounded}},
		{id: "observed-a", want: Status{Label: Grounded}},
		// One retracted and one live piece on the same leaf.
		{id: "unrelated", want: Status{Label: Grounded}},
		{id: "hunch", want: Status{Label: Opinion, Opinions: []string{"hunch"}}},
		{id: "retracted-only", want: Status{Label: InDoubt, Doubted: []string{"retracted-only"}, Retracted: []string{"seen-g"}}},
		{
			id:   "mixed",
			want: Status{Label: InDoubt, Opinions: []string{"hunch"}, Doubted: []string{"retracted-only"}, Retracted: []string{"seen-g"}},
		},
	}

	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			status, err := set.Status(test.id)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(status, test.want) {
				t.Fatalf("status = %+v, want %+v", status, test.want)
			}
		})
	}

	_, err := set.Status("choice")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("Status of a decision: err = %v, want not found", err)
	}
}

func TestStatusOfDecisionsAndRules(t *testing.T) {
	set := loadGood(t)

	because := map[string][]string{}
	for _, decision := range set.Decisions {
		because[decision.ID] = decision.Because
	}

	status := set.StatusOf(because["doubted-choice"])
	if status.Label != InDoubt || !reflect.DeepEqual(status.Doubted, []string{"retracted-only"}) {
		t.Fatalf("doubted-choice = %+v, want in doubt on retracted-only", status)
	}

	// later-choice rests on choice, which rests on evidenced beliefs.
	status = set.StatusOf(because["later-choice"])
	if status.Label != Grounded || len(status.Opinions) != 0 || len(status.Doubted) != 0 {
		t.Fatalf("later-choice = %+v, want grounded", status)
	}

	status = set.StatusOf([]string{"later-choice", "hunch"})
	if status.Label != Opinion || !reflect.DeepEqual(status.Opinions, []string{"hunch"}) {
		t.Fatalf("rule over later-choice and hunch = %+v, want opinion on hunch", status)
	}

	status = set.StatusOf(nil)
	if status.Label != Grounded {
		t.Fatalf("empty because = %+v, want grounded", status)
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
