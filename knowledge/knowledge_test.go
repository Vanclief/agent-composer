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

	if len(set.Evidence) != 6 || len(set.Beliefs) != 8 || len(set.Decisions) != 3 || len(set.Rulesets) != 1 || len(set.Actions) != 3 {
		t.Fatalf("loaded %d evidence, %d beliefs, %d decisions, %d rulesets, %d actions", len(set.Evidence), len(set.Beliefs), len(set.Decisions), len(set.Rulesets), len(set.Actions))
	}

	action := set.Actions[1]
	if action.ID != "done-action" || action.Opened == nil || action.Done == nil || action.Done.When != "2026-11-03" || action.File != filepath.Join("testdata", "good", "actions.yaml") {
		t.Fatalf("action = %+v", action)
	}
	if set.Actions[0].Done != nil {
		t.Fatalf("open action has done: %+v", set.Actions[0].Done)
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
	opened := "    opened: { who: franco, when: 2026-11-01 }\n"

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
		{
			name: "evidence witnessed by a harness",
			files: map[string]string{
				"e.yaml": "evidence:\n  - id: e\n    evidence: Seen.\n    who: codex_cli\n    when: 2026-11-01\n",
			},
			want: "e.yaml: evidence e: who must be a person or a mechanical check, not a model; put the model in translated_by and the real source in source, or record this as an opinion",
		},
		{
			name: "evidence witnessed by a claude model",
			files: map[string]string{
				"e.yaml": "evidence:\n  - id: e\n    evidence: Seen.\n    who: Claude-Opus-5\n    when: 2026-11-01\n",
			},
			want: "evidence e: who must be a person or a mechanical check, not a model",
		},
		{
			name: "evidence witnessed by an agent",
			files: map[string]string{
				"e.yaml": "evidence:\n  - id: e\n    evidence: Seen.\n    who: agent\n    when: 2026-11-01\n",
			},
			want: "evidence e: who must be a person or a mechanical check, not a model",
		},
		{
			name: "translated evidence without a source",
			files: map[string]string{
				"e.yaml": "evidence:\n  - id: e\n    evidence: Seen.\n    who: franco\n    when: 2026-11-01\n    translated_by: codex-cli\n",
			},
			want: "e.yaml: evidence e: translated_by needs source",
		},
		{
			name: "action without opened",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n",
				"d.yaml": "decisions:\n  - id: d\n    decision: D.\n    because: [a]\n" + decided,
				"x.yaml": "actions:\n  - id: x\n    action: Do X.\n    because: [d]\n",
			},
			want: "x.yaml: action x: opened: needs who and when",
		},
		{
			name: "action with half a done",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n",
				"d.yaml": "decisions:\n  - id: d\n    decision: D.\n    because: [a]\n" + decided,
				"x.yaml": "actions:\n  - id: x\n    action: Do X.\n    because: [d]\n" + opened + "    done: { who: franco }\n",
			},
			want: "x.yaml: action x: done: needs who and when",
		},
		{
			name: "action without text",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n",
				"d.yaml": "decisions:\n  - id: d\n    decision: D.\n    because: [a]\n" + decided,
				"x.yaml": "actions:\n  - id: x\n    because: [d]\n" + opened,
			},
			want: "x.yaml: action x: action: text is required",
		},
		{
			name: "action without because",
			files: map[string]string{
				"x.yaml": "actions:\n  - id: x\n    action: Do X.\n" + opened,
			},
			want: "x.yaml: action x: because: is required",
		},
		{
			name: "action resting only on beliefs",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n",
				"x.yaml": "actions:\n  - id: x\n    action: Do X.\n    because: [a]\n" + opened,
			},
			want: "x.yaml: action x: because: needs at least one decision",
		},
		{
			name: "dangling action because",
			files: map[string]string{
				"x.yaml": "actions:\n  - id: x\n    action: Do X.\n    because: [missing]\n" + opened,
			},
			want: "x.yaml: action x: because names missing",
		},
		{
			name: "action id reused by a belief",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: x\n    belief: One.\n",
				"x.yaml": "actions:\n  - id: x\n    action: Do X.\n    because: [x]\n" + opened,
			},
			want: "action x: id is already used in",
		},
		{
			name: "alternative forming a cycle",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n    evidence: [e]\n    or:\n      - because: [b]\n" +
					"  - id: b\n    belief: Two.\n    because: [a]\n",
				"e.yaml": "evidence:\n  - id: e\n    evidence: Seen.\n    who: franco\n    when: 2026-11-01\n",
			},
			want: "belief a: because forms a cycle: a -> b -> a",
		},
		{
			name: "dangling alternative",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n  - id: b\n    belief: Two.\n    because: [a]\n    or:\n      - because: [missing]\n",
			},
			want: "belief b: because names missing",
		},
		{
			name: "alternative citing missing evidence",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n  - id: b\n    belief: Two.\n    because: [a]\n    or:\n      - evidence: [missing]\n",
			},
			want: "belief b: evidence names missing",
		},
		{
			name: "empty alternative",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n  - id: b\n    belief: Two.\n    because: [a]\n    or:\n      - {}\n",
			},
			want: "belief b: each or: entry needs because: or evidence:",
		},
		{
			name: "alternative to nothing",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n  - id: b\n    belief: Two.\n    or:\n      - because: [a]\n",
			},
			want: "belief b: or: is an alternative to because: or evidence:, which are empty",
		},
		{
			name: "decision alternative citing evidence",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n",
				"d.yaml": "decisions:\n  - id: d\n    decision: D.\n    because: [a]\n    or:\n      - because: [a]\n        evidence: [e]\n" + decided,
			},
			want: "decision d: only beliefs cite evidence",
		},
		{
			name: "rule alternative to nothing",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n",
				"r.yaml": "subject: s\nrules:\n  - id: r\n    rule: R.\n    or:\n      - because: [a]\n",
			},
			want: "rule r: or: is an alternative to because:, which is empty",
		},
		{
			name: "contradicting something missing",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n    contradicts: [missing]\n",
			},
			want: "a.yaml: belief a: contradicts names missing",
		},
		{
			name: "contradicting itself",
			files: map[string]string{
				"e.yaml": "evidence:\n  - id: e\n    evidence: Seen.\n    who: franco\n    when: 2026-11-01\n    contradicts: [e]\n",
			},
			want: "e.yaml: evidence e: contradicts names e",
		},
		{
			name: "contradicting a decision",
			files: map[string]string{
				"a.yaml": "beliefs:\n  - id: a\n    belief: One.\n    contradicts: [d]\n",
				"d.yaml": "decisions:\n  - id: d\n    decision: D.\n    because: [a]\n" + decided,
			},
			want: "belief a: contradicts names d, which is not another loaded belief or evidence",
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

func TestTranslatedEvidence(t *testing.T) {
	set := load(t, writeFiles(t, map[string]string{
		"e.yaml": "evidence:\n  - id: e\n    evidence: The build log shows a cgo error.\n    who: ci\n    when: 2026-11-01\n" +
			"    source: https://example.com/build/1\n    translated_by: codex-cli\n",
		"b.yaml": "beliefs:\n  - id: b\n    belief: Release builds lack cgo.\n    evidence: [e]\n",
	}))

	if set.Evidence[0].TranslatedBy != "codex-cli" {
		t.Fatalf("translated_by = %q", set.Evidence[0].TranslatedBy)
	}

	got := set.EvidenceLine(set.Evidence[0])
	want := "ci saw that the build log shows a cgo error on 2026-11-01, written up by codex-cli."
	if got.Text != want {
		t.Fatalf("evidence = %q, want %q", got.Text, want)
	}

	reason, err := set.Why("b")
	if err != nil {
		t.Fatal(err)
	}
	why := set.WhyLines(reason, 0)[0].Text
	if why != "I believe release builds lack cgo because ci saw on 2026-11-01 that the build log shows a cgo error, written up by codex-cli." {
		t.Fatalf("why = %q", why)
	}

	for _, who := range []string{"franco", "ci", "Ana", "pilot"} {
		if isModel(who) {
			t.Fatalf("isModel(%q) = true", who)
		}
	}
	for _, who := range []string{"claude_code", "codex-cli", "pi", "GPT-5.5", "gemini-3", "LLM", "model"} {
		if !isModel(who) {
			t.Fatalf("isModel(%q) = false", who)
		}
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
	// done-action rests on choice too, but it is done.
	if !reflect.DeepEqual(ids(doubted.Actions), []string{"open-action"}) || len(doubted.Standing) != 0 {
		t.Fatalf("actions = %v, standing = %v", ids(doubted.Actions), doubted.Standing)
	}

	doubted, err = set.Doubt("unrelated")
	if err != nil {
		t.Fatal(err)
	}
	if len(doubted.Beliefs) != 0 || len(doubted.Decisions) != 0 || !reflect.DeepEqual(ids(doubted.Rules), []string{"unrelated-rule"}) || len(doubted.Actions) != 0 {
		t.Fatalf("unrelated = %+v", doubted)
	}

	doubted, err = set.Doubt("retracted-only")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids(doubted.Actions), []string{"doubted-action"}) {
		t.Fatalf("retracted-only actions = %v", ids(doubted.Actions))
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

	supports := map[string][]Support{}
	for _, decision := range set.Decisions {
		supports[decision.ID] = decision.Supports()
	}

	status := set.StatusOf(supports["doubted-choice"])
	if status.Label != InDoubt || !reflect.DeepEqual(status.Doubted, []string{"retracted-only"}) {
		t.Fatalf("doubted-choice = %+v, want in doubt on retracted-only", status)
	}

	// later-choice rests on choice, which rests on evidenced beliefs.
	status = set.StatusOf(supports["later-choice"])
	if status.Label != Grounded || len(status.Opinions) != 0 || len(status.Doubted) != 0 {
		t.Fatalf("later-choice = %+v, want grounded", status)
	}

	status = set.StatusOf([]Support{{Because: []string{"later-choice", "hunch"}}})
	if status.Label != Opinion || !reflect.DeepEqual(status.Opinions, []string{"hunch"}) {
		t.Fatalf("rule over later-choice and hunch = %+v, want opinion on hunch", status)
	}

	status = set.StatusOf([]Support{{}})
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

func load(t *testing.T, paths ...string) *Set {
	t.Helper()

	set, err := Load(paths...)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	return set
}

func without(t *testing.T, set *Set, ids ...string) *Set {
	t.Helper()

	view, err := set.Without(ids)
	if err != nil {
		t.Fatalf("Without: %v", err)
	}

	return view
}

func status(t *testing.T, set *Set, id string) Status {
	t.Helper()

	found, err := set.Status(id)
	if err != nil {
		t.Fatal(err)
	}

	return found
}

func TestAlternatives(t *testing.T) {
	set := load(t, "testdata/alternatives")

	if !reflect.DeepEqual(set.Beliefs[2].Or, []Support{{Evidence: []string{"seen-y"}}}) {
		t.Fatalf("either or = %+v", set.Beliefs[2].Or)
	}

	tests := []struct {
		name    string
		without []string
		id      string
		want    Status
	}{
		{name: "both sets stand", id: "either", want: Status{Label: Grounded}},
		{name: "the alternative stands", without: []string{"seen-x"}, id: "either", want: Status{Label: Grounded}},
		{name: "the main set stands", without: []string{"seen-y"}, id: "either", want: Status{Label: Grounded}},
		{
			name:    "neither stands",
			without: []string{"seen-x", "seen-y"},
			id:      "either",
			want:    Status{Label: InDoubt, Doubted: []string{"by-x", "either"}, Without: []string{"seen-x", "seen-y"}},
		},
		{
			name:    "no alternative",
			without: []string{"seen-x"},
			id:      "only-x",
			want:    Status{Label: InDoubt, Doubted: []string{"by-x"}, Without: []string{"seen-x"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := status(t, without(t, set, test.without...), test.id)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("status = %+v, want %+v", got, test.want)
			}
		})
	}

	// A decision whose own reasons fall stands on its alternative.
	decision := set.Decisions[1]
	got := without(t, set, "seen-x").StatusOf(decision.Supports())
	if decision.ID != "choose-x" || got.Label != Grounded {
		t.Fatalf("%s without seen-x = %+v, want grounded", decision.ID, got)
	}

	reason, err := set.Why("either")
	if err != nil {
		t.Fatal(err)
	}
	if len(reason.Because) != 1 || reason.Because[0].Belief.ID != "by-x" || len(reason.Or) != 1 || ids(reason.Or[0].Evidence)[0] != "seen-y" {
		t.Fatalf("why either = %+v", reason)
	}
}

func TestDoubtStillStanding(t *testing.T) {
	set := load(t, "testdata/alternatives")

	doubted, err := set.Doubt("seen-x")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ids(doubted.Beliefs), []string{"by-x", "only-x"}) {
		t.Fatalf("beliefs = %v", ids(doubted.Beliefs))
	}
	// choose-either rests on either, which still stands, so it is not
	// listed at all.
	if !reflect.DeepEqual(ids(doubted.Decisions), []string{"decide-x"}) {
		t.Fatalf("decisions = %v", ids(doubted.Decisions))
	}
	if !reflect.DeepEqual(ids(doubted.Rules), []string{"rule-x"}) {
		t.Fatalf("rules = %v", ids(doubted.Rules))
	}
	// act-done rests on decide-x too, but it is done.
	if !reflect.DeepEqual(ids(doubted.Actions), []string{"act-falls"}) {
		t.Fatalf("actions = %v", ids(doubted.Actions))
	}

	want := []Standing{
		{Kind: "belief", ID: "either", Text: "W holds.", File: filepath.Join("testdata", "alternatives", "beliefs.yaml"), Other: []Support{{Evidence: []string{"seen-y"}}}},
		{Kind: "decision", ID: "choose-x", Text: "We chose V.", File: filepath.Join("testdata", "alternatives", "decisions.yaml"), Other: []Support{{Because: []string{"by-y"}}}},
		{Kind: "rule", ID: "rule-or", Text: "Must follow V or Y.", File: filepath.Join("testdata", "alternatives", "rules.yaml"), Other: []Support{{Because: []string{"by-y"}}}},
		{Kind: "action", ID: "act-stands", Text: "Do V or W.", File: filepath.Join("testdata", "alternatives", "actions.yaml"), Other: []Support{{Because: []string{"choose-either"}}}},
	}
	if !reflect.DeepEqual(doubted.Standing, want) {
		t.Fatalf("standing = %+v\nwant %+v", doubted.Standing, want)
	}

	if !reflect.DeepEqual(ids(set.Citing("seen-y")), []string{"by-y", "either"}) {
		t.Fatalf("citing seen-y = %v", ids(set.Citing("seen-y")))
	}
}

func TestWithout(t *testing.T) {
	set := loadGood(t)

	view := without(t, set, "seen-a")
	got := status(t, view, "observed-a")
	want := Status{Label: InDoubt, Doubted: []string{"observed-a"}, Without: []string{"seen-a"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("observed-a without seen-a = %+v, want %+v", got, want)
	}
	if status(t, view, "top").Label != InDoubt {
		t.Fatalf("top without seen-a = %+v, want in doubt", status(t, view, "top"))
	}

	// Nothing is changed in the set itself.
	if status(t, set, "observed-a").Label != Grounded {
		t.Fatalf("observed-a after Without = %+v, want grounded", status(t, set, "observed-a"))
	}

	// One of two live pieces set aside leaves the other.
	if status(t, without(t, set, "seen-e-again"), "unrelated").Label != InDoubt {
		t.Fatal("unrelated without its only live evidence is not in doubt")
	}

	view = without(t, set, "derived")
	got = status(t, view, "top")
	want = Status{Label: InDoubt, Doubted: []string{"derived"}, Without: []string{"derived"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("top without derived = %+v, want %+v", got, want)
	}

	_, err := set.Without([]string{"choice"})
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("Without a decision: err = %v, want not found", err)
	}
}

func TestContested(t *testing.T) {
	set := load(t, "testdata/contradictions")

	tests := []struct {
		id   string
		want Status
	}{
		// Contested through the evidence it cites.
		{id: "claim-a", want: Status{Label: Contested, ContestedBy: []string{"rig-lacks-kvm"}}},
		// Its contradiction is only an opinion, so it is not contested.
		{id: "claim-b", want: Status{Label: Grounded}},
		{id: "claim-c", want: Status{Label: Contested, Opinions: []string{"hunch-c"}, ContestedBy: []string{"claim-b"}}},
		// Retracted evidence contests nothing.
		{id: "claim-d", want: Status{Label: Grounded}},
	}

	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			got := status(t, set, test.id)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("status = %+v, want %+v", got, test.want)
			}
		})
	}

	if status(t, without(t, set, "rig-lacks-kvm"), "claim-a").Label != Grounded {
		t.Fatal("claim-a without what contests it is not grounded")
	}

	// In doubt wins over contested.
	got := status(t, without(t, set, "rig-has-kvm"), "claim-a")
	if got.Label != InDoubt || len(got.ContestedBy) != 0 {
		t.Fatalf("claim-a without its evidence = %+v, want in doubt", got)
	}
}

func TestConflicts(t *testing.T) {
	set := load(t, "testdata/contradictions")

	conflicts := set.Conflicts()
	if len(conflicts) != 2 {
		t.Fatalf("conflicts = %+v, want 2", conflicts)
	}

	first := conflicts[0]
	if first.A.ID != "rig-lacks-kvm" || first.A.Kind != "evidence" || !reflect.DeepEqual(ids(first.A.Evidence), []string{"rig-lacks-kvm"}) {
		t.Fatalf("first a = %+v", first.A)
	}
	if first.B.ID != "rig-has-kvm" || !reflect.DeepEqual(ids(first.B.Evidence), []string{"rig-has-kvm"}) {
		t.Fatalf("first b = %+v", first.B)
	}

	second := conflicts[1]
	wantA := Side{ID: "claim-b", Kind: "belief", Evidence: []Evidence{set.Evidence[2]}, Opinions: []string{}}
	wantB := Side{ID: "claim-c", Kind: "belief", Evidence: []Evidence{}, Opinions: []string{"hunch-c"}}
	if !reflect.DeepEqual(second.A, wantA) || !reflect.DeepEqual(second.B, wantB) {
		t.Fatalf("second = %+v, want %+v and %+v", second, wantA, wantB)
	}

	if len(loadGood(t).Conflicts()) != 0 {
		t.Fatal("conflicts where nothing contradicts")
	}
}

// sentences is a small set for the sentence builder.
func sentences(t *testing.T) *Set {
	t.Helper()

	return load(t, writeFiles(t, map[string]string{
		"evidence.yaml": `evidence:
  - id: gofmt-only
    evidence: gofmt ships with the Go toolchain.
    who: franco
    when: 2026-11-01
    source: https://go.dev/blog/gofmt
  - id: small-spec
    evidence: The Go spec is small.
    who: franco
    when: 2026-11-01
  - id: gone
    evidence: G was seen.
    who: ana
    when: 2026-10-01
    retracted: { who: franco, when: 2026-11-02, why: It was H. }
`,
		"beliefs.yaml": `beliefs:
  - id: one-format
    belief: Go has one format.
    evidence: [gofmt-only]
  - id: small-syntax
    belief: Go's syntax is small.
    evidence: [small-spec]
  - id: agents-write-go
    belief: Go suits agents.
    because: [one-format, small-syntax]
  - id: hunch
    belief: The panel helps.
  - id: g-there
    belief: G is there.
    evidence: [gone]
`,
		"decisions.yaml": `decisions:
  - id: use-go
    decision: We write agc in Go.
    because: [agents-write-go, hunch]
    decided: { who: franco, when: 2026-11-01 }
`,
		"actions.yaml": `actions:
  - id: try-go
    action: Port the CLI to Go.
    because: [use-go]
    opened: { who: franco, when: 2026-11-01 }
  - id: done-go
    action: Port the server.
    because: [use-go]
    opened: { who: franco, when: 2026-11-01 }
    done: { who: franco, when: 2026-11-05 }
`,
		"rules.yaml": `subject: s
rules:
  - id: style
    rule: Write Go with gofmt.
    because: [one-format]
  - id: plain
    rule: Never push.
    check: "true"
`,
	}))
}

func beliefLine(t *testing.T, set *Set, id string) Line {
	t.Helper()

	line, err := set.BeliefLine(id)
	if err != nil {
		t.Fatal(err)
	}

	return line
}

func TestBeliefLine(t *testing.T) {
	set := sentences(t)
	contested := load(t, "testdata/contradictions")
	alternatives := load(t, "testdata/alternatives")

	tests := []struct {
		name string
		set  *Set
		id   string
		want Line
	}{
		{
			name: "grounded",
			set:  set,
			id:   "one-format",
			want: Line{Text: "I believe Go has one format.", Trailer: "(one-format, grounded)"},
		},
		{
			name: "opinion",
			set:  set,
			id:   "hunch",
			want: Line{Text: "I think the panel helps, but have no evidence yet.", Trailer: "(hunch, opinion)"},
		},
		{
			name: "in doubt",
			set:  set,
			id:   "g-there",
			want: Line{Text: "I believed G is there, but G was seen was retracted on 2026-11-02: It was H.", Trailer: "(g-there, in doubt: retracted gone)"},
		},
		{
			name: "contested by evidence",
			set:  contested,
			id:   "claim-a",
			want: Line{
				Text:    "I believe the rig can run sandboxes, though it is contested by what ana saw on 2026-11-02, that the rig has no KVM.",
				Trailer: "(claim-a, contested: contested by rig-lacks-kvm)",
			},
		},
		{
			name: "contested opinion",
			set:  contested,
			id:   "claim-c",
			want: Line{
				Text:    "I think C is true, but have no evidence yet that C seems likely, and it is contested by my belief that B is true.",
				Trailer: "(claim-c, contested: opinion hunch-c · contested by claim-b)",
			},
		},
		{
			name: "without evidence",
			set:  without(t, alternatives, "seen-x"),
			id:   "only-x",
			want: Line{Text: "I would doubt that V follows from X without what franco saw on 2026-11-01.", Trailer: "(only-x, in doubt: without seen-x · doubted by-x)"},
		},
		{
			name: "without a belief",
			set:  without(t, alternatives, "by-x"),
			id:   "only-x",
			want: Line{Text: "I would doubt that V follows from X without the belief that X holds.", Trailer: "(only-x, in doubt: without by-x · doubted by-x)"},
		},
		{
			name: "set aside itself",
			set:  without(t, alternatives, "by-x"),
			id:   "by-x",
			want: Line{Text: "I am setting aside that X holds.", Trailer: "(by-x, in doubt)"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := beliefLine(t, test.set, test.id)
			if got != test.want {
				t.Fatalf("line =\n%+v\nwant\n%+v", got, test.want)
			}
		})
	}
}

func TestWhyLines(t *testing.T) {
	set := sentences(t)

	reason, err := set.Why("agents-write-go")
	if err != nil {
		t.Fatal(err)
	}

	want := []Line{
		{
			Text:    "I believe Go suits agents because Go has one format and Go's syntax is small, both seen by franco on 2026-11-01.",
			Trailer: "(agents-write-go ← one-format, small-syntax)",
		},
		{
			Depth:   1,
			Text:    "I believe Go has one format because franco saw on 2026-11-01 that gofmt ships with the Go toolchain.",
			Trailer: "(one-format ← gofmt-only)",
		},
		{
			Depth:   1,
			Text:    "I believe Go's syntax is small because franco saw on 2026-11-01 that the Go spec is small.",
			Trailer: "(small-syntax ← small-spec)",
		},
	}
	got := set.WhyLines(reason, 0)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines =\n%+v\nwant\n%+v", got, want)
	}

	reason, err = set.Why("g-there")
	if err != nil {
		t.Fatal(err)
	}
	got = set.WhyLines(reason, 0)
	want = []Line{{Text: "I believed G is there because ana saw on 2026-10-01 that G was seen, retracted on 2026-11-02: It was H.", Trailer: "(g-there ← gone)"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines =\n%+v\nwant\n%+v", got, want)
	}

	reason, err = set.Why("hunch")
	if err != nil {
		t.Fatal(err)
	}
	got = set.WhyLines(reason, 0)
	want = []Line{{Text: "I think the panel helps, but have no evidence yet.", Trailer: "(hunch, opinion)"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines =\n%+v\nwant\n%+v", got, want)
	}

	alternatives := load(t, "testdata/alternatives")
	reason, err = alternatives.Why("either")
	if err != nil {
		t.Fatal(err)
	}
	got = alternatives.WhyLines(reason, 0)
	want = []Line{
		{Text: "I believe W holds because X holds, seen by franco on 2026-11-01."},
		{Text: "Or because ana saw on 2026-11-02 that Y was seen.", Trailer: "(either ← by-x | seen-y)"},
		{Depth: 1, Text: "I believe X holds because franco saw on 2026-11-01 that X was seen.", Trailer: "(by-x ← seen-x)"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDecisionActionRuleEvidenceLines(t *testing.T) {
	set := sentences(t)

	got := set.DecisionLine(set.Decisions[0])
	want := Line{
		Text:    "I decided that we write agc in Go because Go suits agents and the panel helps, on 2026-11-01 — though I only think that the panel helps.",
		Trailer: "(use-go ← agents-write-go, hunch · decided by franco)",
	}
	if got != want {
		t.Fatalf("decision =\n%+v\nwant\n%+v", got, want)
	}

	got = set.ActionLine(set.Actions[0])
	want = Line{
		Text:    "I will port the CLI to Go, because I decided that we write agc in Go — though I only think that the panel helps.",
		Trailer: "(try-go ← use-go · opened by franco on 2026-11-01)",
	}
	if got != want {
		t.Fatalf("open action =\n%+v\nwant\n%+v", got, want)
	}

	got = set.ActionLine(set.Actions[1])
	want = Line{Text: "I did port the server on 2026-11-05.", Trailer: "(done-go ← use-go · done by franco)"}
	if got != want {
		t.Fatalf("done action =\n%+v\nwant\n%+v", got, want)
	}

	rules := set.Rulesets[0].Rules
	got = set.RuleLine(rules[0], true)
	want = Line{Text: "Write Go with gofmt because Go has one format.", Trailer: "(style)"}
	if got != want {
		t.Fatalf("rule =\n%+v\nwant\n%+v", got, want)
	}

	got = set.RuleLine(rules[1], true)
	want = Line{Text: "Never push.", Trailer: "(plain · check: true)"}
	if got != want {
		t.Fatalf("rule with check =\n%+v\nwant\n%+v", got, want)
	}

	got = set.EvidenceLine(set.Evidence[0])
	want = Line{Text: "franco saw that gofmt ships with the Go toolchain on 2026-11-01.", Trailer: "(gofmt-only · source: https://go.dev/blog/gofmt · cited by 1 belief)"}
	if got != want {
		t.Fatalf("evidence =\n%+v\nwant\n%+v", got, want)
	}

	got = set.EvidenceLine(set.Evidence[2])
	want = Line{Text: "ana saw that G was seen on 2026-10-01, retracted on 2026-11-02: It was H.", Trailer: "(gone · cited by 1 belief)"}
	if got != want {
		t.Fatalf("retracted evidence =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDoubtAndConflictLines(t *testing.T) {
	alternatives := load(t, "testdata/alternatives")

	doubted, err := alternatives.Doubt("seen-x")
	if err != nil {
		t.Fatal(err)
	}

	got := alternatives.StandingLine(doubted.Standing[0])
	want := Line{Text: "I believe W holds, which still stands because ana saw on 2026-11-02 that Y was seen.", Trailer: "(either ← seen-y)"}
	if got != want {
		t.Fatalf("standing =\n%+v\nwant\n%+v", got, want)
	}

	header := alternatives.DoubtHeader("seen-x")
	if header != "If what franco saw on 2026-11-01 is wrong, that X was seen, recheck what rests on it." {
		t.Fatalf("header = %q", header)
	}

	contested := load(t, "testdata/contradictions")
	conflicts := contested.Conflicts()

	lines := contested.ConflictLines(conflicts[0])
	wantLines := []Line{{Text: "ana saw on 2026-11-02 that the rig has no KVM, but franco saw on 2026-11-01 that the rig has KVM.", Trailer: "(rig-lacks-kvm ↔ rig-has-kvm)"}}
	if !reflect.DeepEqual(lines, wantLines) {
		t.Fatalf("lines =\n%+v\nwant\n%+v", lines, wantLines)
	}

	lines = contested.ConflictLines(conflicts[1])
	wantLines = []Line{
		{Text: "I believe B is true, but I think C is true."},
		{Depth: 1, Text: "The first rests on what franco saw on 2026-11-01, that B was seen."},
		{Depth: 1, Text: "The second rests on my opinion that C seems likely.", Trailer: "(claim-b ↔ claim-c)"},
	}
	if !reflect.DeepEqual(lines, wantLines) {
		t.Fatalf("lines =\n%+v\nwant\n%+v", lines, wantLines)
	}
}

func TestEmbed(t *testing.T) {
	set := sentences(t)

	tests := map[string]string{
		"Go has one format.":          "Go has one format",
		"Go's syntax is small.":       "Go's syntax is small",
		"The panel helps.":            "the panel helps",
		"A local SQLite file.":        "a local SQLite file",
		"SQLite is enough.":           "SQLite is enough",
		"G is there.":                 "G is there",
		"Several\n  lines folded.":    "several lines folded",
		"gofmt ships with Go.":        "gofmt ships with Go",
		"Ends with a version 1.2.":    "ends with a version 1.2",
		"I hold this.":                "I hold this",
		"":                            "",
		"`go vet ./...` is clean.":    "`go vet ./...` is clean",
		"Over SSH, pkill -f kills.":   "over SSH, pkill -f kills",
		"When it rains, it pours.":    "when it rains, it pours",
		"Never write `;` in Go code.": "never write `;` in Go code",
	}

	for text, want := range tests {
		got := set.embed(text)
		if got != want {
			t.Fatalf("embed(%q) = %q, want %q", text, got, want)
		}
	}

	if joinAnd([]string{"a"}) != "a" || joinAnd([]string{"a", "b"}) != "a and b" || joinAnd([]string{"a", "b", "c"}) != "a, b and c" {
		t.Fatal("joinAnd")
	}
	if joinAnd([]string{"a, x", "b"}) != "a, x, and b" {
		t.Fatalf("joinAnd with commas = %q", joinAnd([]string{"a, x", "b"}))
	}
}

func TestKind(t *testing.T) {
	set := loadGood(t)

	tests := map[string]string{
		"seen-a":      "evidence",
		"observed-a":  "belief",
		"choice":      "decision",
		"passes":      "rule",
		"open-action": "action",
	}
	for id, want := range tests {
		got, found := set.Kind(id)
		if !found || got != want {
			t.Fatalf("Kind(%q) = %q, %v, want %q", id, got, found, want)
		}
	}

	got, found := set.Kind("missing")
	if found || got != "" {
		t.Fatalf("Kind(missing) = %q, %v", got, found)
	}
}

func TestWhyAny(t *testing.T) {
	set := loadGood(t)

	// A rule follows down through decisions to beliefs and evidence.
	node, err := set.WhyAny("manual-one")
	if err != nil {
		t.Fatal(err)
	}
	if node.Kind != "rule" || node.Text != "Must be read by a human." || len(node.Because) != 1 {
		t.Fatalf("manual-one = %+v", node)
	}
	later := node.Because[0]
	if later.Kind != "decision" || later.ID != "later-choice" || len(later.Because) != 1 || later.Because[0].ID != "choice" {
		t.Fatalf("later-choice = %+v", later)
	}
	derived := later.Because[0].Because[0]
	if derived.Kind != "belief" || derived.ID != "derived" || len(derived.Because) != 2 {
		t.Fatalf("derived = %+v", derived)
	}
	leaf := derived.Because[0]
	if leaf.ID != "observed-a" || !reflect.DeepEqual(ids(leaf.Evidence), []string{"seen-a"}) || leaf.Evidence[0].Who != "franco" {
		t.Fatalf("observed-a = %+v", leaf)
	}

	node, err = set.WhyAny("open-action")
	if err != nil {
		t.Fatal(err)
	}
	if node.Kind != "action" || node.File != filepath.Join("testdata", "good", "actions.yaml") || len(node.Because) != 1 || node.Because[0].Kind != "decision" {
		t.Fatalf("open-action = %+v", node)
	}

	// A rule resting on nothing has no reasons.
	node, err = set.WhyAny("fails")
	if err != nil {
		t.Fatal(err)
	}
	if len(node.Because) != 0 || len(node.Evidence) != 0 || len(node.Or) != 0 {
		t.Fatalf("fails = %+v", node)
	}

	// Alternatives are kept apart from the node's own reasons.
	alternatives := load(t, "testdata/alternatives")
	node, err = alternatives.WhyAny("either")
	if err != nil {
		t.Fatal(err)
	}
	if node.Kind != "belief" || len(node.Because) != 1 || node.Because[0].ID != "by-x" || len(node.Or) != 1 || !reflect.DeepEqual(ids(node.Or[0].Evidence), []string{"seen-y"}) {
		t.Fatalf("either = %+v", node)
	}
	node, err = alternatives.WhyAny("rule-or")
	if err != nil {
		t.Fatal(err)
	}
	if node.Because[0].ID != "only-x" || len(node.Or) != 1 || node.Or[0].Because[0].ID != "by-y" || node.Or[0].Because[0].Evidence[0].Who != "ana" {
		t.Fatalf("rule-or = %+v", node)
	}

	for _, id := range []string{"seen-a", "missing"} {
		_, err = set.WhyAny(id)
		if ez.ErrorCode(err) != ez.ENOTFOUND {
			t.Fatalf("WhyAny(%s): err = %v, want not found", id, err)
		}
	}
}

func TestDoubtDecisionAndRule(t *testing.T) {
	set := loadGood(t)

	doubted, err := set.Doubt("choice")
	if err != nil {
		t.Fatal(err)
	}
	if len(doubted.Beliefs) != 0 || !reflect.DeepEqual(ids(doubted.Decisions), []string{"later-choice"}) {
		t.Fatalf("choice beliefs = %v, decisions = %v", ids(doubted.Beliefs), ids(doubted.Decisions))
	}
	if !reflect.DeepEqual(ids(doubted.Rules), []string{"manual-one"}) || !reflect.DeepEqual(ids(doubted.Actions), []string{"open-action"}) {
		t.Fatalf("choice rules = %v, actions = %v", ids(doubted.Rules), ids(doubted.Actions))
	}

	doubted, err = set.Doubt("passes")
	if err != nil {
		t.Fatal(err)
	}
	if len(doubted.Beliefs)+len(doubted.Decisions)+len(doubted.Rules)+len(doubted.Actions)+len(doubted.Standing) != 0 {
		t.Fatalf("passes = %+v, want nothing resting on a rule", doubted)
	}

	// act-stands rests on decide-x but has choose-either too.
	alternatives := load(t, "testdata/alternatives")
	doubted, err = alternatives.Doubt("decide-x")
	if err != nil {
		t.Fatal(err)
	}
	if len(doubted.Decisions) != 0 || !reflect.DeepEqual(ids(doubted.Actions), []string{"act-falls"}) {
		t.Fatalf("decide-x decisions = %v, actions = %v", ids(doubted.Decisions), ids(doubted.Actions))
	}
	want := []Standing{{Kind: "action", ID: "act-stands", Text: "Do V or W.", File: filepath.Join("testdata", "alternatives", "actions.yaml"), Other: []Support{{Because: []string{"choose-either"}}}}}
	if !reflect.DeepEqual(doubted.Standing, want) {
		t.Fatalf("decide-x standing = %+v", doubted.Standing)
	}

	_, err = set.Doubt("open-action")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("Doubt of an action: err = %v, want not found", err)
	}
}

// judgedRules is a ruleset with every kind of rule: one judged by agents,
// one by a person only, a manual one, and a mechanical one also judged.
const judgedRules = `subject: judged
rules:
  - id: by-agents
    rule: Errors are named err.
    judge:
      trusted: [claude_code, codex_cli]
      ask: Are errors named err?
  - id: by-franco
    rule: Never run rm -rf on the rig.
    judge:
      trusted: [franco]
  - id: by-hand
    rule: Never hand-edit a database.
  - id: checked-and-judged
    rule: One statement per line.
    check: "true"
    judge:
      trusted: [codex_cli]
`

func TestJudgeBlockValidation(t *testing.T) {
	tests := []struct {
		name string
		file string
		want string
	}{
		{
			name: "judge without trusted",
			file: "subject: s\nrules:\n  - id: r\n    rule: R.\n    judge:\n      ask: Is it?\n",
			want: "rule r: judge: needs trusted",
		},
		{
			name: "judge with an empty name",
			file: "subject: s\nrules:\n  - id: r\n    rule: R.\n    judge:\n      trusted: [\"\"]\n",
			want: "rule r: judge: trusted has an empty name",
		},
		{
			name: "judge trusting a model that is not a harness",
			file: "subject: s\nrules:\n  - id: r\n    rule: R.\n    judge:\n      trusted: [gpt-5]\n",
			want: "rule r: judge: trusted names gpt-5, a model but not a harness id",
		},
		{
			name: "unknown judge field",
			file: "subject: s\nrules:\n  - id: r\n    rule: R.\n    judge:\n      trusted: [franco]\n      who: me\n",
			want: "field who not found",
		},
		{
			name: "belief when without by",
			file: "beliefs:\n  - id: b\n    belief: B.\n    when: 2026-11-01\n",
			want: "belief b: when: says when by: held it",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := writeFiles(t, map[string]string{"a.yaml": test.file})
			_, err := Load(dir)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}

	dir := writeFiles(t, map[string]string{"rules.yaml": judgedRules})
	set := load(t, dir)
	rule := set.Rulesets[0].Rules[0]
	if rule.Judge == nil || !reflect.DeepEqual(rule.Judge.Trusted, []string{"claude_code", "codex_cli"}) || rule.Judge.Ask != "Are errors named err?" {
		t.Fatalf("judge = %+v", rule.Judge)
	}
}

func TestRuleKinds(t *testing.T) {
	set := load(t, writeFiles(t, map[string]string{"rules.yaml": judgedRules}))
	rules := set.Rulesets[0].Rules

	wantKinds := []Kind{Judged, Judged, Manual, Mechanical}
	wantNotes := []string{
		"needs claude_code or codex_cli",
		"needs franco",
		"needs a human",
		"may also be judged by codex_cli",
	}
	for i, rule := range rules {
		if rule.Kind() != wantKinds[i] {
			t.Fatalf("%s kind = %s, want %s", rule.ID, rule.Kind(), wantKinds[i])
		}
		if JudgeNote(rule) != wantNotes[i] {
			t.Fatalf("%s note = %q, want %q", rule.ID, JudgeNote(rule), wantNotes[i])
		}
	}

	if JudgeNote(Rule{Check: "true"}) != "" {
		t.Fatalf("plain mechanical note = %q", JudgeNote(Rule{Check: "true"}))
	}

	results, err := set.Check(context.Background(), "judged", Shell)
	if err != nil {
		t.Fatal(err)
	}
	for i, result := range results {
		if result.Kind != wantKinds[i] {
			t.Fatalf("%s result kind = %s, want %s", result.Rule.ID, result.Kind, wantKinds[i])
		}
		if (result.Passed != nil) != (wantKinds[i] == Mechanical) {
			t.Fatalf("%s passed = %v", result.Rule.ID, result.Passed)
		}
	}

	if !reflect.DeepEqual(set.Judges(), []string{"claude_code", "codex_cli", "franco"}) {
		t.Fatalf("judges = %v", set.Judges())
	}
}

func TestJudgeTrustedFiltering(t *testing.T) {
	set := load(t, writeFiles(t, map[string]string{"rules.yaml": judgedRules}))
	subject := Subject{Name: "the diff against HEAD~1", Text: "+parseErr := parse()"}

	prompts := map[string]string{}
	answers := map[string]string{
		"by-agents":          "VERDICT: fail — parseErr is not named err.",
		"checked-and-judged": "VERDICT: pass — one statement per line.",
	}
	fake := func(ctx context.Context, rule Rule, prompt string) (string, error) {
		prompts[rule.ID] = prompt
		return answers[rule.ID], nil
	}

	judgments, err := set.Judge(context.Background(), "codex_cli", subject, fake)
	if err != nil {
		t.Fatal(err)
	}

	got := []string{}
	for _, judgment := range judgments {
		state := string(judgment.Verdict)
		if judgment.Skipped {
			state = "skipped"
		}
		got = append(got, judgment.Rule.ID+":"+state)
	}
	want := []string{"by-agents:fail", "by-franco:skipped", "checked-and-judged:pass"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("judgments = %v, want %v", got, want)
	}
	if judgments[0].Reason != "parseErr is not named err." || judgments[0].By != "codex_cli" {
		t.Fatalf("judgment = %+v", judgments[0])
	}

	prompt := prompts["by-agents"]
	for _, part := range []string{"Rule: Errors are named err.", "Question: Are errors named err?", "the diff against HEAD~1", "+parseErr := parse()", "VERDICT: pass", "VERDICT: fail"} {
		if !strings.Contains(prompt, part) {
			t.Fatalf("prompt misses %q:\n%s", part, prompt)
		}
	}
	if !strings.Contains(prompts["checked-and-judged"], "Question: One statement per line.") {
		t.Fatalf("ask does not default to the rule text:\n%s", prompts["checked-and-judged"])
	}
	_, asked := prompts["by-franco"]
	if asked {
		t.Fatal("asked an untrusted judge")
	}

	judgments, err = set.Judge(context.Background(), "pi", subject, fake)
	if err != nil {
		t.Fatalf("a harness trusted nowhere: %v", err)
	}
	for _, judgment := range judgments {
		if !judgment.Skipped {
			t.Fatalf("pi judged %s", judgment.Rule.ID)
		}
	}

	_, err = set.Judge(context.Background(), "nobody", subject, fake)
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(err.Error(), "unknown judge nobody") {
		t.Fatalf("unknown judge err = %v", err)
	}

	_, err = set.Judge(context.Background(), "franco", subject, nil)
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(err.Error(), "franco is a person") {
		t.Fatalf("person err = %v", err)
	}

	failing := func(ctx context.Context, rule Rule, prompt string) (string, error) {
		return "", ez.New(ez.EINTERNAL, "claude run failed", nil)
	}
	judgments, err = set.Judge(context.Background(), "claude_code", subject, failing)
	if err != nil {
		t.Fatal(err)
	}
	if judgments[0].Verdict != Unclear || !strings.Contains(judgments[0].Reason, "claude run failed") {
		t.Fatalf("failed judge = %+v", judgments[0])
	}
}

func TestParseVerdict(t *testing.T) {
	tests := []struct {
		answer  string
		verdict Verdict
		reason  string
	}{
		{"VERDICT: pass — every error is named err.", Pass, "every error is named err."},
		{"VERDICT: fail — judge.go names one readErr.", Fail, "judge.go names one readErr."},
		{"verdict: PASS - fine.", Pass, "fine."},
		{"VERDICT: fail: no migration for the new column.", Fail, "no migration for the new column."},
		{"I read the diff.\n\n**VERDICT:** pass — nothing touches REST.\n", Pass, "nothing touches REST."},
		{"Looks fine to me.", Unclear, ""},
		{"VERDICT: maybe — hard to say.", Unclear, ""},
		{"VERDICT: pass —", Unclear, ""},
		{"VERDICT: pass — a.\nVERDICT: fail — b.", Unclear, ""},
		{"", Unclear, ""},
	}

	for _, test := range tests {
		verdict, reason := ParseVerdict(test.answer)
		if verdict != test.verdict || reason != test.reason {
			t.Fatalf("ParseVerdict(%q) = %s %q, want %s %q", test.answer, verdict, reason, test.verdict, test.reason)
		}
	}
}

func TestRecordJudgmentsAsOpinions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "beliefs", "judgments.yaml")
	subject := Subject{Name: "the diff against HEAD~1"}
	judgments := []Judgment{
		{Rule: Rule{ID: "go-err-naming", Rule: "Name error variables `err`."}, By: "codex_cli", Verdict: Pass, Reason: "every error is named err."},
		{Rule: Rule{ID: "rest-endpoint-openapi", Rule: "A new REST endpoint is documented."}, By: "codex_cli", Verdict: Fail, Reason: "GET /judge is missing from openapi.yaml."},
		{Rule: Rule{ID: "go-ez-errors", Rule: "Use ez."}, By: "codex_cli", Verdict: Unclear},
		{Rule: Rule{ID: "rig-no-rm-rf", Rule: "No rm -rf."}, By: "codex_cli", Skipped: true},
	}

	written, err := Record(path, judgments, subject, "abc1234", "2026-11-01", map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 2 {
		t.Fatalf("wrote %d, want the pass and the fail only", len(written))
	}

	set := load(t, path)
	if len(set.Evidence) != 0 {
		t.Fatalf("recorded evidence: %+v", set.Evidence)
	}
	if len(set.Beliefs) != 2 {
		t.Fatalf("beliefs = %+v", set.Beliefs)
	}

	first := set.Beliefs[0]
	wantText := "“Name error variables `err`” held on the diff against HEAD~1: every error is named err."
	if first.ID != "judged-go-err-naming-abc1234" || first.By != "codex_cli" || first.When != "2026-11-01" || first.Belief != wantText {
		t.Fatalf("first = %+v", first)
	}
	if len(first.Evidence) != 0 || len(first.Because) != 0 {
		t.Fatalf("a judgment cites something: %+v", first)
	}
	if !strings.Contains(set.Beliefs[1].Belief, "did not hold on the diff against HEAD~1: GET /judge is missing") {
		t.Fatalf("fail = %q", set.Beliefs[1].Belief)
	}

	for _, belief := range set.Beliefs {
		label := status(t, set, belief.ID).Label
		if label != Opinion {
			t.Fatalf("%s is %s, want opinion", belief.ID, label)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "# Verdicts of agents") {
		t.Fatalf("header lost:\n%s", data)
	}

	taken := map[string]bool{"judged-go-err-naming-abc1234-2": true}
	written, err = Record(path, judgments[:1], subject, "abc1234", "2026-11-02", taken)
	if err != nil {
		t.Fatal(err)
	}
	if written[0].ID != "judged-go-err-naming-abc1234-3" {
		t.Fatalf("repeated id = %s", written[0].ID)
	}

	set = load(t, path)
	if len(set.Beliefs) != 3 || len(set.Evidence) != 0 {
		t.Fatalf("after a second record: %d beliefs, %d evidence", len(set.Beliefs), len(set.Evidence))
	}

	written, err = Record(filepath.Join(dir, "none.yaml"), judgments[2:], subject, "abc1234", "2026-11-01", nil)
	if err != nil || len(written) != 0 {
		t.Fatalf("unclear and skipped wrote %v, %v", written, err)
	}
	_, err = os.Stat(filepath.Join(dir, "none.yaml"))
	if !os.IsNotExist(err) {
		t.Fatalf("a file was written with nothing to record: %v", err)
	}

	notBeliefs := filepath.Join(dir, "rules.yaml")
	err = os.WriteFile(notBeliefs, []byte(judgedRules), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Record(notBeliefs, judgments, subject, "abc1234", "2026-11-01", nil)
	if err == nil || !strings.Contains(err.Error(), "not a beliefs file") {
		t.Fatalf("recording into a ruleset: %v", err)
	}
}

func TestReadSubject(t *testing.T) {
	ctx := context.Background()
	dir := writeFiles(t, map[string]string{"a.go": "package a\n", "b.bin": "x\x00y"})

	subject, err := ReadSubject(ctx, filepath.Join(dir, "a.go"), nil)
	if err != nil || subject.Text != "package a\n" || subject.Name != filepath.Join(dir, "a.go") {
		t.Fatalf("file subject = %+v, %v", subject, err)
	}

	subject, err = ReadSubject(ctx, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(subject.Text, "Files:\n") || !strings.Contains(subject.Text, "package a") || strings.Contains(subject.Text, "x\x00y") {
		t.Fatalf("dir subject = %q", subject.Text)
	}

	subject, err = ReadSubject(ctx, "-", strings.NewReader(strings.Repeat("a", subjectCap+10)))
	if err != nil || subject.Name != "stdin" || !strings.HasSuffix(subject.Text, "[cut at 200 KB]") {
		t.Fatalf("stdin subject = %d bytes, %v", len(subject.Text), err)
	}

	_, err = ReadSubject(ctx, "-", strings.NewReader("  \n"))
	if err == nil || !strings.Contains(err.Error(), "nothing to judge") {
		t.Fatalf("empty subject err = %v", err)
	}

	_, err = ReadSubject(ctx, "no-such-path-or-ref-xyz", nil)
	if err == nil || !strings.Contains(err.Error(), "neither a path nor a git ref") {
		t.Fatalf("bad --on err = %v", err)
	}
}

func TestJudgment(t *testing.T) {
	set := load(t, "testdata/judgment")

	cases := []struct {
		id   string
		want Status
	}{
		{id: "icons-help", want: Status{Label: ExpertJudgment, Judgments: []string{"icons-help"}}},
		{id: "panel-uses-icons", want: Status{Label: ExpertJudgment, Judgments: []string{"icons-help"}}},
		{id: "icons-everywhere", want: Status{Label: Opinion, Opinions: []string{"hunch-screens"}, Judgments: []string{"icons-help"}}},
		{id: "ana-thinks-slots", want: Status{Label: Grounded}},
	}
	for _, c := range cases {
		got := status(t, set, c.id)
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%s = %+v, want %+v", c.id, got, c.want)
		}
	}

	got := set.StatusOf([]Support{{Because: []string{"panel-uses-icons"}}})
	if got.Label != ExpertJudgment || !reflect.DeepEqual(got.Judgments, []string{"icons-help"}) {
		t.Fatalf("StatusOf over a judgment = %+v", got)
	}

	belief, _ := set.belief("ana-judges-shapes")
	if belief.By != "ana" || belief.Experience == nil || belief.Experience.Of != "ana" || belief.Experience.In != "color-blind testing" {
		t.Fatalf("ana-judges-shapes = %+v", belief)
	}

	if status(t, without(t, set, "icons-help"), "panel-uses-icons").Label != InDoubt {
		t.Fatal("without icons-help, panel-uses-icons should be in doubt")
	}
}

func TestJudgmentLines(t *testing.T) {
	set := load(t, "testdata/judgment")

	cases := []struct {
		id   string
		want Line
	}{
		{id: "icons-help", want: Line{Text: "I believe icons help operators, from franco's experience in years of operator UIs.", Trailer: "(icons-help, judgment)"}},
		{id: "panel-uses-icons", want: Line{Text: "I believe the panel should use icons.", Trailer: "(panel-uses-icons, judgment: judgment icons-help)"}},
		{id: "icons-everywhere", want: Line{Text: "I think every screen should use icons, but have no evidence yet that screens are read at a glance.", Trailer: "(icons-everywhere, opinion: opinion hunch-screens · judgment icons-help)"}},
		{id: "ana-thinks-slots", want: Line{Text: "ana believes slots beat lists.", Trailer: "(ana-thinks-slots, grounded)"}},
		{id: "ana-judges-shapes", want: Line{Text: "ana believes shapes beat colors, from ana's experience in color-blind testing.", Trailer: "(ana-judges-shapes, judgment)"}},
		{id: "hunch-screens", want: Line{Text: "I think screens are read at a glance, but have no evidence yet.", Trailer: "(hunch-screens, opinion)"}},
	}
	for _, c := range cases {
		got := beliefLine(t, set, c.id)
		if got != c.want {
			t.Fatalf("%s = %+v, want %+v", c.id, got, c.want)
		}
	}

	reason, err := set.Why("panel-uses-icons")
	if err != nil {
		t.Fatal(err)
	}
	lines := set.WhyLines(reason, 0)
	if len(lines) != 3 {
		t.Fatalf("why lines = %+v", lines)
	}
	leaf := Line{Depth: 1, Text: "I believe icons help operators, from franco's experience in years of operator UIs.", Trailer: "(icons-help, judgment)"}
	if lines[1] != leaf {
		t.Fatalf("why leaf = %+v, want %+v", lines[1], leaf)
	}
	if !strings.HasPrefix(lines[0].Text, "I believe the panel should use icons because icons help operators and the panel is for operators") {
		t.Fatalf("why root = %q", lines[0].Text)
	}
}

func TestJudgmentRejects(t *testing.T) {
	_, err := Load("testdata/judgment/bad")
	if err == nil || !strings.Contains(err.Error(), "model-judges") || !strings.Contains(err.Error(), "experience: of must be a person") {
		t.Fatalf("model experience: err = %v", err)
	}

	dir := writeFiles(t, map[string]string{"beliefs.yaml": "beliefs:\n  - id: half\n    belief: Half an experience.\n    experience: { of: franco }\n"})
	_, err = Load(dir)
	if err == nil || !strings.Contains(err.Error(), "experience: needs of and in") {
		t.Fatalf("experience without in: err = %v", err)
	}
}
