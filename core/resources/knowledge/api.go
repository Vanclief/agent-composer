// Package knowledge serves the evidence, beliefs, decisions and rules of
// package knowledge read only, with each belief's label computed when asked
// (docs/decisions/0006-rules-and-beliefs.md). Checks are never run here.
package knowledge

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/vanclief/ez"

	kb "github.com/vanclief/agent-composer/knowledge"
)

// kinds are the subdirectories of the knowledge directory, the same ones
// the agc CLI loads by default.
var kinds = []string{"evidence", "beliefs", "decisions", "rules"}

type API struct {
	dir string
}

// NewAPI returns the knowledge API over dir, which holds evidence/,
// beliefs/, decisions/ and rules/.
func NewAPI(dir string) *API {
	return &API{dir: dir}
}

// load reads the knowledge directory on every call so edits show live. A
// missing directory, or one with none of the kinds, is an empty set.
func (api *API) load() (*kb.Set, error) {
	paths := []string{}
	for _, kind := range kinds {
		path := filepath.Join(api.dir, kind)
		_, err := os.Stat(path)
		if err == nil {
			paths = append(paths, path)
		}
	}

	set, err := kb.Load(paths...)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return set, nil
}

// Evidence is a piece of evidence with the beliefs citing it.
type Evidence struct {
	ID        string         `json:"id"`
	Text      string         `json:"text"`
	Who       string         `json:"who"`
	When      string         `json:"when"`
	Source    string         `json:"source"`
	Retracted *kb.Retraction `json:"retracted"`
	CitedBy   []string       `json:"cited_by"`
	File      string         `json:"file"`
}

// Belief is a belief with its computed status.
type Belief struct {
	ID        string   `json:"id"`
	Text      string   `json:"text"`
	Because   []string `json:"because"`
	Evidence  []string `json:"evidence"`
	Label     string   `json:"label"`
	Opinions  []string `json:"opinions"`
	Doubted   []string `json:"doubted"`
	Retracted []string `json:"retracted"`
	File      string   `json:"file"`
}

// Decision is a decision with the status of what it rests on.
type Decision struct {
	ID       string         `json:"id"`
	Text     string         `json:"text"`
	Because  []string       `json:"because"`
	ADR      string         `json:"adr"`
	Decided  kb.Observation `json:"decided"`
	Label    string         `json:"label"`
	Opinions []string       `json:"opinions"`
	Doubted  []string       `json:"doubted"`
	File     string         `json:"file"`
}

// Rule is a rule with its subject, whether a command checks it, and the
// status of what it rests on.
type Rule struct {
	Subject  string   `json:"subject"`
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Check    string   `json:"check"`
	Because  []string `json:"because"`
	Kind     string   `json:"kind"`
	Label    string   `json:"label"`
	Opinions []string `json:"opinions"`
	Doubted  []string `json:"doubted"`
	File     string   `json:"file"`
}

// label is the package's label as an identifier: "in doubt" is in_doubt.
func label(status kb.Status) string {
	return strings.ReplaceAll(string(status.Label), " ", "_")
}

// list returns ids, or an empty list for nil, so JSON says [] not null.
func list(ids []string) []string {
	if ids == nil {
		return []string{}
	}

	return ids
}

func evidenceView(set *kb.Set, evidence kb.Evidence) Evidence {
	citedBy := []string{}
	for _, belief := range set.Citing(evidence.ID) {
		citedBy = append(citedBy, belief.ID)
	}

	return Evidence{
		ID:        evidence.ID,
		Text:      evidence.Evidence,
		Who:       evidence.Who,
		When:      evidence.When,
		Source:    evidence.Source,
		Retracted: evidence.Retracted,
		CitedBy:   citedBy,
		File:      evidence.File,
	}
}

func beliefView(set *kb.Set, belief kb.Belief) (Belief, error) {
	status, err := set.Status(belief.ID)
	if err != nil {
		return Belief{}, ez.Wrap(err)
	}

	return Belief{
		ID:        belief.ID,
		Text:      belief.Belief,
		Because:   list(belief.Because),
		Evidence:  list(belief.Evidence),
		Label:     label(status),
		Opinions:  list(status.Opinions),
		Doubted:   list(status.Doubted),
		Retracted: list(status.Retracted),
		File:      belief.File,
	}, nil
}

func decisionView(set *kb.Set, decision kb.Decision) Decision {
	status := set.StatusOf(decision.Because)

	view := Decision{
		ID:       decision.ID,
		Text:     decision.Decision,
		Because:  list(decision.Because),
		ADR:      decision.ADR,
		Label:    label(status),
		Opinions: list(status.Opinions),
		Doubted:  list(status.Doubted),
		File:     decision.File,
	}
	if decision.Decided != nil {
		view.Decided = *decision.Decided
	}

	return view
}

func ruleView(set *kb.Set, subject string, rule kb.Rule) Rule {
	status := set.StatusOf(rule.Because)

	kind := "manual"
	if strings.TrimSpace(rule.Check) != "" {
		kind = "mechanical"
	}

	return Rule{
		Subject:  subject,
		ID:       rule.ID,
		Text:     rule.Rule,
		Check:    rule.Check,
		Because:  list(rule.Because),
		Kind:     kind,
		Label:    label(status),
		Opinions: list(status.Opinions),
		Doubted:  list(status.Doubted),
		File:     rule.File,
	}
}

func beliefViews(set *kb.Set, beliefs []kb.Belief) ([]Belief, error) {
	views := []Belief{}
	for _, belief := range beliefs {
		view, err := beliefView(set, belief)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		views = append(views, view)
	}

	return views, nil
}

func decisionViews(set *kb.Set, decisions []kb.Decision) []Decision {
	views := []Decision{}
	for _, decision := range decisions {
		views = append(views, decisionView(set, decision))
	}

	return views
}
