// Package knowledge serves the evidence, beliefs, decisions, rules and
// actions of package knowledge read only, with each label computed when
// asked (docs/decisions/0006-rules-and-beliefs.md). Checks are never run
// here.
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
var kinds = []string{"evidence", "beliefs", "decisions", "rules", "actions"}

type API struct {
	dir string
}

// NewAPI returns the knowledge API over dir, which holds evidence/,
// beliefs/, decisions/, rules/ and actions/.
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
// TranslatedBy names the model that wrote the record from its source, if
// one did, and Contradicts the beliefs and evidence it says it
// contradicts.
type Evidence struct {
	ID           string         `json:"id"`
	Text         string         `json:"text"`
	Who          string         `json:"who"`
	When         string         `json:"when"`
	Source       string         `json:"source"`
	TranslatedBy string         `json:"translated_by"`
	Retracted    *kb.Retraction `json:"retracted"`
	Contradicts  []string       `json:"contradicts"`
	CitedBy      []string       `json:"cited_by"`
	File         string         `json:"file"`
}

// Belief is a belief with its computed status. Or lists its alternative
// sets of reasons as ids, and ContestedBy what stands against it when it
// is contested. By names who holds it, empty for the repo owner, and
// Experience, null when absent, whose experience a judgment comes from.
type Belief struct {
	ID          string         `json:"id"`
	Text        string         `json:"text"`
	By          string         `json:"by"`
	Because     []string       `json:"because"`
	Evidence    []string       `json:"evidence"`
	Or          [][]string     `json:"or"`
	Experience  *kb.Experience `json:"experience"`
	Label       string         `json:"label"`
	Opinions    []string       `json:"opinions"`
	Doubted     []string       `json:"doubted"`
	Retracted   []string       `json:"retracted"`
	ContestedBy []string       `json:"contested_by"`
	File        string         `json:"file"`
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

// Action is something to do because of a decision, with the status of
// what it rests on. Done is null while it is open.
type Action struct {
	ID       string          `json:"id"`
	Text     string          `json:"text"`
	Because  []string        `json:"because"`
	Opened   kb.Observation  `json:"opened"`
	Done     *kb.Observation `json:"done"`
	Label    string          `json:"label"`
	Opinions []string        `json:"opinions"`
	Doubted  []string        `json:"doubted"`
	File     string          `json:"file"`
}

// label is the package's label as an identifier, its spaces replaced by
// underscores: grounded, judgment, opinion, in_doubt or contested. Every
// label in the API goes through here.
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

// alternatives lists each alternative set of reasons as its ids: the
// beliefs and decisions it rests on, then the evidence it cites.
func alternatives(sets []kb.Support) [][]string {
	ids := [][]string{}
	for _, set := range sets {
		ids = append(ids, append(append([]string{}, set.Because...), set.Evidence...))
	}

	return ids
}

func evidenceView(set *kb.Set, evidence kb.Evidence) Evidence {
	citedBy := []string{}
	for _, belief := range set.Citing(evidence.ID) {
		citedBy = append(citedBy, belief.ID)
	}

	return Evidence{
		ID:           evidence.ID,
		Text:         evidence.Evidence,
		Who:          evidence.Who,
		When:         evidence.When,
		Source:       evidence.Source,
		TranslatedBy: evidence.TranslatedBy,
		Retracted:    evidence.Retracted,
		Contradicts:  list(evidence.Contradicts),
		CitedBy:      citedBy,
		File:         evidence.File,
	}
}

func beliefView(set *kb.Set, belief kb.Belief) (Belief, error) {
	status, err := set.Status(belief.ID)
	if err != nil {
		return Belief{}, ez.Wrap(err)
	}

	return Belief{
		ID:          belief.ID,
		Text:        belief.Belief,
		By:          belief.By,
		Because:     list(belief.Because),
		Evidence:    list(belief.Evidence),
		Or:          alternatives(belief.Or),
		Experience:  belief.Experience,
		Label:       label(status),
		Opinions:    list(status.Opinions),
		Doubted:     list(status.Doubted),
		Retracted:   list(status.Retracted),
		ContestedBy: list(status.ContestedBy),
		File:        belief.File,
	}, nil
}

func decisionView(set *kb.Set, decision kb.Decision) Decision {
	status := set.StatusOf(decision.Supports())

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
	status := set.StatusOf(rule.Supports())

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

func actionView(set *kb.Set, action kb.Action) Action {
	status := set.StatusOf(action.Supports())

	view := Action{
		ID:       action.ID,
		Text:     action.Action,
		Because:  list(action.Because),
		Done:     action.Done,
		Label:    label(status),
		Opinions: list(status.Opinions),
		Doubted:  list(status.Doubted),
		File:     action.File,
	}
	if action.Opened != nil {
		view.Opened = *action.Opened
	}

	return view
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

func actionViews(set *kb.Set, actions []kb.Action) []Action {
	views := []Action{}
	for _, action := range actions {
		views = append(views, actionView(set, action))
	}

	return views
}

// subjects maps each rule id to the subject of its ruleset. Rule ids are
// unique across the set.
func subjects(set *kb.Set) map[string]string {
	found := map[string]string{}
	for _, ruleset := range set.Rulesets {
		for _, rule := range ruleset.Rules {
			found[rule.ID] = ruleset.Subject
		}
	}

	return found
}
