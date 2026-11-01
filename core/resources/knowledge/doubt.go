package knowledge

import (
	"context"
	"strings"

	"github.com/vanclief/ez"
)

type DoubtRequest struct {
	ID string `json:"id"`
}

func (r DoubtRequest) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ez.New(ez.EINVALID, "id is required", nil)
	}

	return nil
}

// Doubted is what falls if a belief, a decision, a rule or a piece of
// evidence is wrong, and apart, in Standing, what rests on it but still
// stands on other support.
type Doubted struct {
	Beliefs   []Belief    `json:"beliefs"`
	Decisions []Decision  `json:"decisions"`
	Rules     []Rule      `json:"rules"`
	Actions   []Action    `json:"actions"`
	Standing  StandingSet `json:"standing"`
}

// StandingSet is what rests on the doubted id in some of its sets of
// reasons and still has another, by kind. Only open actions are listed.
type StandingSet struct {
	Beliefs   []Standing `json:"beliefs"`
	Decisions []Standing `json:"decisions"`
	Rules     []Standing `json:"rules"`
	Actions   []Standing `json:"actions"`
}

// Standing is one thing still standing, with Other, the sets of reasons
// that do not reach the doubted id, each as its ids.
type Standing struct {
	ID    string     `json:"id"`
	Text  string     `json:"text"`
	Other [][]string `json:"other"`
	File  string     `json:"file"`
}

// Doubt lists the beliefs, decisions, rules and open actions resting on a
// belief, a decision, a rule or a piece of evidence, and those still
// standing on other support. Nothing is changed.
func (api *API) Doubt(ctx context.Context, requester interface{}, request *DoubtRequest) (*Doubted, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	set, err := api.load()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	doubted, err := set.Doubt(request.ID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	ruleSubjects := subjects(set)
	response := &Doubted{
		Decisions: decisionViews(set, doubted.Decisions),
		Rules:     []Rule{},
		Actions:   actionViews(set, doubted.Actions),
		Standing:  StandingSet{Beliefs: []Standing{}, Decisions: []Standing{}, Rules: []Standing{}, Actions: []Standing{}},
	}

	response.Beliefs, err = beliefViews(set, doubted.Beliefs)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	for _, rule := range doubted.Rules {
		response.Rules = append(response.Rules, ruleView(set, ruleSubjects[rule.ID], rule))
	}

	for _, standing := range doubted.Standing {
		view := Standing{ID: standing.ID, Text: standing.Text, Other: alternatives(standing.Other), File: standing.File}
		switch standing.Kind {
		case "belief":
			response.Standing.Beliefs = append(response.Standing.Beliefs, view)
		case "decision":
			response.Standing.Decisions = append(response.Standing.Decisions, view)
		case "rule":
			response.Standing.Rules = append(response.Standing.Rules, view)
		case "action":
			response.Standing.Actions = append(response.Standing.Actions, view)
		}
	}

	return response, nil
}
