package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		want := `{"dir":"` + dir + `","evidence":[],"beliefs":[],"decisions":[],"rules":[]}`
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
	if seenA.ID != "seen-a" || seenA.Text != "A was seen." || seenA.Source != "https://example.com/a" || seenA.Retracted != nil || !slices.Equal(seenA.CitedBy, []string{"observed-a"}) {
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
	// Only live evidence has a null: its retracted.
	if strings.Contains(strings.ReplaceAll(string(body), `"retracted":null`, ""), "null") {
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
	if !strings.Contains(string(body), "belief nope not found") {
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
	if !strings.Contains(string(body), "belief or evidence nope not found") {
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
	for _, path := range []string{"/api/knowledge", "/api/knowledge/why/orphan", "/api/knowledge/doubt/orphan"} {
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
