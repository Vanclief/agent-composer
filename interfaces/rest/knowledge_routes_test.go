package rest

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/vanclief/compose/components/ratelimit"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/knowledge"
	"github.com/vanclief/agent-composer/interfaces/rest/handler"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
)

// knowledgeFixture is a small knowledge directory, one file per kind,
// shaped like knowledge/testdata/good.
var knowledgeFixture = map[string]string{
	"evidence/seen.yaml": `evidence:
  - id: seen-a
    evidence: A was seen.
    who: franco
    when: 2026-11-01
    source: https://example.com/a
    translated_by: codex-cli
  - id: seen-g
    evidence: G was seen.
    who: franco
    when: 2026-10-01
    retracted: { who: franco, when: 2026-11-02, why: G turned out not to be there. }
`,
	"beliefs/held.yaml": `beliefs:
  - id: observed-a
    belief: A was seen.
    evidence: [seen-a]
  - id: derived
    belief: C follows from A.
    because: [observed-a]
  - id: hunch
    belief: F is true, though nobody saw it.
  - id: retracted-only
    belief: G is there.
    evidence: [seen-g]
`,
	"decisions/made.yaml": `decisions:
  - id: choice
    decision: We chose X because of C.
    because: [derived]
    adr: docs/decisions/x.md
    decided: { who: franco, when: 2026-11-01 }
  - id: doubted-choice
    decision: We chose Z because G is there.
    because: [retracted-only]
    decided: { who: franco, when: 2026-11-01 }
`,
	"rules/testing.yaml": `subject: testing
rules:
  - id: passes
    rule: Must pass.
    check: echo fine
    because: [choice]
  - id: by-hand
    rule: Must be read by a human.
    because: [hunch]
`,
}

// newKnowledgeRoutes serves the API routes with dir as the knowledge
// directory.
func newKnowledgeRoutes(t *testing.T, dir string) *echo.Echo {
	t.Helper()

	server := &restserver.Server{
		Ctrl:         &controller.Controller{},
		RateLimiter:  ratelimit.NewWindowCounter(10, 100),
		KnowledgeAPI: knowledge.NewAPI(dir),
	}

	e := echo.New()
	addAPIRoutes(e, handler.NewHandler(server))

	return e
}

func writeKnowledge(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		err := os.MkdirAll(filepath.Dir(path), 0o755)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(path, []byte(content), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

// getKnowledge serves GET path, checks the status and returns the body.
func getKnowledge(t *testing.T, e *echo.Echo, path string, status int) []byte {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	recorder := httptest.NewRecorder()

	e.ServeHTTP(recorder, request)

	if recorder.Code != status {
		t.Fatalf("GET %s: status %d, body %s", path, recorder.Code, recorder.Body.String())
	}

	return recorder.Body.Bytes()
}

func decodeKnowledge(t *testing.T, body []byte, into any) {
	t.Helper()

	err := json.Unmarshal(body, into)
	if err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
}

func TestKnowledgeRoutesEmptyDir(t *testing.T) {
	for _, dir := range []string{t.TempDir(), filepath.Join(t.TempDir(), "missing")} {
		e := newKnowledgeRoutes(t, dir)

		body := getKnowledge(t, e, "/api/knowledge", http.StatusOK)
		want := `{"dir":"` + dir + `","without":[],"evidence":[],"beliefs":[],"decisions":[],"rules":[],"actions":[]}`
		if strings.TrimSpace(string(body)) != want {
			t.Fatalf("body %s, want %s", body, want)
		}

		getKnowledge(t, e, "/api/knowledge/why/anything", http.StatusNotFound)
	}
}

func TestKnowledgeRoutesList(t *testing.T) {
	dir := writeKnowledge(t, knowledgeFixture)
	e := newKnowledgeRoutes(t, dir)

	body := getKnowledge(t, e, "/api/knowledge", http.StatusOK)

	var set knowledge.KnowledgeSet
	decodeKnowledge(t, body, &set)

	if set.Dir != dir || len(set.Evidence) != 2 || len(set.Beliefs) != 4 || len(set.Decisions) != 2 || len(set.Rules) != 2 {
		t.Fatalf("set %s", body)
	}

	seenA := set.Evidence[0]
	if seenA.ID != "seen-a" || seenA.Text != "A was seen." || seenA.Source != "https://example.com/a" || seenA.Retracted != nil || !slices.Equal(seenA.CitedBy, []string{"observed-a"}) || seenA.TranslatedBy != "codex-cli" || !slices.Equal(seenA.Contradicts, []string{}) {
		t.Fatalf("evidence seen-a %+v", seenA)
	}
	if seenA.File != filepath.Join(dir, "evidence", "seen.yaml") {
		t.Fatalf("evidence file %s", seenA.File)
	}

	seenG := set.Evidence[1]
	if seenG.Retracted == nil || seenG.Retracted.Why != "G turned out not to be there." {
		t.Fatalf("evidence seen-g %+v", seenG)
	}

	labels := map[string]string{}
	for _, belief := range set.Beliefs {
		labels[belief.ID] = belief.Label
	}
	want := map[string]string{"observed-a": "grounded", "derived": "grounded", "hunch": "opinion", "retracted-only": "in_doubt"}
	for id, label := range want {
		if labels[id] != label {
			t.Fatalf("belief %s label %q, want %q", id, labels[id], label)
		}
	}

	doubted := set.Beliefs[3]
	if !slices.Equal(doubted.Doubted, []string{"retracted-only"}) || !slices.Equal(doubted.Retracted, []string{"seen-g"}) {
		t.Fatalf("belief retracted-only %+v", doubted)
	}

	choice := set.Decisions[0]
	if choice.Label != "grounded" || choice.ADR != "docs/decisions/x.md" || choice.Decided.Who != "franco" {
		t.Fatalf("decision choice %+v", choice)
	}
	if set.Decisions[1].Label != "in_doubt" {
		t.Fatalf("decision doubted-choice %+v", set.Decisions[1])
	}

	passes := set.Rules[0]
	if passes.Subject != "testing" || passes.Kind != "mechanical" || passes.Label != "grounded" || passes.Check != "echo fine" {
		t.Fatalf("rule passes %+v", passes)
	}
	byHand := set.Rules[1]
	if byHand.Kind != "manual" || byHand.Label != "opinion" || !slices.Equal(byHand.Opinions, []string{"hunch"}) {
		t.Fatalf("rule by-hand %+v", byHand)
	}

	// Empty lists are [] not null.
	// The only nulls are the retracted of live evidence and the
	// experience of beliefs that are not judgments.
	stripped := strings.ReplaceAll(string(body), `"retracted":null`, "")
	stripped = strings.ReplaceAll(stripped, `"experience":null`, "")
	if strings.Contains(stripped, "null") {
		t.Fatalf("body has a null list: %s", body)
	}
	if !strings.Contains(string(body), `"opinions":[]`) || !strings.Contains(string(body), `"because":[]`) {
		t.Fatalf("body %s has no empty lists", body)
	}
}

func TestKnowledgeRoutesWhy(t *testing.T) {
	e := newKnowledgeRoutes(t, writeKnowledge(t, knowledgeFixture))

	body := getKnowledge(t, e, "/api/knowledge/why/derived", http.StatusOK)

	var reason knowledge.Reason
	decodeKnowledge(t, body, &reason)

	if reason.ID != "derived" || len(reason.Evidence) != 0 || len(reason.Because) != 1 {
		t.Fatalf("reason %s", body)
	}

	leaf := reason.Because[0]
	if leaf.ID != "observed-a" || len(leaf.Evidence) != 1 || leaf.Evidence[0].ID != "seen-a" || leaf.Evidence[0].Who != "franco" {
		t.Fatalf("reason leaf %s", body)
	}

	body = getKnowledge(t, e, "/api/knowledge/why/nope", http.StatusNotFound)
	if !strings.Contains(string(body), "belief, decision, rule or action nope not found") {
		t.Fatalf("body %s", body)
	}
}

func TestKnowledgeRoutesDoubt(t *testing.T) {
	e := newKnowledgeRoutes(t, writeKnowledge(t, knowledgeFixture))

	cases := []struct {
		id        string
		beliefs   []string
		decisions []string
		rules     []string
	}{
		{"observed-a", []string{"derived"}, []string{"choice"}, []string{"passes"}},
		{"seen-g", []string{"retracted-only"}, []string{"doubted-choice"}, []string{}},
		{"hunch", []string{}, []string{}, []string{"by-hand"}},
	}

	for _, tc := range cases {
		body := getKnowledge(t, e, "/api/knowledge/doubt/"+tc.id, http.StatusOK)

		var doubted knowledge.Doubted
		decodeKnowledge(t, body, &doubted)

		beliefs := []string{}
		for _, belief := range doubted.Beliefs {
			beliefs = append(beliefs, belief.ID)
		}
		decisions := []string{}
		for _, decision := range doubted.Decisions {
			decisions = append(decisions, decision.ID)
		}
		rules := []string{}
		for _, rule := range doubted.Rules {
			rules = append(rules, rule.ID)
			if rule.Subject != "testing" {
				t.Fatalf("doubt %s: rule %s has subject %q", tc.id, rule.ID, rule.Subject)
			}
		}

		if !slices.Equal(beliefs, tc.beliefs) || !slices.Equal(decisions, tc.decisions) || !slices.Equal(rules, tc.rules) {
			t.Fatalf("doubt %s: %s", tc.id, body)
		}
	}

	body := getKnowledge(t, e, "/api/knowledge/doubt/nope", http.StatusNotFound)
	if !strings.Contains(string(body), "belief, evidence, decision or rule nope not found") {
		t.Fatalf("body %s", body)
	}
}

func TestKnowledgeRoutesValidationError(t *testing.T) {
	dir := writeKnowledge(t, map[string]string{
		"beliefs/held.yaml": `beliefs:
  - id: orphan
    belief: Nobody saw this.
    evidence: [missing]
`,
	})
	e := newKnowledgeRoutes(t, dir)

	want := filepath.Join(dir, "beliefs", "held.yaml") + ": belief orphan: evidence names missing, which is not loaded evidence"
	for _, path := range []string{"/api/knowledge", "/api/knowledge/why/orphan", "/api/knowledge/doubt/orphan", "/api/knowledge/conflicts"} {
		body := getKnowledge(t, e, path, http.StatusBadRequest)

		var envelope struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		decodeKnowledge(t, body, &envelope)

		if envelope.Error.Message != want {
			t.Fatalf("GET %s: message %q, want %q", path, envelope.Error.Message, want)
		}
	}
}

// copyTestdata lays knowledge/testdata/<name> out as a knowledge
// directory: each <kind>.yaml goes in <kind>/.
func copyTestdata(t *testing.T, name string) string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join("..", "..", "knowledge", "testdata", name, "*.yaml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("testdata %s: %v", name, err)
	}

	files := map[string]string{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		kind := strings.TrimSuffix(filepath.Base(path), ".yaml")
		files[filepath.Join(kind, kind+".yaml")] = string(data)
	}

	return writeKnowledge(t, files)
}

func listKnowledge(t *testing.T, e *echo.Echo, path string) (knowledge.KnowledgeSet, string) {
	t.Helper()

	body := getKnowledge(t, e, path, http.StatusOK)

	var set knowledge.KnowledgeSet
	decodeKnowledge(t, body, &set)

	return set, string(body)
}

func beliefLabels(set knowledge.KnowledgeSet) map[string]string {
	labels := map[string]string{}
	for _, belief := range set.Beliefs {
		labels[belief.ID] = belief.Label
	}

	return labels
}

func TestKnowledgeRoutesListActions(t *testing.T) {
	e := newKnowledgeRoutes(t, copyTestdata(t, "good"))

	set, body := listKnowledge(t, e, "/api/knowledge")
	if len(set.Actions) != 3 || !slices.Equal(set.Without, []string{}) {
		t.Fatalf("set %s", body)
	}

	open := set.Actions[0]
	if open.ID != "open-action" || open.Text != "Build X." || !slices.Equal(open.Because, []string{"choice"}) || open.Opened.Who != "franco" || open.Done != nil || open.Label != "grounded" {
		t.Fatalf("open-action %+v", open)
	}
	if !strings.HasSuffix(open.File, filepath.Join("actions", "actions.yaml")) {
		t.Fatalf("open-action file %s", open.File)
	}

	done := set.Actions[1]
	if done.ID != "done-action" || done.Done == nil || done.Done.When != "2026-11-03" {
		t.Fatalf("done-action %+v", done)
	}

	doubted := set.Actions[2]
	if doubted.Label != "in_doubt" || !slices.Equal(doubted.Doubted, []string{"retracted-only"}) || !slices.Equal(doubted.Opinions, []string{}) {
		t.Fatalf("doubted-action %+v", doubted)
	}

	if !strings.Contains(body, `"done":null`) {
		t.Fatalf("open action has no null done: %s", body)
	}
}

func TestKnowledgeRoutesListAlternativesAndContradictions(t *testing.T) {
	e := newKnowledgeRoutes(t, copyTestdata(t, "alternatives"))

	set, body := listKnowledge(t, e, "/api/knowledge")
	ors := map[string][][]string{}
	for _, belief := range set.Beliefs {
		ors[belief.ID] = belief.Or
	}
	if len(ors["by-x"]) != 0 || len(ors["either"]) != 1 || !slices.Equal(ors["either"][0], []string{"seen-y"}) {
		t.Fatalf("or %v", ors)
	}
	if !strings.Contains(body, `"or":[]`) {
		t.Fatalf("no alternatives is not []: %s", body)
	}

	e = newKnowledgeRoutes(t, copyTestdata(t, "contradictions"))

	set, body = listKnowledge(t, e, "/api/knowledge")
	contested := map[string][]string{}
	for _, belief := range set.Beliefs {
		contested[belief.ID] = belief.ContestedBy
	}
	want := map[string]string{"claim-a": "contested", "claim-b": "grounded", "claim-c": "contested", "hunch-c": "opinion", "claim-d": "grounded"}
	if !maps.Equal(beliefLabels(set), want) {
		t.Fatalf("labels %v, want %v", beliefLabels(set), want)
	}
	if !slices.Equal(contested["claim-a"], []string{"rig-lacks-kvm"}) || !slices.Equal(contested["claim-c"], []string{"claim-b"}) || !slices.Equal(contested["claim-d"], []string{}) {
		t.Fatalf("contested_by %v", contested)
	}

	contradicts := map[string][]string{}
	for _, evidence := range set.Evidence {
		contradicts[evidence.ID] = evidence.Contradicts
	}
	if !slices.Equal(contradicts["rig-lacks-kvm"], []string{"rig-has-kvm"}) || !slices.Equal(contradicts["rig-has-kvm"], []string{}) {
		t.Fatalf("contradicts %v in %s", contradicts, body)
	}
}

func TestKnowledgeRoutesListWithout(t *testing.T) {
	e := newKnowledgeRoutes(t, copyTestdata(t, "good"))

	set, body := listKnowledge(t, e, "/api/knowledge?without=seen-a,%20seen-e-again")
	if !slices.Equal(set.Without, []string{"seen-a", "seen-e-again"}) {
		t.Fatalf("without %v", set.Without)
	}

	labels := beliefLabels(set)
	if labels["observed-a"] != "in_doubt" || labels["top"] != "in_doubt" || labels["unrelated"] != "in_doubt" || labels["observed-b"] != "grounded" {
		t.Fatalf("labels %v", labels)
	}
	if set.Decisions[0].ID != "choice" || set.Decisions[0].Label != "in_doubt" || set.Actions[0].Label != "in_doubt" {
		t.Fatalf("set without seen-a %s", body)
	}

	// The real state is unchanged.
	set, _ = listKnowledge(t, e, "/api/knowledge")
	if beliefLabels(set)["observed-a"] != "grounded" {
		t.Fatalf("observed-a %v", beliefLabels(set))
	}

	body = string(getKnowledge(t, e, "/api/knowledge?without=nope", http.StatusNotFound))
	if !strings.Contains(body, "without: nope is not a loaded belief or evidence") {
		t.Fatalf("body %s", body)
	}
}

func TestKnowledgeRoutesWhyAnyKind(t *testing.T) {
	e := newKnowledgeRoutes(t, copyTestdata(t, "good"))

	why := func(e *echo.Echo, id string) (knowledge.Reason, string) {
		body := getKnowledge(t, e, "/api/knowledge/why/"+id, http.StatusOK)

		var reason knowledge.Reason
		decodeKnowledge(t, body, &reason)

		return reason, string(body)
	}

	reason, body := why(e, "manual-one")
	if reason.Kind != "rule" || reason.Text != "Must be read by a human." || len(reason.Because) != 1 || reason.Because[0].Kind != "decision" {
		t.Fatalf("why manual-one %s", body)
	}
	choice := reason.Because[0].Because[0]
	if choice.ID != "choice" || choice.Kind != "decision" || choice.Because[0].ID != "derived" || choice.Because[0].Kind != "belief" {
		t.Fatalf("why manual-one %s", body)
	}

	reason, body = why(e, "open-action")
	if reason.Kind != "action" || reason.Because[0].ID != "choice" || len(reason.Or) != 0 {
		t.Fatalf("why open-action %s", body)
	}

	reason, body = why(e, "observed-a")
	if reason.Kind != "belief" || reason.Evidence[0].ID != "seen-a" || !strings.Contains(body, `"or":[]`) {
		t.Fatalf("why observed-a %s", body)
	}

	e = newKnowledgeRoutes(t, copyTestdata(t, "alternatives"))

	// An alternative made of evidence lists it as an evidence node.
	reason, body = why(e, "either")
	if len(reason.Because) != 1 || reason.Because[0].ID != "by-x" || len(reason.Or) != 1 || len(reason.Or[0]) != 1 {
		t.Fatalf("why either %s", body)
	}
	seenY := reason.Or[0][0]
	if seenY.ID != "seen-y" || seenY.Kind != "evidence" || seenY.Who != "ana" || seenY.When != "2026-11-02" || seenY.Text != "Y was seen." {
		t.Fatalf("why either alternative %+v", seenY)
	}

	// An alternative made of beliefs nests them with their evidence.
	reason, body = why(e, "choose-x")
	if reason.Kind != "decision" || reason.Because[0].ID != "only-x" || len(reason.Or) != 1 {
		t.Fatalf("why choose-x %s", body)
	}
	byY := reason.Or[0][0]
	if byY.ID != "by-y" || byY.Kind != "belief" || byY.Evidence[0].ID != "seen-y" {
		t.Fatalf("why choose-x alternative %+v", byY)
	}

	getKnowledge(t, e, "/api/knowledge/why/seen-x", http.StatusNotFound)
}

func TestKnowledgeRoutesDoubtAnyKind(t *testing.T) {
	doubt := func(e *echo.Echo, id string) (knowledge.Doubted, string) {
		body := getKnowledge(t, e, "/api/knowledge/doubt/"+id, http.StatusOK)

		var doubted knowledge.Doubted
		decodeKnowledge(t, body, &doubted)

		return doubted, string(body)
	}
	idsOf := func(items any) []string {
		ids := []string{}
		switch list := items.(type) {
		case []knowledge.Decision:
			for _, item := range list {
				ids = append(ids, item.ID)
			}
		case []knowledge.Rule:
			for _, item := range list {
				ids = append(ids, item.ID)
			}
		case []knowledge.Action:
			for _, item := range list {
				ids = append(ids, item.ID)
			}
		case []knowledge.Standing:
			for _, item := range list {
				ids = append(ids, item.ID)
			}
		}

		return ids
	}

	e := newKnowledgeRoutes(t, copyTestdata(t, "good"))

	doubted, body := doubt(e, "choice")
	if len(doubted.Beliefs) != 0 || !slices.Equal(idsOf(doubted.Decisions), []string{"later-choice"}) || !slices.Equal(idsOf(doubted.Rules), []string{"manual-one"}) || !slices.Equal(idsOf(doubted.Actions), []string{"open-action"}) {
		t.Fatalf("doubt choice %s", body)
	}
	if doubted.Rules[0].Subject != "testing" || doubted.Actions[0].Opened.Who != "franco" {
		t.Fatalf("doubt choice %s", body)
	}

	_, body = doubt(e, "passes")
	empty := `{"beliefs":[],"decisions":[],"rules":[],"actions":[],"standing":{"beliefs":[],"decisions":[],"rules":[],"actions":[]}}`
	if strings.TrimSpace(body) != empty {
		t.Fatalf("doubt passes %s, want %s", body, empty)
	}

	getKnowledge(t, e, "/api/knowledge/doubt/open-action", http.StatusNotFound)

	e = newKnowledgeRoutes(t, copyTestdata(t, "alternatives"))

	doubted, body = doubt(e, "seen-x")
	if !slices.Equal(idsOf(doubted.Actions), []string{"act-falls"}) {
		t.Fatalf("doubt seen-x %s", body)
	}
	standing := doubted.Standing
	if !slices.Equal(idsOf(standing.Beliefs), []string{"either"}) || !slices.Equal(idsOf(standing.Decisions), []string{"choose-x"}) || !slices.Equal(idsOf(standing.Rules), []string{"rule-or"}) || !slices.Equal(idsOf(standing.Actions), []string{"act-stands"}) {
		t.Fatalf("doubt seen-x standing %s", body)
	}
	either := standing.Beliefs[0]
	if either.Text != "W holds." || len(either.Other) != 1 || !slices.Equal(either.Other[0], []string{"seen-y"}) {
		t.Fatalf("standing either %+v", either)
	}
	if !slices.Equal(standing.Actions[0].Other[0], []string{"choose-either"}) {
		t.Fatalf("standing act-stands %+v", standing.Actions[0])
	}
}

func TestKnowledgeRoutesConflicts(t *testing.T) {
	e := newKnowledgeRoutes(t, copyTestdata(t, "contradictions"))

	body := getKnowledge(t, e, "/api/knowledge/conflicts", http.StatusOK)

	var set knowledge.ConflictSet
	decodeKnowledge(t, body, &set)

	want := []knowledge.Conflict{
		{
			A:        knowledge.ConflictSide{ID: "rig-lacks-kvm", Kind: "evidence", Text: "The rig has no KVM."},
			B:        knowledge.ConflictSide{ID: "rig-has-kvm", Kind: "evidence", Text: "The rig has KVM."},
			ARestsOn: []string{"rig-lacks-kvm"},
			BRestsOn: []string{"rig-has-kvm"},
		},
		{
			A:        knowledge.ConflictSide{ID: "claim-b", Kind: "belief", Text: "B is true."},
			B:        knowledge.ConflictSide{ID: "claim-c", Kind: "belief", Text: "C is true."},
			ARestsOn: []string{"seen-b"},
			BRestsOn: []string{"hunch-c"},
		},
	}
	if !reflect.DeepEqual(set.Conflicts, want) {
		t.Fatalf("conflicts %s", body)
	}

	e = newKnowledgeRoutes(t, copyTestdata(t, "good"))
	body = getKnowledge(t, e, "/api/knowledge/conflicts", http.StatusOK)
	if strings.TrimSpace(string(body)) != `{"conflicts":[]}` {
		t.Fatalf("conflicts %s", body)
	}
}

func TestKnowledgeRoutesListJudgment(t *testing.T) {
	e := newKnowledgeRoutes(t, copyTestdata(t, "judgment"))

	set, body := listKnowledge(t, e, "/api/knowledge")
	labels := beliefLabels(set)
	want := map[string]string{"icons-help": "judgment", "panel-uses-icons": "judgment", "icons-everywhere": "opinion", "ana-thinks-slots": "grounded"}
	for id, label := range want {
		if labels[id] != label {
			t.Fatalf("belief %s label %q, want %q: %s", id, labels[id], label, body)
		}
	}

	judged := set.Beliefs[0]
	if judged.By != "" || judged.Experience == nil || judged.Experience.Of != "franco" || judged.Experience.In != "years of operator UIs" {
		t.Fatalf("icons-help %+v", judged)
	}
	if !strings.Contains(body, `"by":"ana"`) || !strings.Contains(body, `"experience":{"of":"franco","in":"years of operator UIs"}`) {
		t.Fatalf("body %s", body)
	}
}
