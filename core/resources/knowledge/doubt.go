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

// Doubted is what falls if a belief or a piece of evidence is wrong.
type Doubted struct {
	Beliefs   []Belief   `json:"beliefs"`
	Decisions []Decision `json:"decisions"`
	Rules     []Rule     `json:"rules"`
}

// Doubt lists the beliefs, decisions and rules resting on a belief or a
// piece of evidence. Nothing is changed.
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

	// Rule ids are unique across the set, so the id finds the subject.
	subjects := map[string]string{}
	for _, ruleset := range set.Rulesets {
		for _, rule := range ruleset.Rules {
			subjects[rule.ID] = ruleset.Subject
		}
	}

	response := &Doubted{
		Decisions: decisionViews(set, doubted.Decisions),
		Rules:     []Rule{},
	}

	response.Beliefs, err = beliefViews(set, doubted.Beliefs)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	for _, rule := range doubted.Rules {
		response.Rules = append(response.Rules, ruleView(set, subjects[rule.ID], rule))
	}

	return response, nil
}
